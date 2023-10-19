package value

const (
	NullKind      = Kind("null")
	StringKind    = Kind("string")
	BoolKind      = Kind("bool")
	NumberKind    = Kind("number")
	ArrayKind     = Kind("array")
	ObjectKind    = Kind("object")
	FuncKind      = Kind("func")
	SchemaKind    = Kind("schema")
	UnionKind     = Kind("union")
	UndefinedKind = Kind("undefined")
)

var BuiltinKinds = []Kind{
	NullKind,
	StringKind,
	BoolKind,
	NumberKind,
	ArrayKind,
	ObjectKind,
	FuncKind,
	SchemaKind,
}

type Kind string

type Value interface {
	Kind() Kind
}

// IsSimpleKind returns true if the kind is a string, number, or bool.
func IsSimpleKind(kind Kind) bool {
	switch kind {
	case StringKind, BoolKind, NumberKind, NullKind:
		return true
	}
	return false
}
