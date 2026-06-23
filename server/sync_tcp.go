package server

import (
	"io"
	"log"
	"net"
	"strconv"

	"github.com/jony-jas/gopherKV/config"
)

func RunSyncTCPServer() {
	log.Println("Starting a synchronous TCP server on", config.Host, config.Port)
	
	con_client_cnt := 0
	host := config.Host
	port := strconv.Itoa(config.Port);

	lsnr, err := net.Listen("tcp", host+":"+port)

	if (err != nil) {
		panic(err)
	}

	for {
		conn, err := lsnr.Accept()

		if err != nil {
			panic(err)
		}

		con_client_cnt += 1
		log.Println("client connected with address:", conn.RemoteAddr(), "active clients", con_client_cnt)
		
		for {

			cmd, err := readCommand(conn)

			if (err != nil) {
				conn.Close()
				con_client_cnt -= 1
				log.Println("client disconnected", conn.RemoteAddr(), "concurrent clients", con_client_cnt)
				if err == io.EOF {
					break
				}
				log.Println("err", err)
			}

			log.Println("client", conn.RemoteAddr(), "sent:", cmd)

			if err = respond(cmd, conn); err != nil {
				log.Print("err write:", err)
			}
		}
	}
}

func readCommand(conn net.Conn) (string, error) {
	var buf []byte = make([]byte, 512)

	n, err := conn.Read(buf[:])

	if err != nil {
		return "", err
	}

	return string(buf[:n]), nil
}

func respond(cmd string, conn net.Conn) error {
	_, err := conn.Write([]byte(cmd))
	if err != nil {
		return  err
	}
	return nil
}