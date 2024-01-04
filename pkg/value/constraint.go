package value

import (
	"context"
	"errors"
	"fmt"
)

var ErrMustMatchAlternate = errors.New("must match alternate")

type Constraints []Constraint

func (c Constraints) Check(ctx context.Context, left Value) error {
	for _, checker := range c {
		err := checker.Check(ctx, left)
		if err != nil {
			return err
		}
	}
	return nil
}

const (
	MustBeIntOp          = "mustBeInt"
	MustMatchAlternateOp = "mustMatchAlternate"
	MustMatchSchema      = "mustMatchSchema"
)

func MustMatchAlternate() []Constraint {
	return []Constraint{
		{
			Op: MustMatchAlternateOp,
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
	Op    string `json:"op,omitempty"`
	Right Value  `json:"right,omitempty"`
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
	v, err := BinaryOperation(op, left, func() (Value, error) {
		return toConcrete(right)
	})
	if err != nil {
		return err
	}
	b, err := ToBool(v)
	if err != nil {
		return err
	}
	if !b {
		return fmt.Errorf("constraint [value %s %s] is not true", c.Op, right)
	}
	return nil
}

func (c *Constraint) Check(ctx context.Context, left Value) error {
	switch Operator(c.Op) {
	case GtOp, GeOp, LtOp, LeOp, EqOp, NeqOp, MatOp, NmatOp:
		return c.check(Operator(c.Op), left, c.Right)
	case MustMatchSchema:
		_, err := c.Right.(*TypeSchema).Validate(ctx, left)
		return err
	case MustBeIntOp:
		_, err := ToInt(left)
		return err
	case MustMatchAlternateOp:
		return ErrMustMatchAlternate
	default:
		return fmt.Errorf("unknown operator for constraint: %s", c.Op)
	}
}
