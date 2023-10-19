package value

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
)

type (
	evalPathKey struct{}
	dataPathKey struct{}
)

func withPathElement(ctx context.Context, path PathElement) context.Context {
	currentPath, _ := ctx.Value(evalPathKey{}).(Path)
	return context.WithValue(ctx, evalPathKey{}, append(currentPath, path))

}

func withDataPathElement(ctx context.Context, path PathElement) context.Context {
	currentPath, _ := ctx.Value(dataPathKey{}).(Path)
	return context.WithValue(ctx, dataPathKey{}, append(currentPath, path))

}

func WithDataKeyPath(ctx context.Context, key string) context.Context {
	return withDataPathElement(ctx, PathElement{
		Key: &key,
	})
}

func WithDataIndexPath(ctx context.Context, idx int) context.Context {
	return withDataPathElement(ctx, PathElement{
		Index: &idx,
	})
}

func GetDataPath(ctx context.Context) Path {
	currentPath, _ := ctx.Value(dataPathKey{}).(Path)
	return currentPath
}

func WithKeyPath(ctx context.Context, key string) context.Context {
	return withPathElement(ctx, PathElement{
		Key: &key,
	})
}

func WithCallPath(ctx context.Context) context.Context {
	return withPathElement(ctx, PathElement{
		Call: true,
	})
}

func WithIndexPath(ctx context.Context, idx int) context.Context {
	return withPathElement(ctx, PathElement{
		Index: &idx,
	})
}

func GetPath(ctx context.Context) Path {
	currentPath, _ := ctx.Value(evalPathKey{}).(Path)
	return currentPath
}

type Path []PathElement

func (p Path) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

func (p Path) Equals(right Path) bool {
	if len(p) != len(right) {
		return false
	}
	for i := range p {
		if !p[i].Equals(right[i]) {
			return false
		}
	}
	return true
}

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

func ptrEq[T comparable](left, right *T) bool {
	if left == nil && right != nil {
		return false
	} else if left != nil && right == nil {
		return false
	} else if left == nil && right == nil {
		return true
	}
	return *left == *right
}

func (p PathElement) Equals(right PathElement) bool {
	return ptrEq(p.Key, right.Key) &&
		ptrEq(p.Index, right.Index) &&
		p.Call == right.Call
}
