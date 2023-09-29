package eval

import (
	"fmt"

	"github.com/acorn-io/aml/pkg/value"
)

type Position value.Position

func (p Position) String() string {
	if p.Filename == "" {
		return fmt.Sprintf("%d:%d", p.Line, p.Column)
	}
	return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
}
