# Acorn Markup Language

Acorn Markup Language (AML) is the markup language that is used to write Acornfiles in [Acorn](https://acorn.io). The
language is a configuration file format to describe json data using expressions, functions, and has a
corresponding schema language to validate data.

## JSON Superset

AML is a superset of JSON. This means that any valid JSON is also valid AML. All AML after being evaluated produces
valid JSON.

### Syntax simplifications

```cue
// Comments are allowed in AML
// Notice this document doesn't start with a curly brace. The outer '{' and '}' are optional and will
// still yield a valid JSON object.

// Keys that start with a letter and only have letters, numbers, and underscores can be written
// without quotes.
simpleKey: "value"

// Object that have a single key can be written without curly braces.
simpleObject: singleKey: "value"

// Commas at that end of the line are optional. Also a trailing comma is allowed.
trailingComma: "value",
```
The above AML will evaluate to the following JSON:
```json
{
  "simpleKey": "value",
  "simpleObject": {
    "singleKey": "value"
  },
  "trailingComma": "value"
}
```

## Native Data Types

The native data type in AML are the exact same as JSON, which are as below

```cue
aNumber: 1
aString: "string"
aBoolean: true
aNull: null
anArray: [1, "string", true, null]
anObject: {
  "key": "value"
}
```

### Strings
```cue
multiline: """
    Multiline string in AML are written using triple quotes. The body of the string must still be 
    escaped like a normal string, but the triple quotes allow for newlines and quotes to be written.
    The following " does not need to be escaped, but \n \t and \r will still be treated as newlinee,
    tab, and carriage return respectively.
    
    If all of the lines of the multiline string are indented with the same amount of whitespace the
    leading whitespace will be trimmed from the evaluated string.
    """
rawString: `This is a raw string. The body of the string is not escaped, but the backtick must be`

rawMultiLine: ```
This is a raw multiline string. The body of the string is not escaped,
but the backtick must be.```
```


### Numbers
```cue
// Integer, which is it's type is just number
aInteger: 1

// This is the same 1000000, the _ is just a separator that is ignored. Numbers can have as many _ as you
// want, but can not start or end with an underscore.
aIntegerWithUnderscore: 1_000_000

// A suffix of K, M, G, T, or P can be added implying the number is multiplied by 1000, 1000000, 1000000000, etc
oneMillion: 1M

// A suffix of Ki, Mi, Gi, Ti, or Pi can be added implying the number is multiplied by 1024, 1048576, 1073741824, etc
megabyte: 1Mi

// A suffix of K, M, G, T, or P can be added implying the number is multiplied by 1000, 1000000, 1000000000,

// Float, which is it's type is just number
aFloat: 1.0

// Scientific notation, following the same format as JSON
aFloatWithE: 1.0e10
```

### Multiple Definitions
```cue
anObject: {
    key: "value"
}

// Objects can be defined multiple as long as the values are new or the same
anObject: {
    aNewKey: "value"
    key: "value"
}

anObject: thirdKey: "value"

aNumber: 4
aNumber: 4
```
The above AML will produce the following JSON.
```json
{
  "anObject": {
    "aNewKey": "value",
    "key": "value",
    "thirdKey": "value"
  },
  "aNumber": 4
}
```

## Expressions

### Math
```cue
addition: 1 + 2
subtraction: 1 - 2
multiplication: 1 * 2
division: 1 / 2
parens: (1 + 2) * 3
```

### Comparisons
```cue
lessThan: 1 < 2 
lessThanEquals: 1 <= 2
greaterThan: 1 > 2
greaterThanEquals: 1 >= 2
equals: 1 == 2
notEquals: 1 != 2
regexpMatch: "string" =~ "str.*"
regexpNotMatch: "string" !~ "str.*"
```

### References, Lookup
```cue
value: {
    nested: 1
}
reference: value.nested
```

### Index, Slice
```cue
array: [1, 2, 3, 4, 5]
index: array[0]
// Slices are inclusive of the start index and exclusive of the end index
slice: array[0:2]
tail: array[2:]
head: array[:2]
```

### String Interpolation
```cue
value: 1
// Interpolation in a string starts with \( and ends with ) and can contain any expression.
output: "the value is \(value)"

// Interpolation can also be used in keys
"key\(value)": "dynamic key"
```
The above will produce the following JSON.
```json
{
  "value": 1,
  "output": "the value is 1",
  "key1": "dynamic key"
}
```

### Conditions, If
```cue
value: 1

if value == 2 {
    output: "value is 2"
} else if value == 3 {
    output: "value is 3"
} else {
    output: "value is not 2 or 3"
}
```
The above will produce the following JSON.
```json
{
  "value": 1,
  "output": "value is not 2 or 3"
}
```

### Loops, For
```cue
for i, v in ["a", "b", "c"] {
    // This key is using string interpolation which is described above
    "key\(i)": v
}

for v in ["a", "b", "c"] {
    // This key is using string interpolation which is described above
    "key\(v)": v
}
```
The above will produce the following JSON.
```json
{
  "key0": "a",
  "key1": "b",
  "key2": "c",
  "keya": "a",
  "keyb": "b",
  "keyc": "c"
}
```

### List comprehension
```cue
list: [1, 2, 3, 4, 5]

listOfInts: [for i in list if i > 2 {
    i * 2
}]

listOfObjects: [for i in list if i > 2 {
    "key\(i)": i * 2
}]
```
The above will produce the following JSON
```json
{
  "list": [1, 2, 3, 4, 5],
  "listOfInts": [6, 8, 10],
  "listOfObjects": [
    {"key3": 6},
    {"key4": 8},
    {"key5": 10}
  ]
}
```

### Let
```cue
// Let is used to define a variable that can be used in the current scope but is not outputed
let x: 1
y: x
```
The above will produce the following JSON
```json
{
  "y": 1
}
```

### Embedding
```cue
subObject: {
    a: 1
}

parentObject: {
    // This object will be embedded into the parent object allowing composition
    subObject
    
    b: 2
}
```
The above will produces the following JSON
```json
{
   "subObject": {
        "a": 1
    },
    "parentObject": {
        "a": 1,
        "b": 2
    }
}
```

### Functions
```cue
// Functions are defined using the function keyword
myAppend: function {
    // Args are defined using the args keyword
    args: {
        // Args are defined using the name of the arg and the type of the arg. The type of the arg
        // follows the schema syntax described later in this document
        head: string
        tail: string
    }
    
    someVariable: "some value"
    
    // The return of the function should be defined using the return key
    return: args.head + args.tail + someVariable
}

// The arguments will be applied by the order they are assigned
callByPosition: myAppend("head", "tail")

// The arguments can also be applied by name
callByName: myAppend(tail: "tail", head: "head")
```
The above will produce the following JSON
```json
{
  "callByPosition": "headtailsome value",
  "callByName": "headtailsome value"
}
```

## Evaluation Args and Profiles

When evaluating AML using the go library or CLI you can pass in args and profiles. Args are used to pass in parameterized
data and profiles are used to provide alternative set of default values.

```cue
// Args are defined using the args keyword at the top level of the document.  They can not be nested
// in any scopes.
args: {
    // A name you want outputed
    someName: "default"
}

profiles: {
    one: {
        someName: "Bob"
    }
    two: {
        someName: "Alice"
    }
}

theName: args.someName
```

Running the above AML with the following command
```shell
aml eval file.acorn --someName John
```
Will produce the following JSON
```json
{
  "theName": "John"
}
```
The following command
```shell
aml eval file.acorn --profile one
```
Will produce the following JSON
```json
{
  "theName": "Bob"
}
```
The following command
```shell
aml eval file.acorn --profile two
```
Will produce the following JSON
```json
{
  "theName": "Alice"
}
```

### Args and profiles in help text in CLI
The following command
```shell
aml eval file.acorn --help
```
Will produce the following output
```shell
Usage of file.acorn:
      --profile strings   Available profiles (one, two)
      --someName string   An name you want outputed
```

## Schema

AML defines a full schema language used to validate the structure of the data. The schema language is designed to be
written in a style that is similar to the data it is validating.

Schema can be validated using the go library or CLI. The CLI can be used as follows.
```shell
aml eval --schema-file schema.acorn file.acorn
```

### Simple Data Fields
```cue
// A key call requiredString must in the data and must be a string
requiredString: string
// A key call optionalString may or may not be in the data and must be a string if it is
optionalString?: string
// A key call requiredNumber must in the data and must be a number
requiredNumber: number
// A key call optionalNumber may or may not be in the data and must be a number if it is
optionalNumber?: number
// A key call requiredBool must in the data and must be a bool
requiredBool: bool
// A key call optionalBool may or may not be in the data and must be a bool if it is
optionalBool?: bool
```

### Objects
```cue
// An object is defined looking like a regular object
someObject: {
    fieldOne: string
    fieldTwo: number
    
    // Dynamic keys can be matched using regular expressions.  The regular expression will only be checked
    // if no other required or optional keys match first
    match "field.*": string
}
```

### Arrays
```cue
arrayOfStrings: [string]
arrayOfNumbers: [number]
arrayOfObjects: [{key: "value"}]

// The below is interpreted as a key someArray is required, must be an array and the
// values of the array must match the schema `string` or `{key: "value"}`
mixedArrayOfStringAndObject: [string, {key: "value"}]
```

### Default values
```cue
// The following schema means that this key is required and must be a string, but if it is not in the
// data the default value of "hi" will be used.
defaultedString: "hi"
// This can also be written using the default keyword, but is unnecessary for in most situations, but
// may more clearly describe your intent.
defaultedStringWithKeyword: default "hi"
```
### Conditions and Expressions

Conditions `>`, `>=`, `<`, `<=`, `==`, `!=`, `=~`, `!~` can be used in the schema to validate the data. The condition expressions
must be written as the type is on the left and the condition is on the right.
```cue
aPositionNumber: number > 0
anExplicitConstant: string == "value"
aRegex: string =~ "str.*"
```
Complex expressions can be written by using the operators `&&` and `||` and using parens to group expressions.
```cue
aNumberRange: number > 0 && number < 10 || default 1
```
### Types (psuedo)
The following pattern can be used to define reusable types.  Custom types are not a first class object in the
language but instead objects will schema fields can be reused.
```cue
// By convention put types in a let field named types. This ensurs the objects
// are not outputed in the final JSON
let types: {
    // Types by convention start with an uppercase letter following PascalCase
    StringItem: {
        item: string
    }
    NumberItem: {
        item: number
    }
    
    Item: types.StringItem || types.NumberItem
}

items: [types.Item]
```

## Examples

As this is the language used by [Acorn](https://github.com/acorn-io/runtime), Acornfiles are a great place to look for
examples of AML syntax. Try this [GitHub search](https://github.com/search?q=path%3A**%2FAcornfile&type=code&ref=advsearch)

For an example of schema, you can refer the [schema file used to validate Acornfile](https://github.com/acorn-io/runtime/blob/main/pkg/appdefinition/app.acorn)
which is quite a complete example of most all schema features.

## License

It's Apache 2.0. See [LICENSE](LICENSE).
