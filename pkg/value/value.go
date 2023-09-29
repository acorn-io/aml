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
	UndefinedKind = Kind("undefined")
)

var Kinds = []Kind{
	NullKind,
	StringKind,
	BoolKind,
	NumberKind,
	ArrayKind,
	ObjectKind,
	FuncKind,
	SchemaKind,
	UndefinedKind,
}

type Kind string

type Value interface {
	Kind() Kind
}

// IsSimpleKind returns true if the kind is a string, number, or bool.
func IsSimpleKind(kind Kind) bool {
	switch kind {
	case StringKind, BoolKind, NumberKind:
		return true
	}
	return false
}
