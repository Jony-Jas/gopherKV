package core

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"time"
)

var (
	RESP_NIL     = []byte("$-1\r\n")
	RESP_OK      = []byte("+OK\r\n")
	RESP_ZERO    = []byte(":0\r\n")
	RESP_ONE     = []byte(":1\r\n")
	RESP_MINUS_1 = []byte(":-1\r\n")
	RESP_MINUS_2 = []byte(":-2\r\n")
)

// Helper to quickly encode RESP errors
func encodeError(msg string) []byte {
	return Encode(errors.New(msg), false)
}

func evalPING(args []string) []byte {
	if len(args) >= 2 {
		return encodeError("ERR wrong number of arguments for 'ping' command")
	}
	if len(args) == 0 {
		return Encode("PONG", true)
	}
	return Encode(args[0], false)
}

func evalSET(args []string) []byte {
	if len(args) <= 1 {
		return encodeError("ERR wrong number of arguments for 'set' command")
	}

	key, value := args[0], args[1]
	var exDurationMs int64 = -1

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "EX", "ex":
			i++
			if i == len(args) {
				return encodeError("ERR syntax error")
			}

			exDurationSec, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return encodeError("ERR value is not an integer or out of range")
			}
			exDurationMs = exDurationSec * 1000
		default:
			return encodeError("ERR syntax error")
		}
	}

	// put the k and value in the Hash Table
	Put(key, NewObj(value, exDurationMs))
	return RESP_OK
}

func evalGET(args []string) []byte {
	if len(args) != 1 {
		return encodeError("ERR wrong number of arguments for 'get' command")
	}

	key := args[0]
	obj := Get(key)

	// if key does not exist or has expired, return RESP encoded nil
	if obj == nil {
		return RESP_NIL
	}
	if obj.ExpiresAt != -1 && obj.ExpiresAt <= time.Now().UnixMilli() {
		return RESP_NIL
	}

	return Encode(obj.Value, false)
}

func evalTTL(args []string) []byte {
	if len(args) != 1 {
		return encodeError("ERR wrong number of arguments for 'ttl' command")
	}

	key := args[0]
	obj := Get(key)

	if obj == nil {
		return RESP_MINUS_2
	}
	if obj.ExpiresAt == -1 {
		return RESP_MINUS_1
	}

	durationMs := obj.ExpiresAt - time.Now().UnixMilli()
	if durationMs < 0 {
		return RESP_MINUS_2
	}

	return Encode(int64(durationMs/1000), false)
}

func evalDEL(args []string) []byte {
	var countDeleted int = 0

	for _, key := range args {
		if ok := Del(key); ok {
			countDeleted++
		}
	}

	return Encode(countDeleted, false)
}

func evalEXPIRE(args []string) []byte {
	if len(args) <= 1 {
		return encodeError("ERR wrong number of arguments for 'expire' command")
	}

	key := args[0]
	exDurationSec, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return encodeError("ERR value is not an integer or out of range")
	}

	obj := Get(key)
	if obj == nil {
		return RESP_ZERO
	}

	obj.ExpiresAt = time.Now().UnixMilli() + (exDurationSec * 1000)
	return RESP_ONE
}

func EvalAndRespond(cmds RedisCmds, c io.ReadWriter) {
	// safely handle empty slices
	if len(cmds) == 0 {
		c.Write(encodeError("ERR invalid command"))
		return
	}

	var buf bytes.Buffer

	for _, cmd := range cmds {
		switch cmd.Cmd {
		case "PING":
			buf.Write(evalPING(cmd.Args))
		case "SET":
			buf.Write(evalSET(cmd.Args))
		case "GET":
			buf.Write(evalGET(cmd.Args))
		case "TTL":
			buf.Write(evalTTL(cmd.Args))
		case "DEL":
			buf.Write(evalDEL(cmd.Args))
		case "EXPIRE":
			buf.Write(evalEXPIRE(cmd.Args))
		default:
			buf.Write(evalPING(cmd.Args))
		}
	}

	c.Write(buf.Bytes())
}