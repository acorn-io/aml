package schema

type Kind string

var (
	StringKind = Kind("string")
	BoolKind   = Kind("bool")
	NumberKind = Kind("number")
	ArrayKind  = Kind("array")
	ObjectKind = Kind("object")
)
