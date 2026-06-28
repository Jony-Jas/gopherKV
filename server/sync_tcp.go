package server

import (
	"fmt"
	"io"
	"strings"

	"github.com/jony-jas/gopherKV/core"
)

// func RunSyncTCPServer() {
// 	log.Println("Starting a synchronous TCP server on", config.Host, config.Port)

// 	con_client_cnt := 0
// 	host := config.Host
// 	port := strconv.Itoa(config.Port);

// 	lsnr, err := net.Listen("tcp", host+":"+port)

// 	if (err != nil) {
// 		panic(err)
// 	}

// 	for {
// 		conn, err := lsnr.Accept()

// 		if err != nil {
// 			panic(err)
// 		}

// 		con_client_cnt += 1
// 		log.Println("client connected with address:", conn.RemoteAddr(), "active clients", con_client_cnt)

// 		for {

// 			cmds, err := readCommands(conn)

// 			if (err != nil) {
// 				conn.Close()
// 				con_client_cnt -= 1
// 				log.Println("client disconnected", conn.RemoteAddr(), "concurrent clients", con_client_cnt)
// 				if err == io.EOF {
// 					break
// 				}
// 				log.Println("err", err)
// 			}

// 			var summary []string
// 			for _, cmd := range cmds {
// 				summary = append(summary, fmt.Sprintf("%s %v", cmd.Cmd, cmd.Args))
// 			}

// 			log.Printf("client %s sent: [%s]\n", conn.RemoteAddr(), strings.Join(summary, " | "))

// 			respond(cmds, conn)
// 		}
// 	}
// }

// TODO: Max read in one shot is 512 bytes
// To allow input > 512 bytes, then repeated read until
// we get EOF or designated delimiter
func readCommands(conn io.ReadWriter) (core.RedisCmds, error) {
	var buf []byte = make([]byte, 512)

	n, err := conn.Read(buf[:])

	if err != nil {
		return nil, err
	}

	values, err := core.Decode(buf[:n])

	if err != nil {
		return nil, err
	}

	var cmds []*core.RedisCmd = make([]*core.RedisCmd, 0)
	for _, value := range values {
		tokens, err := toArrayString(value.([]any))
		if err != nil {
			return nil, err
		}
		cmds = append(cmds, &core.RedisCmd{
			Cmd:  strings.ToUpper(tokens[0]),
			Args: tokens[1:],
		})
	}
	return cmds, nil
}

func toArrayString(ai []any) ([]string, error) {
	as := make([]string, len(ai))
	for i := range ai {
		as[i] = ai[i].(string)
	}
	return as, nil
}

func respond(cmd core.RedisCmds, c *core.Client) {
	core.EvalAndRespond(cmd, c)
}

func respondError(conn io.ReadWriter, err error) {
	conn.Write([]byte(fmt.Sprintf("-%s\r\n", err)))
}