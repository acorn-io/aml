package value

import (
	"errors"
	"fmt"
)

var ErrMustMatchAlternate = errors.New("must match alternate")

type Checker interface {
	Check(left Value) error
	Description() string
	ID() string
	OpString() string
	RightNative() (any, bool, error)
}

type Constraints []Checker

func (c Constraints) Check(left Value) error {
	for _, checker := range c {
		err := checker.Check(left)
		if err != nil {
			return err
		}
	}
	return nil
}

func MustMatchAlternate() []Checker {
	return []Checker{
		&CustomConstraint{
			CustomID:          "or",
			CustomDescription: "must match alternate",
			Checker: func(left Value) error {
				return ErrMustMatchAlternate
			},
		},
	}
}

type CustomConstraint struct {
	CustomID          string
	CustomDescription string
	Checker           func(left Value) error
}

func (c *CustomConstraint) Check(left Value) error {
	return c.Checker(left)
}

func (c *CustomConstraint) ID() string {
	return c.CustomID
}

func (c *CustomConstraint) Description() string {
	return c.CustomDescription
}

func (c *CustomConstraint) OpString() string {
	return "custom"
}

func (c *CustomConstraint) RightNative() (any, bool, error) {
	return nil, false, nil
}

type Constraint struct {
	Op    string
	Right Value
}

func (c *Constraint) ID() string {
	return ""
}

func (c *Constraint) Description() string {
	return ""
}

func (c *Constraint) OpString() string {
	return c.Op
}

func (c *Constraint) RightNative() (any, bool, error) {
	if ts, ok := c.Right.(*TypeSchema); ok {
		return ts, true, nil
	}
	return NativeValue(c.Right)
}

func toConcrete(val Value) (Value, error) {
	def, ok, err := DefaultValue(val)
	if err != nil {
		return nil, err
	}
	if ok {
		return def, nil
	}
	return val, nil
}

func (c *Constraint) check(op Operator, left, right Value) error {
	left, err := toConcrete(left)
	if err != nil {
		return err
	}
	right, err = toConcrete(right)
	if err != nil {
		return err
	}
	v, err := BinaryOperation(op, left, right)
	if err != nil {
		return err
	}
	b, err := ToBool(v)
	if err != nil {
		return err
	}
	if !b {
		return fmt.Errorf("invalid constraint [value %s %s] where value is [%s]", c.Op, right, left)
	}
	return nil
}

func (c *Constraint) Check(left Value) error {
	switch Operator(c.Op) {
	case GtOp, GeOp, LtOp, LeOp, EqOp, NeqOp, MatOp, NmatOp:
		return c.check(Operator(c.Op), left, c.Right)
	case Operator("type"):
		_, err := Merge(c.Right, left)
		return err
	default:
		return fmt.Errorf("unknown operator for constraint: %s", c.Op)
	}
}
