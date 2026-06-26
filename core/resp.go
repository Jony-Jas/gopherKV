package core

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

func encodeString(v string) []byte {
	return []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(v), v))
}

func Encode(value interface{}, isSimple bool) []byte {
	switch v := value.(type) {
	case string:
		if isSimple {
			return []byte(fmt.Sprintf("+%s\r\n", v))
		}
		return encodeString(v)
	case int, int8, int16, int32, int64:
		return []byte(fmt.Sprintf(":%d\r\n", v))
	case error:
		return []byte(fmt.Sprintf("-%s\r\n", v))
	case []string:
		var b []byte
		buf := bytes.NewBuffer(b)
		for _, b := range value.([]string) {
			buf.Write(encodeString(b))
		}
		return []byte(fmt.Sprintf("*%d\r\n%s", len(v), buf.Bytes()))
	default:
		return RESP_NIL
	}
}

func Decode(data []byte) ([]any, error) {
	if len(data) == 0 {
		return nil, errors.New("no data")
	}

	var values []any = make([]any, 0)
	var index int = 0
	for index < len(data) {
		value, delta, err := decodeOne(data[index:])
		if err != nil {
			return values, err
		}
		index = index + delta
		values = append(values, value)
	}
	return values, nil
}

func decodeOne(data []byte) (any, int, error) {
	if len(data) == 0 {
		return nil, 0, errors.New("no data")
	}

	switch data[0] {
	case '+':
		return readSimpleString(data)
	case '-':
		return readError(data)
	case ':':
		return readInt64(data)
	case '$':
		return readBulkString(data)
	case '*':
		return readArray(data)
	default:
		return nil, 0, fmt.Errorf("unknown RESP type byte: %c", data[0])
	}
}

func findCRLF(data []byte, offset int) (int, error) {
	for i := offset; i < len(data)-1; i++ {
		if data[i] == '\r' && data[i+1] == '\n' {
			return i, nil
		}
	}
	return 0, errors.New("CRLF not found")
}

func readSimpleString(data []byte) (string, int, error) {
	crlfIdx, err := findCRLF(data, 1)
	if err != nil {
		return "", 0, errors.New("malformed simple string")
	}

	return string(data[1:crlfIdx]), crlfIdx + 2, nil
}

func readError(data []byte) (string, int, error) {
	return readSimpleString(data)
}

func readInt64(data []byte) (int64, int, error) {
	crlfIdx, err := findCRLF(data, 1)
	if err != nil {
		return 0, 0, errors.New("malformed integer")
	}

	strVal := string(data[1:crlfIdx])
	value, err := strconv.ParseInt(strVal, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid integer format: %w", err)
	}

	return value, crlfIdx + 2, nil
}

func readLength(data []byte) (int, int, error) {
	val, pos, err := readInt64(data)
	if err != nil {
		return 0, 0, err
	}
	return int(val), pos, nil
}

func readBulkString(data []byte) (any, int, error) {
	length, pos, err := readLength(data)
	if err != nil {
		return nil, 0, err
	}

	if length == -1 {
		return nil, pos, nil
	}
	if length < -1 {
		return nil, 0, errors.New("invalid bulk string length")
	}

	if pos+length+2 > len(data) {
		return nil, 0, errors.New("incomplete bulk string data")
	}

	if data[pos+length] != '\r' || data[pos+length+1] != '\n' {
		return nil, 0, errors.New("malformed bulk string: missing trailing CRLF")
	}

	str := string(data[pos : pos+length])
	return str, pos + length + 2, nil
}

func readArray(data []byte) ([]any, int, error) {
	length, pos, err := readLength(data)
	if err != nil {
		return nil, 0, err
	}

	if length == -1 {
		return nil, pos, nil
	}
	if length < -1 {
		return nil, 0, errors.New("invalid array length")
	}

	elems := make([]any, length)
	for i := 0; i < length; i++ {
		if pos >= len(data) {
			return nil, 0, errors.New("incomplete array data")
		}

		val, delta, err := decodeOne(data[pos:])
		if err != nil {
			return nil, 0, err
		}
		elems[i] = val
		pos += delta
	}

	return elems, pos, nil
}