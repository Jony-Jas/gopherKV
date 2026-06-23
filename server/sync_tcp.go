package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/jony-jas/gopherKV/config"
	"github.com/jony-jas/gopherKV/core"
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

			log.Printf("client %s sent: %s %v\n", conn.RemoteAddr(), cmd.Cmd, cmd.Args)

			respond(cmd, conn)
		}
	}
}

func readCommand(conn io.ReadWriter) (*core.RedisCmd, error) {
	var buf []byte = make([]byte, 512)

	n, err := conn.Read(buf[:])

	if err != nil {
		return nil, err
	}

	tokens, err := core.DecodeArrayString(buf[:n])

	if err != nil {
		return nil, err
	}

	return &core.RedisCmd{
		Cmd: strings.ToUpper(tokens[0]),
		Args: tokens[1:],
	}, nil
}

func respond(cmd *core.RedisCmd, conn io.ReadWriter) error {
	err := core.EvalAndRespond(cmd, conn)
	if err != nil {
		respondError(conn, err)
	}
	return nil
}

func respondError(conn io.ReadWriter, err error) {
	conn.Write([]byte(fmt.Sprintf("-%s\r\n", err)))
}