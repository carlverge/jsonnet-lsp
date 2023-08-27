package annotation

import (
	"bytes"
	"testing"
)

type parserTestCase struct {
	Name   string
	Source string
}

var parserTestCases = []parserTestCase{
	{
		Name:   "Literal number type",
		Source: "number",
	},
	{
		Name:   "Literal boolean type",
		Source: "boolean",
	},
	{
		Name:   "Union type of either a literal string or literal null",
		Source: "string | null",
	},
	{
		Name:   "Array without any specific element type",
		Source: "array",
	},
	{
		Name:   "Array with string elements",
		Source: "array[string]",
	},
	{
		Name:   "Type Parameter",
		Source: "T",
	},
	{
		Name:   "Normal Ident",
		Source: "someVar",
	},
	{
		Name:   "Dotted Ident",
		Source: "someVar.subVar",
	},
	{
		Name:   "Object without any specific element type",
		Source: "object",
	},
	{
		Name:   "Object where all the object values are of type number",
		Source: "object[number]",
	},
	{
		Name:   "Object with fields",
		Source: "{a: number, b: string | null, c: array[boolean]}",
	},
	{
		Name:   "Array of objects whose values are either number, string, or null",
		Source: "array[object[number | string | null]]",
	},
	{
		Name:   "Function with two known parameters",
		Source: "function(a, b)",
	},
	{
		Name:   "Sum function",
		Source: "function(nums: array[number]) -> number",
	},
	{
		Name:   "Map function with type parameters",
		Source: "function(fn: function(elem: A) -> B, arr: array[A]) -> array[B]",
	},
}

func TestLexerParserPrinter(t *testing.T) {

	for _, tt := range parserTestCases {
		t.Run(tt.Name, func(t *testing.T) {
			// s := newScanner()
			p := newParser(bytes.NewBufferString(tt.Source))
			node, err := p.Parse()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := node.String()
			if got != tt.Source {
				t.Errorf("unexpected string result:\n   got: %s\n  want: %s", got, tt.Source)
			}

		})
	}
}
