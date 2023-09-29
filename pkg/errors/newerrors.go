package errors

import "fmt"

type ErrIndexNotFound struct {
	Index int64
}

func (c *ErrIndexNotFound) Error() string {
	return fmt.Sprintf("index not found: %d", c.Index)
}
