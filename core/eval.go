package core

import (
	"bytes"
	"errors"
	"fmt"
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
	oType, oEnco := deduceTypeEncoding(value)
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
	Put(key, NewObj(value, exDurationMs, oType, oEnco))
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
	if hasExpired(obj) {
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

	// 1. Get the expiry using your new helper
	exp, isExpirySet := getExpiry(obj)
	
	// 2. If no expiry is set, return -1
	if !isExpirySet {
		return RESP_MINUS_1
	}

	// 3. If it is already expired, return -2
	if exp < uint64(time.Now().UnixMilli()) {
		return RESP_MINUS_2
	}

	// 4. Compute the time remaining
	durationMs := exp - uint64(time.Now().UnixMilli())

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

	setExpiry(obj, exDurationSec*1000)
	return RESP_ONE
}

// TODO: Make it async by forking a new process
func evalBGREWRITEAOF(args []string) []byte {
	DumpAllAOF()
	return RESP_OK
}

func evalINCR(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'incr' command"), false)
	}

	var key = args[0]
	obj := Get(key)

	if obj == nil {
		obj = NewObj("0", -1, OBJ_TYPE_STRING, OBJ_ENCODING_INT)
		Put(key, obj)
	}

	if err := assertType(obj.TypeEncoding, OBJ_TYPE_STRING); err != nil {
		return Encode(err, false)
	}

	if err := assertEncoding(obj.TypeEncoding, OBJ_ENCODING_INT); err != nil {
		return Encode(err, false)
	}

	i, _ := strconv.ParseInt(obj.Value.(string), 10, 64)
	i++
	obj.Value = strconv.FormatInt(i, 10)

	return Encode(i, false)
}

func evalINFO(args []string) []byte {
	var info []byte
	buf := bytes.NewBuffer(info)
	buf.WriteString("# Keyspace\r\n")
	for i := range KeyspaceStat {
		buf.WriteString(fmt.Sprintf("db%d:keys=%d,expires=0,avg_ttl=0\r\n", i, KeyspaceStat[i]["keys"]))
	}
	return Encode(buf.String(), false)
}

func evalCLIENT(args []string) []byte {
	return RESP_OK
}

func evalLATENCY(args []string) []byte {
	return Encode([]string{}, false)
}

func evalLRU(args []string) []byte {
	evictAllkeysLRU()
	return RESP_OK
}

func evalSLEEP(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("ERR wrong number of arguments for 'SLEEP' command"), false)
	}

	durationSec, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return Encode(errors.New("ERR value is not an integer or out of range"), false)
	}
	time.Sleep(time.Duration(durationSec) * time.Second)
	return RESP_OK
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
		case "BGREWRITEAOF":
			buf.Write(evalBGREWRITEAOF(cmd.Args))
		case "INCR":
			buf.Write(evalINCR(cmd.Args))
		case "INFO":
			buf.Write(evalINFO(cmd.Args))
		case "CLIENT":
			buf.Write(evalCLIENT(cmd.Args))
		case "LATENCY":
			buf.Write(evalLATENCY(cmd.Args))
		case "LRU":
			buf.Write(evalLRU(cmd.Args))
		case "SLEEP":
			buf.Write(evalSLEEP(cmd.Args))
		default:
			buf.Write(evalPING(cmd.Args))
		}
	}

	c.Write(buf.Bytes())
}