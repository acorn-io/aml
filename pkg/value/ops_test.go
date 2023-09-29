package value

import (
	"fmt"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnary(t *testing.T) {
	tests := []struct {
		op     string
		val    any
		expect autogold.Value
	}{
		{op: "+", val: 1, expect: autogold.Expect(Number("1"))},
		{op: "-", val: 1, expect: autogold.Expect(Number("-1"))},
		{op: "-", val: Number("4"), expect: autogold.Expect(Number("-4"))},
		{op: "!", val: false, expect: autogold.Expect(true)},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%s%d", t.Name(), i), func(t *testing.T) {
			v, err := UnaryOperation(Operator(test.op), NewValue(test.val))
			require.NoError(t, err)
			nv, _, err := NativeValue(v)
			require.NoError(t, err)
			test.expect.Equal(t, nv)
		})
	}
}

func TestIndex(t *testing.T) {
	v, ok, err := Index(NewValue([]any{
		"key", "key2",
	}), NewValue(1))
	require.NoError(t, err)
	assert.True(t, ok)
	nv, _, err := NativeValue(v)
	require.NoError(t, err)
	assert.Equal(t, "key2", nv)
}

func TestSlice(t *testing.T) {
	v, ok, err := Slice(NewValue([]any{
		"key", "key2", "key3",
	}), NewValue(1), NewValue(3))
	require.NoError(t, err)
	assert.True(t, ok)
	nv, _, err := NativeValue(v)
	require.NoError(t, err)
	assert.Equal(t, []any{"key2", "key3"}, nv)
}

func TestLookup(t *testing.T) {
	v, ok, err := Lookup(NewValue(map[string]any{
		"key": "value",
	}), NewValue("key"))
	require.NoError(t, err)
	assert.True(t, ok)
	nv, _, err := NativeValue(v)
	require.NoError(t, err)
	assert.Equal(t, "value", nv)
}

func TestBinary(t *testing.T) {
	tests := []struct {
		op     string
		left   any
		right  any
		expect autogold.Value
	}{
		{op: "*", left: 2, right: 3, expect: autogold.Expect(Number("6"))},
		{op: "*", left: 2.0, right: 3, expect: autogold.Expect(Number("6"))},
		{op: "*", left: 0.1, right: 30, expect: autogold.Expect(Number("3"))},
		{op: "/", left: 6, right: 2, expect: autogold.Expect(Number("3"))},
		{op: "&&", left: false, right: true, expect: autogold.Expect(false)},
		{op: "||", left: false, right: true, expect: autogold.Expect(true)},
		{op: "<", left: 3, right: 4, expect: autogold.Expect(true)},
		{op: "<=", left: 4, right: 4, expect: autogold.Expect(true)},
		{op: ">", left: 3, right: 4, expect: autogold.Expect(false)},
		{op: ">=", left: 4, right: 4, expect: autogold.Expect(true)},
		{op: "==", left: 1, right: 1, expect: autogold.Expect(true)},
		{op: "==", left: true, right: true, expect: autogold.Expect(true)},
		{op: "==", left: "x", right: "x", expect: autogold.Expect(true)},
		{op: "==", left: nil, right: nil, expect: autogold.Expect(true)},
		{op: "!=", left: 1, right: 1, expect: autogold.Expect(false)},
		{op: "!=", left: true, right: true, expect: autogold.Expect(false)},
		{op: "!=", left: "x", right: "x", expect: autogold.Expect(false)},
		{op: "!=", left: nil, right: nil, expect: autogold.Expect(false)},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%s%d - %s %s %s", t.Name(), i, test.left, test.op, test.right), func(t *testing.T) {
			v, err := BinaryOperation(Operator(test.op), NewValue(test.left), NewValue(test.right))
			require.NoError(t, err)
			nv, _, err := NativeValue(v)
			require.NoError(t, err)
			test.expect.Equal(t, nv)
		})
	}
}
