package aml

import (
	"bytes"
	"strconv"

	"cuelang.org/go/cue/literal"
)

func Unmarshal(data []byte, v any) error {
	return NewDecoder(bytes.NewBuffer(data)).Decode(v)
}

// ParseInt parses a number string to int following the
// same number syntax that AML supports.
func ParseInt(numString string) (int64, error) {
	numInfo := literal.NumInfo{}
	err := literal.ParseNum(numString, &numInfo)
	if err != nil {
		return -1, err
	}

	quantity, err := strconv.ParseInt(numInfo.String(), 10, 64)
	if err != nil {
		return -1, err
	}

	return quantity, nil
}
