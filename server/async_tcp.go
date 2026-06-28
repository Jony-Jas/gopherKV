package server

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jony-jas/gopherKV/config"
	"github.com/jony-jas/gopherKV/core"
)

var cronFrequency time.Duration = 1 * time.Second
var lastCronExecTime time.Time = time.Now()
const EngineStatus_WAITING int32 = 1 << 1
const EngineStatus_BUSY int32 = 1 << 2
const EngineStatus_SHUTTING_DOWN int32 = 1 << 3
const EngineStatus_TRANSACTION int32 = 1 << 4

var eStatus int32 = EngineStatus_WAITING

var connectedClients map[int]*core.Client

func init() {
	connectedClients = make(map[int]*core.Client)
}

func WaitForSignal(wg *sync.WaitGroup, sigs chan os.Signal) {
    defer wg.Done()
    <-sigs

    log.Println("Shutdown signal received, waiting for server to finish current tasks...")

    // Spin until we successfully transition from WAITING -> SHUTTING_DOWN
    for {
        current := atomic.LoadInt32(&eStatus)
        if current == EngineStatus_BUSY {
            // Server is busy, yield the processor to avoid burning CPU
            time.Sleep(10 * time.Millisecond) 
            continue
        }

        // The state is WAITING. Try to atomically swap it to SHUTTING_DOWN.
        // If this succeeds, the server thread CANNOT transition to BUSY anymore.
        if atomic.CompareAndSwapInt32(&eStatus, EngineStatus_WAITING, EngineStatus_SHUTTING_DOWN) {
            break // Lock acquired! We are safe to shut down.
        }
    }

    // Now we safely execute the shutdown
    core.Shutdown()
}


func RunAsyncTCPServer(wg *sync.WaitGroup) error {

	defer wg.Done()
	defer func() {
		atomic.StoreInt32(&eStatus, EngineStatus_SHUTTING_DOWN)
	}()

	log.Println("starting an asynchronous TCP server on", config.Host, config.Port)

	max_clients := 20000

	// Epoll events array to hold incoming epoll events
	var events []syscall.EpollEvent = make([]syscall.EpollEvent, max_clients)

	//create socket
	serverFD, err := syscall.Socket(syscall.AF_INET, syscall.O_NONBLOCK|syscall.SOCK_STREAM, 0)

	if err != nil {
		return err
	}

	defer syscall.Close(serverFD)

	// Set the Socket operate in a non-blocking mode
	if err = syscall.SetNonblock(serverFD, true); err != nil {
		return err
	}

	// Bind the IP and the port
	ip4 := net.ParseIP(config.Host)
	if err = syscall.Bind(serverFD, &syscall.SockaddrInet4{
		Port: config.Port,
		Addr: [4]byte{ip4[0], ip4[1], ip4[2], ip4[3]},
	}); err != nil {
		return err
	}

	// Start listening
	if err = syscall.Listen(serverFD, max_clients); err != nil {
		return err
	}

	// AsyncIO starts here!!

	// creating EPOLL instance
	epollFD, err := syscall.EpollCreate1(0)
	if err != nil {
		log.Fatal(err)
	}
	defer syscall.Close(epollFD)

	// Specify the events we want to get hints about (here only monitor incomming EPOLLIN)
	// and set the socket on which
	var socketServerEvent syscall.EpollEvent = syscall.EpollEvent{
		Events: syscall.EPOLLIN,
		Fd:     int32(serverFD),
	}

	// Listen to read events on the Server itself
	if err = syscall.EpollCtl(epollFD, syscall.EPOLL_CTL_ADD, serverFD, &socketServerEvent); err != nil {
		return err
	}

	for atomic.LoadInt32(&eStatus) != EngineStatus_SHUTTING_DOWN {
		if time.Now().After(lastCronExecTime.Add(cronFrequency)) {
			core.DeleteExpiredKeys()
			lastCronExecTime = time.Now()
		}

		// Say, the Engine triggered SHUTTING down when the control flow is here ->
		// Current: Engine status == WAITING
		// Update: Engine status = SHUTTING_DOWN
		// Then we have to exit (handled in Signal Handler)

		// see if any FD is ready for an IO
		
		// Wait for maximum 100ms
		nevents, e := syscall.EpollWait(epollFD, events[:], 100) 
		if e != nil {
			continue
		}
		if nevents == 0 {
			// Timeout reached, loop around to check if eStatus is SHUTTING_DOWN
			continue 
		}

		// Here, we do not want server to go back from SHUTTING DOWN
		// to BUSY
		// If the engine status == SHUTTING_DOWN over here ->
		// We have to exit
		// hence the only legal transitiion is from WAITING to BUSY
		// if that does not happen then we can exit.

		// mark engine as BUSY only when it is in the waiting state
		if !atomic.CompareAndSwapInt32(&eStatus, EngineStatus_WAITING, EngineStatus_BUSY) {
			// if swap unsuccessful then the existing status is not WAITING, but something else
			switch eStatus {
			case EngineStatus_SHUTTING_DOWN:
				return nil
			}
		}

		for i := 0; i < nevents; i++ {
			// if the socket server itself is ready for an IO
			if int(events[i].Fd) == serverFD {
				// accept the incoming connection from a client
				fd, _, err := syscall.Accept(serverFD)
				if err != nil {
					log.Println("err", err)
					continue
				}

				// increase the number of concurrent clients count
				connectedClients[fd] = core.NewClient(fd)
				syscall.SetNonblock(serverFD, true)

				// add this new TCP connection to be monitored
				var socketClientEvent syscall.EpollEvent = syscall.EpollEvent{
					Events: syscall.EPOLLIN,
					Fd:     int32(fd),
				}
				if err := syscall.EpollCtl(epollFD, syscall.EPOLL_CTL_ADD, fd, &socketClientEvent); err != nil {
					log.Fatal(err)
				}
			} else {
				comm := connectedClients[int(events[i].Fd)]
				if comm == nil {
					continue
				}
				cmds, err := readCommands(comm)
				if err != nil {
					syscall.Close(int(events[i].Fd))
					delete(connectedClients, int(events[i].Fd))
					continue
				}
				var summary []string
				for _, cmd := range cmds {
					summary = append(summary, fmt.Sprintf("%s %v", cmd.Cmd, cmd.Args))
				}

				log.Printf("client %d sent: [%s]\n", comm.Fd, strings.Join(summary, " | "))
				respond(cmds, comm)				
			}
		}
		// mark engine as WAITING
		// no contention as the signal handler is blocked until
		// the engine is BUSY
		atomic.StoreInt32(&eStatus, EngineStatus_WAITING)
	}
	return nil
}