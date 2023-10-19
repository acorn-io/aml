package eval

import (
	"context"
	"strconv"
	"strings"
)

type (
	schemaKey       struct{}
	allowNewKeysKey struct{}
	scopeKey        struct{}
)

func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

func GetScope(ctx context.Context) Scope {
	scope, ok := ctx.Value(scopeKey{}).(Scope)
	if !ok {
		scope, _ = EmptyScope(ctx, nil)
	}
	return scope
}

func WithSchema(ctx context.Context, schema bool) context.Context {
	return context.WithValue(ctx, schemaKey{}, schema)
}

func IsSchema(ctx context.Context) bool {
	// default to false
	v, _ := ctx.Value(schemaKey{}).(bool)
	return v
}

func WithAllowNewKeys(ctx context.Context, allowNewKeys bool) context.Context {
	// default to true
	return context.WithValue(ctx, allowNewKeysKey{}, &allowNewKeys)
}

func IsAllowNewKeys(ctx context.Context) bool {
	v, _ := ctx.Value(allowNewKeysKey{}).(*bool)
	if v == nil {
		return false
	}
	return *v
}

type Path []PathElement

func (p Path) String() string {
	var buf strings.Builder
	for _, part := range p {
		if part.Call {
			buf.WriteString("()")
		} else if part.Key != nil {
			if buf.Len() > 0 {
				buf.WriteString(".")
			}
			buf.WriteString(*part.Key)
		} else if part.Index != nil {
			buf.WriteString("[")
			buf.WriteString(strconv.Itoa(*part.Index))
			buf.WriteString("]")
		}
	}
	return buf.String()
}

type PathElement struct {
	Key   *string
	Index *int
	Call  bool
}
