package analysis

import (
	"sort"

	"github.com/google/go-jsonnet/ast"
)

var StdLibFunctions = map[string]*Function{
	"extVar": {
		Comment:    []string{"If an external variable with the given name was defined, return its string value. Otherwise, raise an error."},
		ReturnType: AnyType,
		Params: []Param{
			{Name: "x", Type: StringType},
		},
	},
	"thisFile": {
		Comment:    []string{"Note that this is a field. It contains the current Jsonnet filename as a string."},
		ReturnType: StringType,
		Params:     []Param{},
	},
	"type": {
		Comment:    []string{"Return a string that indicates the type of the value. The possible return values are:\n\"array\", \"boolean\", \"function\", \"null\", \"number\", \"object\", and \"string\".\n\nThe following functions are also available and return a boolean:\n`std.isArray(v)`, `std.isBoolean(v)`, `std.isFunction(v)`,\n`std.isNumber(v)`, `std.isObject(v)`, and\n`std.isString(v)`."},
		ReturnType: StringType,
		Params: []Param{
			{Name: "x", Type: AnyType},
		},
	},
	"isObject": {
		Comment:    []string{"Returns `true` if `x` is of type 'object'."},
		ReturnType: BooleanType,
		Params:     []Param{{Name: "x", Type: AnyType}},
	},
	"isNumber": {
		Comment:    []string{"Returns `true` if `x` is of type 'number'."},
		ReturnType: BooleanType,
		Params:     []Param{{Name: "x", Type: AnyType}},
	},
	"isString": {
		Comment:    []string{"Returns `true` if `x` is of type 'string'."},
		ReturnType: BooleanType,
		Params:     []Param{{Name: "x", Type: AnyType}},
	},
	"isBoolean": {
		Comment:    []string{"Returns `true` if `x` is of type 'boolean'."},
		ReturnType: BooleanType,
		Params:     []Param{{Name: "x", Type: AnyType}},
	},
	"isArray": {
		Comment:    []string{"Returns `true` if `x` is of type 'array'."},
		ReturnType: BooleanType,
		Params:     []Param{{Name: "x", Type: AnyType}},
	},
	"isFunction": {
		Comment:    []string{"Returns `true` if `x` is of type 'function'."},
		ReturnType: BooleanType,
		Params:     []Param{{Name: "x", Type: AnyType}},
	},
	"length": {
		Comment:    []string{"Depending on the type of the value given, either returns the number of elements in the\narray, the number of codepoints in the string, the number of parameters in the function, or\nthe number of fields in the object. Raises an error if given a primitive value, i.e.\n`null`, `true` or `false`."},
		ReturnType: NumberType,
		Params: []Param{
			{Name: "x", Type: AnyType},
		},
	},
	"get": {
		Comment:    []string{"Returns the object's field if it exists or default value otherwise.\n`inc_hidden` controls whether to include hidden fields."},
		ReturnType: AnyType,
		Params: []Param{
			{Name: "o", Type: ObjectType},
			{Name: "f", Type: StringType},
			{Name: "default", Type: AnyType, Default: &ast.LiteralNull{}},
			{Name: "inc_hidden", Type: AnyType, Default: &ast.LiteralBoolean{Value: true}},
		},
	},
	"objectHas": {
		Comment:    []string{"Returns `true` if the given object has the field (given as a string), otherwise\n`false`. Raises an error if the arguments are not object and string\nrespectively. Returns false if the field is hidden."},
		ReturnType: BooleanType,
		Params: []Param{
			{Name: "o", Type: ObjectType},
			{Name: "f", Type: StringType},
		},
	},
	"objectFields": {
		Comment:    []string{"Returns an array of strings, each element being a field from the given object. Does not include\nhidden fields."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "o", Type: ObjectType},
		},
	},
	"objectValues": {
		Comment:    []string{"Returns an array of the values in the given object. Does not include hidden fields."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "o", Type: ObjectType},
		},
	},
	"objectHasAll": {
		Comment:    []string{"As `std.objectHas` but also includes hidden fields."},
		ReturnType: BooleanType,
		Params: []Param{
			{Name: "o", Type: ObjectType},
			{Name: "f", Type: StringType},
		},
	},
	"objectFieldsAll": {
		Comment:    []string{"As `std.objectFields` but also includes hidden fields."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "o", Type: ObjectType},
		},
	},
	"objectValuesAll": {
		Comment:    []string{"As `std.objectValues` but also includes hidden fields."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "o", Type: ObjectType},
		},
	},
	"prune": {
		Comment:    []string{"Recursively remove all \"empty\" members of `a`. \"Empty\" is defined as zero\nlength \\`arrays\\`, zero length \\`objects\\`, or \\`null\\` values.\nThe argument `a` may have any type."},
		ReturnType: AnyType,
		Params: []Param{
			{Name: "a", Type: AnyType},
		},
	},
	"mapWithKey": {
		Comment:    []string{"Apply the given function to all fields of the given object, also passing\nthe field name. The function `func` is expected to take the\nfield name as the first parameter and the field value as the second."},
		ReturnType: ObjectType,
		Params: []Param{
			{Name: "func", Type: FunctionType},
			{Name: "obj", Type: ObjectType},
		},
	},
	"clamp": {
		Comment:    []string{"Clamp a value to fit within the range \\[ `minVal`, `maxVal`\\].\nEquivalent to `std.max(minVal, std.min(x, maxVal))`.\n\nExamples:\n\n  Input: `std.clamp(-3, 0, 5)`\n  Output: `0`\n\n  Input: `std.clamp(4, 0, 5)`\n  Output: `4`\n\n  Input: `std.clamp(7, 0, 5)`\n  Output: `5`"},
		ReturnType: AnyType,
		Params: []Param{
			{Name: "x", Type: AnyType},
			{Name: "minVal", Type: AnyType},
			{Name: "maxVal", Type: AnyType},
		},
	},
	"assertEqual": {
		Comment:    []string{"Ensure that `a == b`. Returns `true` or throws an error message."},
		ReturnType: BooleanType,
		Params: []Param{
			{Name: "a", Type: AnyType},
			{Name: "b", Type: AnyType},
		},
	},
	"toString": {
		Comment:    []string{"Convert the given argument to a string."},
		ReturnType: StringType,
		Params: []Param{
			{Name: "a", Type: AnyType},
		},
	},
	"codepoint": {
		Comment:    []string{"Returns the positive integer representing the unicode codepoint of the character in the\ngiven single-character string. This function is the inverse of `std.char(n)`."},
		ReturnType: NumberType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"char": {
		Comment:    []string{"Returns a string of length one whose only unicode codepoint has integer id `n`.\nThis function is the inverse of `std.codepoint(str)`."},
		ReturnType: StringType,
		Params: []Param{
			{Name: "n", Type: NumberType},
		},
	},
	"substr": {
		Comment:    []string{"Returns a string that is the part of `s` that starts at offset `from`\nand is `len` codepoints long. If the string `s` is shorter than\n`from+len`, the suffix starting at position `from` will be returned."},
		ReturnType: StringType,
		Params: []Param{
			{Name: "str", Type: StringType},
			{Name: "from", Type: NumberType},
			{Name: "len", Type: NumberType},
		},
	},
	"findSubstr": {
		Comment:    []string{"Returns an array that contains the indexes of all occurrences of `pat` in\n`str`."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "pat", Type: StringType},
			{Name: "str", Type: StringType},
		},
	},
	"startsWith": {
		Comment:    []string{"Returns whether the string a is prefixed by the string b."},
		ReturnType: BooleanType,
		Params: []Param{
			{Name: "a", Type: StringType},
			{Name: "b", Type: StringType},
		},
	},
	"endsWith": {
		Comment:    []string{"Returns whether the string a is suffixed by the string b."},
		ReturnType: BooleanType,
		Params: []Param{
			{Name: "a", Type: StringType},
			{Name: "b", Type: StringType},
		},
	},
	"stripChars": {
		Comment:    []string{"Removes characters `chars` from the beginning and from the end of `str`.\n\nExamples:\n\n  Input: `std.stripChars(\" test test test     \", \" \")`\n  Output: `\"test test test\"`\n\n  Input: `std.stripChars(\"aaabbbbcccc\", \"ac\")`\n  Output: `\"bbbb\"`\n\n  Input: `std.stripChars(\"cacabbbbaacc\", \"ac\")`\n  Output: `\"bbbb\"`"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "str", Type: StringType},
			{Name: "chars", Type: StringType},
		},
	},
	"lstripChars": {
		Comment:    []string{"Removes characters `chars` from the beginning of `str`.\n\nExamples:\n\n  Input: `std.lstripChars(\" test test test     \", \" \")`\n  Output: `\"test test test     \"`\n\n  Input: `std.lstripChars(\"aaabbbbcccc\", \"ac\")`\n  Output: `\"bbbbcccc\"`\n\n  Input: `std.lstripChars(\"cacabbbbaacc\", \"ac\")`\n  Output: `\"bbbbaacc\"`"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "str", Type: StringType},
			{Name: "chars", Type: StringType},
		},
	},
	"rstripChars": {
		Comment:    []string{"Removes characters `chars` from the end of `str`.\n\nExamples:\n\n  Input: `std.rstripChars(\" test test test     \", \" \")`\n  Output: `\" test test test\"`\n\n  Input: `std.rstripChars(\"aaabbbbcccc\", \"ac\")`\n  Output: `\"aaabbbb\"`\n\n  Input: `std.rstripChars(\"cacabbbbaacc\", \"ac\")`\n  Output: `\"cacabbbb\"`"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "str", Type: StringType},
			{Name: "chars", Type: StringType},
		},
	},
	"split": {
		Comment:    []string{"Split the string `str` into an array of strings, divided by the string\n`c`.\n\nNote: Versions up to and including 0.18.0 require `c` to be a single character.\n\nExamples:\n\n  Input: `std.split(\"foo/_bar\", \"/_\")`\n  Output: `[\"foo\",\"bar\"]`\n\n  Input: `std.split(\"/_foo/_bar\", \"/_\")`\n  Output: `[\"\",\"foo\",\"bar\"]`"},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "str", Type: StringType},
			{Name: "c", Type: StringType},
		},
	},
	"splitLimit": {
		Comment:    []string{"As `std.split(str, c)` but will stop after `maxsplits` splits, thereby the largest\narray it will return has length `maxsplits + 1`. A limit of `-1` means unlimited.\n\nNote: Versions up to and including 0.18.0 require `c` to be a single character.\n\nExamples:\n\n  Input: `std.splitLimit(\"foo/_bar\", \"/_\", 1)`\n  Output: `[\"foo\",\"bar\"]`\n\n  Input: `std.splitLimit(\"/_foo/_bar\", \"/_\", 1)`\n  Output: `[\"\",\"foo/_bar\"]`"},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "str", Type: StringType},
			{Name: "c", Type: StringType},
			{Name: "maxsplits", Type: NumberType},
		},
	},
	"splitLimitR": {
		Comment:    []string{"As `std.splitLimit(str, c, maxsplits)` but will split from right to left.\n\nExamples:\n\n  Input: `std.splitLimitR(\"/_foo/_bar\", \"/_\", 1)`\n  Output: `[\"/_foo\",\"bar\"]`"},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "str", Type: StringType},
			{Name: "c", Type: StringType},
			{Name: "maxsplits", Type: NumberType},
		},
	},
	"strReplace": {
		Comment:    []string{"Returns a copy of the string in which all occurrences of string `from` have been\nreplaced with string `to`.\n\nExamples:\n\n  Input: `std.strReplace('I like to skate with my skateboard', 'skate', 'surf')`\n  Output: `\"I like to surf with my surfboard\"`"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "str", Type: StringType},
			{Name: "from", Type: StringType},
			{Name: "to", Type: StringType},
		},
	},
	"asciiUpper": {
		Comment:    []string{"Returns a copy of the string in which all ASCII letters are capitalized.\n\nExamples:\n\n  Input: `std.asciiUpper('100 Cats!')`\n  Output: `\"100 CATS!\"`"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"asciiLower": {
		Comment:    []string{"Returns a copy of the string in which all ASCII letters are lower cased.\n\nExamples:\n\n  Input: `std.asciiLower('100 Cats!')`\n  Output: `\"100 cats!\"`"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"stringChars": {
		Comment:    []string{"Split the string `str` into an array of strings, each containing a single\ncodepoint.\n\nExamples:\n\n  Input: `std.stringChars(\"foo\")`\n  Output: `[\"f\",\"o\",\"o\"]`"},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"format": {
		Comment:    []string{"Format the string `str` using the values in `vals`. The values can be\nan array, an object, or in other cases are treated as if they were provided in a singleton\narray. The string formatting follows the [same rules](https://docs.python.org/2/library/stdtypes.html#string-formatting) as\nPython. The `%` operator can be used as a shorthand for this function.\n\nExamples:\n\n  Input: `std.format(\"Hello %03d\", 12)`\n  Output: `\"Hello 012\"`\n\n  Input: `\"Hello %03d\" % 12`\n  Output: `\"Hello 012\"`\n\n  Input: `\"Hello %s, age %d\" % [\"Foo\", 25]`\n  Output: `\"Hello Foo, age 25\"`\n\n  Input: `\"Hello %(name)s, age %(age)d\" % {age: 25, name: \"Foo\"}`\n  Output: `\"Hello Foo, age 25\"`"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "str", Type: StringType},
			{Name: "vals", Type: AnyType},
		},
	},
	"escapeStringBash": {
		Comment:    []string{"Wrap `str` in single quotes, and escape any single quotes within `str`\nby changing them to a sequence `'\"'\"'`. This allows injection of arbitrary strings\nas arguments of commands in bash scripts."},
		ReturnType: StringType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"escapeStringDollars": {
		Comment:    []string{"Convert $ to $$ in `str`. This allows injection of arbitrary strings into\nsystems that use $ for string interpolation (like Terraform)."},
		ReturnType: StringType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"escapeStringJson": {
		Comment:    []string{"Convert `str` to allow it to be embedded in a JSON representation, within a\nstring. This adds quotes, escapes backslashes, and escapes unprintable characters.\n\nExamples:\n\n  Input: `local description = \"Multiline\\nc:\\\\path\";\n\"{name: %s}\" % std.escapeStringJson(description)\n`\n  Output: `\"{name: \\\"Multiline\\\\nc:\\\\\\\\path\\\"}\"`"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"escapeStringPython": {
		Comment:    []string{"Convert `str` to allow it to be embedded in Python. This is an alias for\n`std.escapeStringJson`."},
		ReturnType: StringType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"parseInt": {
		Comment:    []string{"Parses a signed decimal integer from the input string.\n\nExamples:\n\n  Input: `std.parseInt(\"123\")`\n  Output: `123`\n\n  Input: `std.parseInt(\"-123\")`\n  Output: `-123`"},
		ReturnType: NumberType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"parseOctal": {
		Comment:    []string{"Parses an unsigned octal integer from the input string. Initial zeroes are tolerated.\n\nExamples:\n\n  Input: `std.parseOctal(\"755\")`\n  Output: `493`"},
		ReturnType: NumberType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"parseHex": {
		Comment:    []string{"Parses an unsigned hexadecimal integer, from the input string. Case insensitive.\n\nExamples:\n\n  Input: `std.parseHex(\"ff\")`\n  Output: `255`"},
		ReturnType: NumberType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"parseJson": {
		Comment:    []string{"Parses a JSON string.\n\nExamples:\n\n  Input: `std.parseJson('{\"foo\": \"bar\"}')`\n  Output: `{\"foo\":\"bar\"}`"},
		ReturnType: AnyType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"parseYaml": {
		Comment:    []string{"Parses a YAML string. This is provided as a \"best-effort\" mechanism and should not be relied on to provide\na fully standards compliant YAML parser. YAML is a superset of JSON, consequently \"downcasting\" or\nmanifestation of YAML into JSON or Jsonnet values will only succeed when using the subset of YAML that is\ncompatible with JSON. The parser does not support YAML documents with scalar values at the root. The\nroot node of a YAML document must start with either a YAML sequence or map to be successfully parsed.\n\nExamples:\n\n  Input: `std.parseYaml('foo: bar')`\n  Output: `{\"foo\":\"bar\"}`"},
		ReturnType: AnyType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"encodeUTF8": {
		Comment:    []string{"Encode a string using [UTF8](https://en.wikipedia.org/wiki/UTF-8). Returns an array of numbers\nrepresenting bytes."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"decodeUTF8": {
		Comment:    []string{"Decode an array of numbers representing bytes using [UTF8](https://en.wikipedia.org/wiki/UTF-8).\nReturns a string."},
		ReturnType: StringType,
		Params: []Param{
			{Name: "arr", Type: ArrayType},
		},
	},
	"manifestIni": {
		Comment:    []string{"Convert the given structure to a string in [INI format](https://en.wikipedia.org/wiki/INI_file). This\nallows using Jsonnet's\nobject model to build a configuration to be consumed by an application expecting an INI\nfile. The data is in the form of a set of sections, each containing a key/value mapping.\nThese examples should make it clear:\n\n```\n{\n    main: { a: \"1\", b: \"2\" },\n    sections: {\n        s1: {x: \"11\", y: \"22\", z: \"33\"},\n        s2: {p: \"yes\", q: \"\"},\n        empty: {},\n    }\n}\n```\n\nYields a string containing this INI file:\n\n```\na = 1\nb = 2\n[empty]\n[s1]\nx = 11\ny = 22\nz = 33\n[s2]\np = yes\nq =\n```"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "ini", Type: AnyType},
		},
	},
	"manifestPython": {
		Comment:    []string{"Convert the given value to a JSON-like form that is compatible with Python. The chief\ndifferences are True / False / None instead of true / false / null.\n\n```\n{\n    b: [\"foo\", \"bar\"],\n    c: true,\n    d: null,\n    e: { f1: false, f2: 42 },\n}\n```\n\nYields a string containing Python code like:\n\n```\n{\n    \"b\": [\"foo\", \"bar\"],\n    \"c\": True,\n    \"d\": None,\n    \"e\": {\"f1\": False, \"f2\": 42}\n}\n```"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "v", Type: AnyType},
		},
	},
	"manifestPythonVars": {
		Comment:    []string{"Convert the given object to a JSON-like form that is compatible with Python. The key\ndifference to `std.manifestPython` is that the top level is represented as a list\nof Python global variables.\n\n```\n{\n    b: [\"foo\", \"bar\"],\n    c: true,\n    d: null,\n    e: { f1: false, f2: 42 },\n}\n```\n\nYields a string containing this Python code:\n\n```\nb = [\"foo\", \"bar\"]\nc = True\nd = None\ne = {\"f1\": False, \"f2\": 42}\n```"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "conf", Type: AnyType},
		},
	},
	"manifestJsonEx": {
		Comment:    []string{"Convert the given object to a JSON form. `indent` is a string containing\none or more whitespaces that are used for indentation. `newline` is\nby default `\\n` and is inserted where a newline would normally be used\nto break long lines. `key_val_sep` is used to separate the key and value\nof an object field:\n\nExamples:\n\n  Input: `std.manifestJsonEx(\n{\n    x: [1, 2, 3, true, false, null,\n        \"string\\nstring\"],\n    y: { a: 1, b: 2, c: [1, 2] },\n}, \"    \")\n`\n  Output: `\"{\\n    \\\"x\\\": [\\n        1,\\n        2,\\n        3,\\n        true,\\n        false,\\n        null,\\n        \\\"string\\\\nstring\\\"\\n    ],\\n    \\\"y\\\": {\\n        \\\"a\\\": 1,\\n        \\\"b\\\": 2,\\n        \\\"c\\\": [\\n            1,\\n            2\\n        ]\\n    }\\n}\"`\n\n  Input: `std.manifestJsonEx(\n{\n  x: [1, 2, \"string\\nstring\"],\n  y: { a: 1, b: [1, 2] },\n}, \"\", \" \", \" : \")\n`\n  Output: `\"{ \\\"x\\\" : [ 1, 2, \\\"string\\\\nstring\\\" ], \\\"y\\\" : { \\\"a\\\" : 1, \\\"b\\\" : [ 1, 2 ] } }\"`"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "value", Type: AnyType},
			{Name: "indent", Type: StringType},
			{Name: "newline", Type: StringType, Default: &ast.LiteralString{Value: "\n"}},
			{Name: "key_val_sep", Type: StringType, Default: &ast.LiteralString{Value: ":"}},
		},
	},
	"manifestJsonMinified": {
		Comment:    []string{"Convert the given object to a minified JSON form. Under the covers,\nit calls `std.manifestJsonEx:')`:\n\nExamples:\n\n  Input: `std.manifestJsonMinified(\n{\n    x: [1, 2, 3, true, false, null,\n        \"string\\nstring\"],\n    y: { a: 1, b: 2, c: [1, 2] },\n})\n`\n  Output: `\"{\\\"x\\\":[1,2,3,true,false,null,\\\"string\\\\nstring\\\"],\\\"y\\\":{\\\"a\\\":1,\\\"b\\\":2,\\\"c\\\":[1,2]}}\"`"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "value", Type: AnyType},
		},
	},
	"manifestYamlDoc": {
		Comment:    []string{"Convert the given value to a YAML form. Note that `std.manifestJson` could also\nbe used for this purpose, because any JSON is also valid YAML. But this function will\nproduce more canonical-looking YAML.\n\n```\nstd.manifestYamlDoc(\n  {\n      x: [1, 2, 3, true, false, null,\n          \"string\\nstring\\n\"],\n      y: { a: 1, b: 2, c: [1, 2] },\n  },\n  indent_array_in_object=false)\n```\n\nYields a string containing this YAML:\n\n```\n\"x\":\n  - 1\n  - 2\n  - 3\n  - true\n  - false\n  - null\n  - |\n      string\n      string\n\"y\":\n  \"a\": 1\n  \"b\": 2\n  \"c\":\n      - 1\n      - 2\n```\n\nThe `indent_array_in_object` param adds additional indentation which some people\nmay find easier to read.\n\nThe `quote_keys` parameter controls whether YAML identifiers are always quoted\nor only when necessary."},
		ReturnType: StringType,
		Params: []Param{
			{Name: "value", Type: AnyType},
			{Name: "indent_array_in_object", Type: AnyType, Default: &ast.LiteralBoolean{Value: false}},
			{Name: "quote_keys", Type: AnyType, Default: &ast.LiteralBoolean{Value: true}},
		},
	},
	"manifestYamlStream": {
		Comment:    []string{"Given an array of values, emit a YAML \"stream\", which is a sequence of documents separated\nby `---` and ending with `...`.\n\n```\nstd.manifestYamlStream(\n  ['a', 1, []],\n  indent_array_in_object=false,\n  c_document_end=true)\n```\n\nYields this string:\n\n```\n---\n\"a\"\n---\n1\n---\n[]\n...\n```\n\nThe `indent_array_in_object` and `quote_keys` params are the\nsame as in `manifestYamlDoc`.\n\nThe `c_document_end` param adds the optional terminating `...`."},
		ReturnType: StringType,
		Params: []Param{
			{Name: "value", Type: AnyType},
			{Name: "indent_array_in_object", Type: AnyType, Default: &ast.LiteralBoolean{Value: false}},
			{Name: "c_document_end", Type: AnyType, Default: &ast.LiteralBoolean{Value: false}},
			{Name: "quote_keys", Type: AnyType, Default: &ast.LiteralBoolean{Value: true}},
		},
	},
	"manifestXmlJsonml": {
		Comment:    []string{"Convert the given [JsonML](http://www.jsonml.org/)-encoded value to a string\ncontaining the XML.\n\n```\nstd.manifestXmlJsonml([\n    'svg', { height: 100, width: 100 },\n    [\n        'circle', {\n        cx: 50, cy: 50, r: 40,\n        stroke: 'black', 'stroke-width': 3,\n        fill: 'red',\n        }\n    ],\n])\n```\n\nYields a string containing this XML (all on one line):\n\n```\n<svg height=\"100\" width=\"100\">\n    <circle cx=\"50\" cy=\"50\" fill=\"red\" r=\"40\"\n    stroke=\"black\" stroke-width=\"3\"></circle>;\n</svg>;\n```\n\nWhich represents the following image:\n\n Sorry, your browser does not support inline SVG.\n\nJsonML is designed to preserve \"mixed-mode content\" (i.e., textual data outside of or next\nto elements). This includes the whitespace needed to avoid having all the XML on one line,\nwhich is meaningful in XML. In order to have whitespace in the XML output, it must be\npresent in the JsonML input:\n\n```\nstd.manifestXmlJsonml([\n    'svg',\n    { height: 100, width: 100 },\n    '\\n  ',\n    [\n        'circle',\n        {\n        cx: 50, cy: 50, r: 40, stroke: 'black',\n        'stroke-width': 3, fill: 'red',\n        }\n    ],\n    '\\n',\n])\n```"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "value", Type: AnyType},
		},
	},
	"manifestTomlEx": {
		Comment:    []string{"Convert the given object to a TOML form. `indent` is a string containing\none or more whitespaces that are used for indentation:\n\nExamples:\n\n  Input: `std.manifestTomlEx({\n  key1: \"value\",\n  key2: 1,\n  section: {\n    a: 1,\n    b: \"str\",\n    c: false,\n    d: [1, \"s\", [2, 3]],\n    subsection: {\n      k: \"v\",\n    },\n  },\n  sectionArray: [\n    { k: \"v1\", v: 123 },\n    { k: \"v2\", c: \"value2\" },\n  ],\n}, \"  \")\n`\n  Output: `\"key1 = \\\"value\\\"\\nkey2 = 1\\n\\n[section]\\n  a = 1\\n  b = \\\"str\\\"\\n  c = false\\n  d = [\\n    1,\\n    \\\"s\\\",\\n    [ 2, 3 ]\\n  ]\\n\\n  [section.subsection]\\n    k = \\\"v\\\"\\n\\n[[sectionArray]]\\n  k = \\\"v1\\\"\\n  v = 123\\n\\n[[sectionArray]]\\n  c = \\\"value2\\\"\\n  k = \\\"v2\\\"\"`"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "toml", Type: AnyType},
			{Name: "indent", Type: StringType},
		},
	},
	"makeArray": {
		Comment:    []string{"Create a new array of `sz` elements by calling `func(i)` to initialize\neach element. Func is expected to be a function that takes a single parameter, the index of\nthe element it should initialize.\n\nExamples:\n\n  Input: `std.makeArray(3,function(x) x * x)`\n  Output: `[0,1,4]`"},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "sz", Type: NumberType},
			{Name: "func", Type: FunctionType},
		},
	},
	"member": {
		Comment:    []string{"Returns whether `x` occurs in `arr`.\nArgument `arr` may be an array or a string."},
		ReturnType: BooleanType,
		Params: []Param{
			{Name: "arr", Type: ArrayType},
			{Name: "x", Type: AnyType},
		},
	},
	"count": {
		Comment:    []string{"Return the number of times that `x` occurs in `arr`."},
		ReturnType: NumberType,
		Params: []Param{
			{Name: "arr", Type: ArrayType},
			{Name: "x", Type: AnyType},
		},
	},
	"find": {
		Comment:    []string{"Returns an array that contains the indexes of all occurrences of `value` in\n`arr`."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "value", Type: AnyType},
			{Name: "arr", Type: ArrayType},
		},
	},
	"map": {
		Comment:    []string{"Apply the given function to every element of the array to form a new array."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "func", Type: FunctionType},
			{Name: "arr", Type: ArrayType},
		},
	},
	"mapWithIndex": {
		Comment:    []string{"Similar to [map](#map) above, but it also passes to the function the element's\nindex in the array. The function `func` is expected to take the index as the\nfirst parameter and the element as the second."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "func", Type: FunctionType},
			{Name: "arr", Type: ArrayType},
		},
	},
	"filterMap": {
		Comment:    []string{"It first filters, then maps the given array, using the two functions provided."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "filter_func", Type: FunctionType},
			{Name: "map_func", Type: FunctionType},
			{Name: "arr", Type: ArrayType},
		},
	},
	"flatMap": {
		Comment:    []string{"Apply the given function to every element of `arr` to form a new array then flatten the result.\nThe argument `arr` must be an array or a string. If `arr` is an array, function `func` must return an array.\nIf `arr` is a string, function `func` must return an string.\n\nThe `std.flatMap` function can be thought of as a generalized `std.map`,\nwith each element mapped to 0, 1 or more elements.\n\nExamples:\n\n  Input: `std.flatMap(function(x) [x, x], [1, 2, 3])`\n  Output: `[1,1,2,2,3,3]`\n\n  Input: `std.flatMap(function(x) if x == 2 then [] else [x], [1, 2, 3])`\n  Output: `[1,3]`\n\n  Input: `std.flatMap(function(x) if x == 2 then [] else [x * 3, x * 2], [1, 2, 3])`\n  Output: `[3,2,9,6]`\n\n  Input: `std.flatMap(function(x) x+x, \"foo\")`\n  Output: `\"ffoooo\"`"},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "func", Type: FunctionType},
			{Name: "arr", Type: ArrayType},
		},
	},
	"filter": {
		Comment:    []string{"Return a new array containing all the elements of `arr` for which the\n`func` function returns true."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "func", Type: FunctionType},
			{Name: "arr", Type: ArrayType},
		},
	},
	"foldl": {
		Comment:    []string{"Classic foldl function. Calls the function on the result of the previous function call and\neach array element, or `init` in the case of the initial element. Traverses the\narray from left to right."},
		ReturnType: AnyType,
		Params: []Param{
			{Name: "func", Type: FunctionType, Comment: []string{"function(agg, x) -> agg"}},
			{Name: "arr", Type: ArrayType},
			{Name: "init", Type: AnyType},
		},
	},
	"foldr": {
		Comment:    []string{"Classic foldr function. Calls the function on the result of the previous function call and\neach array element, or `init` in the case of the initial element. Traverses the\narray from right to left."},
		ReturnType: AnyType,
		Params: []Param{
			{Name: "func", Type: FunctionType},
			{Name: "arr", Type: ArrayType},
			{Name: "init", Type: AnyType},
		},
	},
	"range": {
		Comment:    []string{"Return an array of ascending numbers between the two limits, inclusively."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "from", Type: NumberType},
			{Name: "to", Type: NumberType},
		},
	},
	"repeat": {
		Comment:    []string{"Repeats an array or a string `what` a number of times specified by an integer `count`.\n\nExamples:\n\n  Input: `std.repeat([1, 2, 3], 3)`\n  Output: `[1,2,3,1,2,3,1,2,3]`\n\n  Input: `std.repeat(\"blah\", 2)`\n  Output: `\"blahblah\"`"},
		ReturnType: AnyType,
		Params: []Param{
			{Name: "what", Type: AnyType},
			{Name: "count", Type: NumberType},
		},
	},
	"slice": {
		Comment:    []string{"Selects the elements of an array or a string from `index` to `end` with `step` and returns an array or a string respectively.\n\nNote that it's recommended to use dedicated slicing syntax both for arrays and strings (e.g. `arr[0:4:1]` instead of `std.slice(arr, 0, 4, 1)`).\n\nExamples:\n\n  Input: `std.slice([1, 2, 3, 4, 5, 6], 0, 4, 1)`\n  Output: `[1,2,3,4]`\n\n  Input: `std.slice([1, 2, 3, 4, 5, 6], 1, 6, 2)`\n  Output: `[2,4,6]`\n\n  Input: `std.slice(\"jsonnet\", 0, 4, 1)`\n  Output: `\"json\"`"},
		ReturnType: AnyType,
		Params: []Param{
			{Name: "indexable", Type: AnyType},
			{Name: "index", Type: NumberType},
			{Name: "end", Type: NumberType},
			{Name: "step", Type: NumberType},
		},
	},
	"join": {
		Comment:    []string{"If `sep` is a string, then `arr` must be an array of strings, in which\ncase they are concatenated with `sep` used as a delimiter. If `sep`\nis an array, then `arr` must be an array of arrays, in which case the arrays are\nconcatenated in the same way, to produce a single array.\n\nExamples:\n\n  Input: `std.join(\".\", [\"www\", \"google\", \"com\"])`\n  Output: `\"www.google.com\"`\n\n  Input: `std.join([9, 9], [[1], [2, 3]])`\n  Output: `[1,9,9,2,3]`"},
		ReturnType: StringType,
		Params: []Param{
			{Name: "sep", Type: StringType},
			{Name: "arr", Type: ArrayType},
		},
	},
	"lines": {
		Comment:    []string{"Concatenate an array of strings into a text file with newline characters after each string.\nThis is suitable for constructing bash scripts and the like."},
		ReturnType: StringType,
		Params: []Param{
			{Name: "arr", Type: ArrayType},
		},
	},
	"flattenArrays": {
		Comment:    []string{"Concatenate an array of arrays into a single array.\n\nExamples:\n\n  Input: `std.flattenArrays([[1, 2], [3, 4], [[5, 6], [7, 8]]])`\n  Output: `[1,2,3,4,[5,6],[7,8]]`"},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "arr", Type: ArrayType},
		},
	},
	"reverse": {
		Comment:    []string{"Reverses an array."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "arrs", Type: ArrayType},
		},
	},
	"sort": {
		Comment:    []string{"Sorts the array using the <= operator.\n\nOptional argument `keyF` is a single argument function used to extract comparison key from each array element.\nDefault value is identity function `keyF=function(x) x`."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "arr", Type: ArrayType},
			{Name: "keyF", Type: FunctionType, Default: &ast.LiteralNull{}},
		},
	},
	"uniq": {
		Comment:    []string{"Removes successive duplicates. When given a sorted array, removes all duplicates.\n\nOptional argument `keyF` is a single argument function used to extract comparison key from each array element.\nDefault value is identity function `keyF=function(x) x`."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "arr", Type: ArrayType},
			{Name: "keyF", Type: FunctionType, Default: &ast.LiteralNull{}},
		},
	},
	"all": {
		Comment:    []string{"Return true if all elements of `arr` is true, false otherwise. `all([])` evaluates to true.\n\nIt's an error if 1) `arr` is not an array, or 2) `arr` contains non-boolean values."},
		ReturnType: BooleanType,
		Params: []Param{
			{Name: "arr", Type: ArrayType},
		},
	},
	"any": {
		Comment:    []string{"Return true if any element of `arr` is true, false otherwise. `any([])` evaluates to false.\n\nIt's an error if 1) `arr` is not an array, or 2) `arr` contains non-boolean values."},
		ReturnType: BooleanType,
		Params: []Param{
			{Name: "arr", Type: ArrayType},
		},
	},
	"set": {
		Comment:    []string{"Shortcut for std.uniq(std.sort(arr))."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "arr", Type: ArrayType},
			{Name: "keyF", Type: FunctionType, Default: &ast.LiteralNull{}},
		},
	},
	"setInter": {
		Comment:    []string{"Set intersection operation (values in both a and b)."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "a", Type: ArrayType},
			{Name: "b", Type: ArrayType},
			{Name: "keyF", Type: FunctionType, Default: &ast.LiteralNull{}},
		},
	},
	"setUnion": {
		Comment:    []string{"Set union operation (values in any of `a` or `b`). Note that + on sets will simply\nconcatenate\nthe arrays, possibly forming an array that is not a set (due to not being ordered without\nduplicates).\n\nExamples:\n\n  Input: `std.setUnion([1, 2], [2, 3])`\n  Output: `[1,2,3]`\n\n  Input: `std.setUnion([{n:\"A\", v:1}, {n:\"B\"}], [{n:\"A\", v: 9999}, {n:\"C\"}], keyF=function(x) x.n)`\n  Output: `[{\"n\":\"A\",\"v\":1},{\"n\":\"B\"},{\"n\":\"C\"}]`"},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "a", Type: ArrayType},
			{Name: "b", Type: ArrayType},
			{Name: "keyF", Type: FunctionType, Default: &ast.LiteralNull{}},
		},
	},
	"setDiff": {
		Comment:    []string{"Set difference operation (values in a but not b)."},
		ReturnType: ArrayType,
		Params: []Param{
			{Name: "a", Type: ArrayType},
			{Name: "b", Type: ArrayType},
			{Name: "keyF", Type: FunctionType, Default: &ast.LiteralNull{}},
		},
	},
	"setMember": {
		Comment:    []string{"Returns `true` if x is a member of array, otherwise `false`."},
		ReturnType: BooleanType,
		Params: []Param{
			{Name: "x", Type: AnyType},
			{Name: "arr", Type: ArrayType},
			{Name: "keyF", Type: FunctionType, Default: &ast.LiteralNull{}},
		},
	},
	"base64": {
		Comment:    []string{"Encodes the given value into a base64 string. The encoding sequence is `A-Za-z0-9+/` with\n`=`\nto pad the output to a multiple of 4 characters. The value can be a string or an array of\nnumbers, but the codepoints / numbers must be in the 0 to 255 range. The resulting string\nhas no line breaks."},
		ReturnType: StringType,
		Params: []Param{
			{Name: "input", Type: AnyType},
		},
	},
	"base64DecodeBytes": {
		Comment:    []string{"Decodes the given base64 string into an array of bytes (number values). Currently assumes\nthe input string has no linebreaks and is padded to a multiple of 4 (with the = character).\nIn other words, it consumes the output of std.base64()."},
		ReturnType: AnyType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"base64Decode": {
		Comment:    []string{"_Deprecated, use `std.base64DecodeBytes` and decode the string explicitly (e.g. with `std.decodeUTF8`) instead._\n\nBehaves like std.base64DecodeBytes() except returns a naively encoded string instead of an array of bytes."},
		ReturnType: AnyType,
		Params: []Param{
			{Name: "str", Type: StringType},
		},
	},
	"md5": {
		Comment:    []string{"Encodes the given value into an MD5 string."},
		ReturnType: StringType,
		Params: []Param{
			{Name: "s", Type: AnyType},
		},
	},
	"mergePatch": {
		Comment:    []string{"Applies `patch` to `target`\naccording to [RFC7396](https://tools.ietf.org/html/rfc7396)"},
		ReturnType: AnyType,
		Params: []Param{
			{Name: "target", Type: AnyType},
			{Name: "patch", Type: AnyType},
		},
	},
	"trace": {
		Comment:    []string{"Outputs the given string `str` to stderr and\nreturns `rest` as the result.\n\nExample:\n\n```\nlocal conditionalReturn(cond, in1, in2) =\n  if (cond) then\n      std.trace('cond is true returning '\n              + std.toString(in1), in1)\n  else\n      std.trace('cond is false returning '\n              + std.toString(in2), in2);\n\n{\n    a: conditionalReturn(true, { b: true }, { c: false }),\n}\n```\n\nPrints:\n\n```\nTRACE: test.jsonnet:3 cond is true returning {\"b\": true}\n{\n    \"a\": {\n        \"b\": true\n    }\n}\n```"},
		ReturnType: AnyType,
		Params: []Param{
			{Name: "str", Type: StringType},
			{Name: "rest", Type: AnyType},
		},
	},

	// Mathematical Utilities
	"abs":      {ReturnType: NumberType, Params: []Param{{Name: "n", Type: NumberType}}},
	"sign":     {ReturnType: NumberType, Params: []Param{{Name: "n", Type: NumberType}}},
	"max":      {ReturnType: NumberType, Params: []Param{{Name: "a", Type: NumberType}, {Name: "b", Type: NumberType}}},
	"min":      {ReturnType: NumberType, Params: []Param{{Name: "a", Type: NumberType}, {Name: "b", Type: NumberType}}},
	"mod":      {ReturnType: NumberType, Params: []Param{{Name: "a", Type: NumberType}, {Name: "b", Type: NumberType}}},
	"pow":      {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}, {Name: "n", Type: NumberType}}},
	"exp":      {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}}},
	"log":      {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}}},
	"exponent": {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}}},
	"mantissa": {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}}},
	"floor":    {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}}},
	"ceil":     {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}}},
	"sqrt":     {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}}},
	"sin":      {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}}},
	"cos":      {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}}},
	"tan":      {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}}},
	"asin":     {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}}},
	"acos":     {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}}},
	"atan":     {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}}},
	"round":    {ReturnType: NumberType, Params: []Param{{Name: "x", Type: NumberType}}},
}

var StdLibValue = func(fns map[string]*Function) *Value {
	res := &Value{
		Type:    ObjectType,
		Comment: []string{"The built-in jsonnet standard library"},
		Object: &Object{
			AllFieldsKnown: true,
			FieldMap:       map[string]*Field{},
		},
	}

	keys := []string{}
	for name := range fns {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fn := fns[k]
		res.Object.Fields = append(res.Object.Fields, Field{
			Name:    k,
			Type:    FunctionType,
			Comment: []string{k + fn.String()},
		})
		res.Object.FieldMap[k] = &res.Object.Fields[len(res.Object.Fields)-1]
	}
	return res
}(StdLibFunctions)
