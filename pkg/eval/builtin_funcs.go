package eval

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/acorn-io/aml/pkg/value"
	"gopkg.in/yaml.v3"
)

var (
	DebugEnabled = true
	nativeFuncs  = map[string]any{
		"atoi":          NativeFuncValue(Atoi),
		"range":         NativeFuncValue(Range),
		"fromYAML":      NativeFuncValue(FromYAML),
		"toYAML":        NativeFuncValue(ToYAML),
		"sha1sum":       NativeFuncValue(Sha1sum),
		"sha256sum":     NativeFuncValue(Sha256sum),
		"sha512sum":     NativeFuncValue(Sha512sum),
		"base64":        NativeFuncValue(Base64),
		"base64decode":  NativeFuncValue(Base64Decode),
		"toHex":         NativeFuncValue(ToHex),
		"fromHex":       NativeFuncValue(FromHex),
		"toJSON":        NativeFuncValue(ToJSON),
		"fromJSON":      NativeFuncValue(FromJSON),
		"splitHostPort": NativeFuncValue(SplitHostPort),
		"joinHostPort":  NativeFuncValue(JoinHostPort),
		"cut":           NativeFuncValue(Cut),
		"pathJoin":      NativeFuncValue(PathJoin),
		"dirname":       NativeFuncValue(Dirname),
		"basename":      NativeFuncValue(Basename),
		"fileExt":       NativeFuncValue(FileExt),
		"toTitle":       NativeFuncValue(ToTitle),
		"isA":           NativeFuncValue(IsA),
		"split":         NativeFuncValue(Split),
		"join":          NativeFuncValue(Join),
		"endsWith":      NativeFuncValue(EndsWith),
		"startsWith":    NativeFuncValue(StartsWith),
		"toUpper":       NativeFuncValue(ToUpper),
		"toLower":       NativeFuncValue(ToLower),
		"trimSuffix":    NativeFuncValue(TrimSuffix),
		"trimPrefix":    NativeFuncValue(TrimPrefix),
		"trim":          NativeFuncValue(Trim),
		"replace":       NativeFuncValue(Replace),
		"indexOf":       NativeFuncValue(IndexOf),
		"merge":         NativeFuncValue(Merge),
		"sort":          NativeFuncValue(Sort),
		"mod":           NativeFuncValue(Mod),
		"error":         NativeFuncValue(Error),
		"debug":         NativeFuncValue(Debug),
		"catch":         NativeFuncValue(Catch),
		"contains":      NativeFuncValue(Contains),
		"describe":      NativeFuncValue(Describe),
	}
)

func Mod(_ context.Context, args []value.Value) (value.Value, bool, error) {
	left, err := value.ToInt(args[0])
	if err != nil {
		return nil, false, err
	}

	right, err := value.ToInt(args[1])
	if err != nil {
		return nil, false, err
	}

	ret := left % right
	return value.NewValue(ret), true, nil
}

func Len(_ context.Context, args []value.Value) (value.Value, bool, error) {
	v, err := value.Len(args[0])
	return v, true, err
}

type ErrorValue struct {
	value.Value
}

func (e ErrorValue) Error() string {
	return fmt.Sprint(e.Value)
}

func Error(_ context.Context, args []value.Value) (value.Value, bool, error) {
	return nil, false, ErrorValue{
		Value: args[0],
	}
}

func Catch(ctx context.Context, args []value.Value) (value.Value, bool, error) {
	_, _, err := value.Call(ctx, args[0])
	if errorValue := (ErrorValue{}); errors.As(err, &errorValue) {
		return errorValue.Value, true, nil
	}
	return value.NewValue(nil), true, nil
}

func Debug(_ context.Context, args []value.Value) (value.Value, bool, error) {
	if !DebugEnabled {
		return nil, false, nil
	}
	s, err := value.ToString(args[0])
	if err == nil {
		var v []any
		for _, x := range args[1:] {
			v = append(v, x)
		}
		if strings.Contains(s, "%") {
			log.Printf("AML DEBUG: "+s, v...)
		} else {
			log.Print(append([]any{"AML DEBUG: " + s}, v...))
		}
	} else {
		var vs []any
		for _, v := range args {
			vs = append(vs, v)
		}
		log.Print(vs...)
	}
	return nil, false, nil
}

func Keys(_ context.Context, args []value.Value) (value.Value, bool, error) {
	v, err := value.Keys(args[0])
	return value.NewValue(v), true, err
}

func defaultLess(ctx context.Context, args []value.Value) (value.Value, bool, error) {
	left, right := args[0], args[1]
	v, err := value.Lt(left, right)
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

func Sort(ctx context.Context, args []value.Value) (value.Value, bool, error) {
	arr, err := value.ToValueArray(args[0])
	if err != nil {
		return nil, false, err
	}

	less := args[1]
	if less.Kind() == value.NullKind {
		less = NativeFuncValue(defaultLess)
	}

	var errs []error
	sort.Slice(arr, func(i, j int) bool {
		ret, ok, err := value.Call(ctx, less, value.CallArgument{
			Positional: true,
			Value:      arr[i],
		}, value.CallArgument{
			Positional: true,
			Value:      arr[j],
		})
		if err != nil {
			errs = append(errs, err)
		} else if !ok {
			return false
		}
		b, err := value.ToBool(ret)
		if err != nil {
			errs = append(errs, err)
		}
		return b
	})

	return value.NewValue(arr), true, errors.Join(errs...)
}

func mergeValue(left, right value.Value) (value.Value, error) {
	if left == nil {
		return right, nil
	} else if right == nil {
		return left, nil
	}

	if left.Kind() != value.ObjectKind || right.Kind() != value.ObjectKind {
		return right, nil
	}

	merged := map[string]any{}
	leftKeys, err := value.Keys(left)
	if err != nil {
		return nil, err
	}

	for _, leftKey := range leftKeys {
		leftValue, ok, err := value.Lookup(left, value.NewValue(leftKey))
		if err != nil {
			return nil, err
		} else if !ok {
			leftValue = nil
		}

		rightValue, ok, err := value.Lookup(right, value.NewValue(leftKey))
		if err != nil {
			return nil, err
		} else if !ok {
			rightValue = nil
		}

		merged[leftKey], err = mergeValue(leftValue, rightValue)
		if err != nil {
			return nil, err
		}
	}

	rightKeys, err := value.Keys(right)
	if err != nil {
		return nil, err
	}

	for _, rightKey := range rightKeys {
		_, done := merged[rightKey]
		if done {
			continue
		}

		rightValue, ok, err := value.Lookup(right, value.NewValue(rightKey))
		if err != nil {
			return nil, err
		} else if ok {
			merged[rightKey] = rightValue
		}
	}
	return value.NewValue(merged), nil
}

func Merge(_ context.Context, args []value.Value) (value.Value, bool, error) {
	left, right := args[0], args[1]
	merged, err := mergeValue(left, right)
	return merged, true, err
}

func IndexOf(_ context.Context, args []value.Value) (value.Value, bool, error) {
	if args[0].Kind() == value.StringKind {
		str, err := value.ToString(args[0])
		if err != nil {
			return nil, false, err
		}

		part, err := value.ToString(args[1])
		if err != nil {
			return nil, false, err
		}

		return value.NewValue(strings.Index(str, part)), true, nil
	}

	arr, err := value.ToValueArray(args[0])
	if err != nil {
		return nil, false, err
	}

	for i, item := range arr {
		if isTrue(value.Eq(item, args[1])) {
			return value.NewValue(i), true, nil
		}
	}

	return value.NewValue(-1), true, nil
}

func TrimSuffix(_ context.Context, args []value.Value) (value.Value, bool, error) {
	str, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	suffix, err := value.ToString(args[1])
	if err != nil {
		return nil, false, err
	}

	return value.NewValue(strings.TrimSuffix(str, suffix)), true, nil
}

func Trim(_ context.Context, args []value.Value) (value.Value, bool, error) {
	str, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	return value.NewValue(strings.TrimSpace(str)), true, nil
}

func TrimPrefix(_ context.Context, args []value.Value) (value.Value, bool, error) {
	str, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	prefix, err := value.ToString(args[1])
	if err != nil {
		return nil, false, err
	}

	return value.NewValue(strings.TrimPrefix(str, prefix)), true, nil
}

func ToLower(_ context.Context, args []value.Value) (value.Value, bool, error) {
	str, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	return value.NewValue(strings.ToLower(str)), true, nil
}

func ToUpper(_ context.Context, args []value.Value) (value.Value, bool, error) {
	str, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	return value.NewValue(strings.ToUpper(str)), true, nil
}

func StartsWith(_ context.Context, args []value.Value) (value.Value, bool, error) {
	str, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	prefix, err := value.ToString(args[1])
	if err != nil {
		return nil, false, err
	}

	return value.NewValue(strings.HasPrefix(str, prefix)), true, nil
}

func EndsWith(_ context.Context, args []value.Value) (value.Value, bool, error) {
	prefix, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	suffix, err := value.ToString(args[1])
	if err != nil {
		return nil, false, err
	}

	return value.NewValue(strings.HasSuffix(prefix, suffix)), true, nil
}

func Join(_ context.Context, args []value.Value) (value.Value, bool, error) {
	list, err := value.ToValueArray(args[0])
	if err != nil {
		return nil, false, err
	}

	sep, err := value.ToString(args[1])
	if err != nil {
		return nil, false, err
	}

	var parts []string
	for _, item := range list {
		s, err := value.ToString(item)
		if err != nil {
			return nil, false, err
		}
		parts = append(parts, s)
	}

	return value.NewValue(strings.Join(parts, sep)), true, nil
}

func Replace(_ context.Context, args []value.Value) (value.Value, bool, error) {
	str, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	match, err := value.ToString(args[1])
	if err != nil {
		return nil, false, err
	}

	replacement, err := value.ToString(args[2])
	if err != nil {
		return nil, false, err
	}

	count, err := value.ToInt(args[3])
	if err != nil {
		return nil, false, err
	}

	return value.NewValue(strings.Replace(str, match, replacement, int(count))), true, nil
}

func Split(_ context.Context, args []value.Value) (value.Value, bool, error) {
	str, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	sep, err := value.ToString(args[1])
	if err != nil {
		return nil, false, err
	}

	count, err := value.ToInt(args[2])
	if err != nil {
		return nil, false, err
	}

	var result value.Array
	for _, s := range strings.SplitN(str, sep, int(count)) {
		result = append(result, value.NewValue(s))
	}

	return result, true, nil
}

func Describe(ctx context.Context, args []value.Value) (value.Value, bool, error) {
	data, err := json.Marshal(args[0])
	if err != nil {
		return nil, false, err
	}
	result := map[string]any{}
	err = json.Unmarshal(data, &result)
	return value.NewValue(result), true, err
}

func IsA(ctx context.Context, args []value.Value) (value.Value, bool, error) {
	schema := args[1]
	val := args[0]
	_, _, err := value.Call(ctx, schema, value.CallArgument{
		Positional: true,
		Value:      val,
	})
	return value.NewValue(err == nil), true, nil
}

func ToTitle(_ context.Context, args []value.Value) (value.Value, bool, error) {
	s, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}
	if s == "" {
		return value.NewValue(""), true, nil
	}

	return value.NewValue(strings.ToTitle(s[:1]) + s[1:]), true, nil
}

func FileExt(_ context.Context, args []value.Value) (value.Value, bool, error) {
	s, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}
	return value.NewValue(path.Ext(s)), true, nil
}

func Cut(_ context.Context, args []value.Value) (value.Value, bool, error) {
	str, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}
	separator, err := value.ToString(args[1])
	if err != nil {
		return nil, false, err
	}
	before, after, found := strings.Cut(str, separator)
	return value.NewValue(map[string]any{
		"before": before,
		"after":  after,
		"found":  found,
	}), true, nil
}

func Basename(_ context.Context, args []value.Value) (value.Value, bool, error) {
	s, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}
	return value.NewValue(path.Base(s)), true, nil
}

func Dirname(_ context.Context, args []value.Value) (value.Value, bool, error) {
	s, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}
	return value.NewValue(path.Dir(s)), true, nil
}

func PathJoin(_ context.Context, args []value.Value) (value.Value, bool, error) {
	paths, err := value.ToValueArray(args[0])
	if err != nil {
		return nil, false, err
	}
	sep, err := value.ToString(args[1])
	if err != nil {
		return nil, false, err
	}
	if sep != "/" {
		return nil, false, fmt.Errorf("only / separator is currently supported")
	}

	var pathStrings []string
	for _, path := range paths {
		s, err := value.ToString(path)
		if err != nil {
			return nil, false, err
		}
		pathStrings = append(pathStrings, s)
	}

	return value.NewValue(path.Join(pathStrings...)), true, nil
}

func JoinHostPort(_ context.Context, args []value.Value) (value.Value, bool, error) {
	host, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	var port string
	if args[1].Kind() == value.NumberKind {
		i, err := value.ToInt(args[1])
		if err != nil {
			return nil, false, err
		}
		port = strconv.Itoa(int(i))
	} else {
		port, err = value.ToString(args[1])
	}

	result := net.JoinHostPort(host, port)
	return value.NewValue(result), true, nil
}

func SplitHostPort(_ context.Context, args []value.Value) (value.Value, bool, error) {
	s, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return nil, false, err
	}
	return value.NewValue(value.Array{
		value.NewValue(host),
		value.NewValue(port),
	}), true, nil
}

func FromJSON(_ context.Context, args []value.Value) (value.Value, bool, error) {
	s, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	data := map[string]any{}
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return nil, false, err
	}
	return value.NewValue(data), true, nil
}

func ToJSON(_ context.Context, args []value.Value) (value.Value, bool, error) {
	nv, ok, err := value.NativeValue(args[0])
	if err != nil || !ok {
		return nil, ok, err
	}

	data, err := json.Marshal(nv)
	if err != nil {
		return nil, false, err
	}

	return value.NewValue(string(data)), true, nil
}

func ToHex(_ context.Context, args []value.Value) (value.Value, bool, error) {
	s, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	data := hex.EncodeToString([]byte(s))
	return value.NewValue(data), false, nil
}

func FromHex(_ context.Context, args []value.Value) (value.Value, bool, error) {
	s, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	data, err := hex.DecodeString(s)
	if err != nil {
		return nil, false, err
	}

	if !utf8.Valid(data) {
		return nil, false, fmt.Errorf("invalid utf8 content after hex decode")
	}

	return value.NewValue(string(data)), false, nil
}

func Base64(_ context.Context, args []value.Value) (value.Value, bool, error) {
	s, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	data := base64.StdEncoding.EncodeToString([]byte(s))
	return value.NewValue(data), false, nil
}

func Base64Decode(_ context.Context, args []value.Value) (value.Value, bool, error) {
	s, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, false, err
	}

	if !utf8.Valid(data) {
		return nil, false, fmt.Errorf("invalid utf8 content after base64 decode")
	}

	return value.NewValue(string(data)), false, nil
}

func Sha512sum(_ context.Context, args []value.Value) (value.Value, bool, error) {
	s, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	h := sha512.Sum512([]byte(s))
	return value.NewValue(hex.EncodeToString(h[:])), true, nil
}

func Sha256sum(_ context.Context, args []value.Value) (value.Value, bool, error) {
	s, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	h := sha256.Sum256([]byte(s))
	return value.NewValue(hex.EncodeToString(h[:])), true, nil
}

func Sha1sum(_ context.Context, args []value.Value) (value.Value, bool, error) {
	s, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	h := sha1.Sum([]byte(s))
	return value.NewValue(hex.EncodeToString(h[:])), true, nil
}

func ToYAML(_ context.Context, args []value.Value) (value.Value, bool, error) {
	v, ok, err := value.NativeValue(args[0])
	if err != nil || !ok {
		return nil, ok, err
	}

	data, err := yaml.Marshal(v)
	if err != nil {
		return nil, false, err
	}

	return value.NewValue(string(data)), true, nil
}

func FromYAML(_ context.Context, args []value.Value) (value.Value, bool, error) {
	s, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	data := map[string]any{}

	err = yaml.Unmarshal([]byte(s), &data)
	if err != nil {
		return nil, false, err
	}
	return value.NewValue(data), true, nil
}

func Atoi(_ context.Context, args []value.Value) (value.Value, bool, error) {
	str, err := value.ToString(args[0])
	if err != nil {
		return nil, false, err
	}

	i, err := strconv.Atoi(str)
	return value.NewValue(i), true, err
}

func Int() value.Value {
	return &value.TypeSchema{
		KindValue: value.NumberKind,
		Constraints: []value.Constraint{
			{
				Op: value.MustBeIntOp,
			},
		},
	}
}

func Any(kinds map[string]any) value.Value {
	result := &value.TypeSchema{
		KindValue: value.UnionKind,
	}
	for _, name := range []string{"bool", "number", "string", "object", "array", "null"} {
		result.Alternates = append(result.Alternates, kinds[name].(*value.TypeSchema))
	}
	return result
}

func Enum(_ context.Context, args []value.Value) (value.Value, bool, error) {
	var result *value.TypeSchema

	if len(args) == 0 {
		return nil, false, fmt.Errorf("can not create an empty enum")
	}

	for _, arg := range args {
		s, err := value.ToString(arg)
		if err != nil {
			return nil, false, err
		}
		next := value.TypeSchema{
			KindValue: value.StringKind,
			Constraints: []value.Constraint{
				{
					Op:    "==",
					Right: value.NewValue(s),
				},
			},
		}
		if result == nil {
			result = &next
		} else {
			result.Alternates = append(result.Alternates, &next)
		}
	}

	return result, true, nil
}

func Contains(ctx context.Context, args []value.Value) (value.Value, bool, error) {
	collection := args[0]
	if collection.Kind() == value.ObjectKind {
		v, ok, err := value.Lookup(collection, args[1])
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return value.False, true, nil
		}
		if undef := value.IsUndefined(v); undef != nil {
			return undef, true, nil
		}
		return value.True, true, nil
	} else if collection.Kind() == value.ArrayKind {
		values, err := value.ToValueArray(collection)
		if err != nil {
			return nil, false, err
		}
		for _, v := range values {
			if ret, err := value.Eq(v, args[1]); err != nil {
				return nil, false, err
			} else if ret.Kind() == value.UndefinedKind {
				return ret, true, nil
			} else {
				b, err := value.ToBool(ret)
				if err != nil {
					return nil, false, err
				}
				if b {
					return value.True, true, nil
				}
			}
		}
	}

	idx, ok, err := IndexOf(ctx, args)
	if err != nil || !ok {
		return nil, ok, err
	}
	ret, err := value.Neq(idx, value.NewValue(-1))
	return ret, true, err

}

func Range(_ context.Context, args []value.Value) (value.Value, bool, error) {
	var (
		start  = args[0]
		end    = args[1]
		step   = args[2]
		err    error
		result value.Array
	)

	if end.Kind() == value.NullKind {
		end = start
		start = value.NewValue(0)
	}

	var op func(value.Value, value.Value) (value.Value, error)

	if isTrue(value.Eq(step, value.NewValue(0))) {
		return nil, false, fmt.Errorf("step can not be 0")
	}

	if isTrue(value.Lt(step, value.NewValue(0))) {
		op = value.Gt
	} else {
		op = value.Lt
	}

	for isTrue(op(start, end)) {
		result = append(result, start)
		start, err = value.Add(start, step)
		if err != nil {
			return nil, false, err
		}
	}

	return result, true, nil
}

func isTrue(v value.Value, _ error) bool {
	b, _ := value.ToBool(v)
	return b
}

type LoopControl struct {
	Skip  bool
	Break bool
	Value value.Value
}

func (l *LoopControl) withValue(v value.Value) value.Value {
	if l == nil {
		return v
	}
	cp := *l
	cp.Value = v
	return &cp
}

func (l *LoopControl) combine(lc *LoopControl) *LoopControl {
	if l == nil {
		return lc
	} else if lc == nil {
		return l
	}
	return &LoopControl{
		Skip:  l.Skip || lc.Skip,
		Break: l.Break || lc.Break,
	}
}

func (l *LoopControl) Kind() value.Kind {
	return value.UndefinedKind
}

func Skip() value.Value {
	return &LoopControl{
		Skip: true,
	}
}

func Break() value.Value {
	return &LoopControl{
		Break: true,
	}
}
