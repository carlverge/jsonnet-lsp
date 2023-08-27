package annotation

import (
	"bytes"
	"testing"
)

func TestLexer(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []struct {
			tok Token
			lit string
		}
	}{
		{
			name:  "literal number type",
			input: "number",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: IDENT, lit: "number"},
				{tok: EOF, lit: ""},
			},
		},
		{
			name:  "literal boolean type",
			input: "boolean",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: IDENT, lit: "boolean"},
				{tok: EOF, lit: ""},
			},
		},
		{
			name:  "union type of either a literal string or literal null",
			input: "string | null",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: IDENT, lit: "string"},
				{tok: SPACE, lit: " "},
				{tok: UNION, lit: "|"},
				{tok: SPACE, lit: " "},
				{tok: IDENT, lit: "null"},
				{tok: EOF, lit: ""},
			},
		},
		{
			name:  "array with string elements",
			input: "array[string]",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: ARRAY, lit: "array"},
				{tok: BRACKET_OPEN, lit: "["},
				{tok: IDENT, lit: "string"},
				{tok: BRACKET_CLOSE, lit: "]"},
				{tok: EOF, lit: ""},
			},
		},
		{
			name:  "object, where all the object values are of type number",
			input: "object[number]",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: OBJECT, lit: "object"},
				{tok: BRACKET_OPEN, lit: "["},
				{tok: IDENT, lit: "number"},
				{tok: BRACKET_CLOSE, lit: "]"},
				{tok: EOF, lit: ""},
			},
		},
		{
			name:  "a function, with any parameters or return types",
			input: "function",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: FUNCTION, lit: "function"},
				{tok: EOF, lit: ""},
			},
		},
		{
			name:  "simple identifier",
			input: "array",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: ARRAY, lit: "array"},
				{tok: EOF, lit: ""},
			},
		},
		{
			name:  "simple function",
			input: "function(a: string, b: null, c: boolean) -> number",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: FUNCTION, lit: "function"},
				{tok: PAREN_OPEN, lit: "("},
				{tok: IDENT, lit: "a"},
				{tok: COLON, lit: ":"},
				{tok: SPACE, lit: " "},
				{tok: IDENT, lit: "string"},
				{tok: COMMA, lit: ","},
				{tok: SPACE, lit: " "},
				{tok: IDENT, lit: "b"},
				{tok: COLON, lit: ":"},
				{tok: SPACE, lit: " "},
				{tok: IDENT, lit: "null"},
				{tok: COMMA, lit: ","},
				{tok: SPACE, lit: " "},
				{tok: IDENT, lit: "c"},
				{tok: COLON, lit: ":"},
				{tok: SPACE, lit: " "},
				{tok: IDENT, lit: "boolean"},
				{tok: PAREN_CLOSE, lit: ")"},
				{tok: SPACE, lit: " "},
				{tok: ARROW, lit: "->"},
				{tok: SPACE, lit: " "},
				{tok: IDENT, lit: "number"},
				{tok: EOF, lit: ""},
			},
		},
		{
			name:  "function with known parameters",
			input: "function(a, b)",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: FUNCTION, lit: "function"},
				{tok: PAREN_OPEN, lit: "("},
				{tok: IDENT, lit: "a"},
				{tok: COMMA, lit: ","},
				{tok: SPACE, lit: " "},
				{tok: IDENT, lit: "b"},
				{tok: PAREN_CLOSE, lit: ")"},
				{tok: EOF, lit: ""},
			},
		},
		{
			name:  "sum function",
			input: "function(nums: array[number]) -> number",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: FUNCTION, lit: "function"},
				{tok: PAREN_OPEN, lit: "("},
				{tok: IDENT, lit: "nums"},
				{tok: COLON, lit: ":"},
				{tok: SPACE, lit: " "},
				{tok: ARRAY, lit: "array"},
				{tok: BRACKET_OPEN, lit: "["},
				{tok: IDENT, lit: "number"},
				{tok: BRACKET_CLOSE, lit: "]"},
				{tok: PAREN_CLOSE, lit: ")"},
				{tok: SPACE, lit: " "},
				{tok: ARROW, lit: "->"},
				{tok: SPACE, lit: " "},
				{tok: IDENT, lit: "number"},
				{tok: EOF, lit: ""},
			},
		},
		{
			name:  "single type parameter",
			input: "T",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: IDENT, lit: "T"},
				{tok: EOF, lit: ""},
			},
		},
		{
			name:  "map function with type parameters",
			input: "function(fn: function(elem: A) -> B, arr: array[A]) -> array[B]",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: FUNCTION, lit: "function"},
				{tok: PAREN_OPEN, lit: "("},
				{tok: IDENT, lit: "fn"},
				{tok: COLON, lit: ":"},
				{tok: SPACE, lit: " "},
				{tok: FUNCTION, lit: "function"},
				{tok: PAREN_OPEN, lit: "("},
				{tok: IDENT, lit: "elem"},
				{tok: COLON, lit: ":"},
				{tok: SPACE, lit: " "},
				{tok: IDENT, lit: "A"},
				{tok: PAREN_CLOSE, lit: ")"},
				{tok: SPACE, lit: " "},
				{tok: ARROW, lit: "->"},
				{tok: SPACE, lit: " "},
				{tok: IDENT, lit: "B"},
				{tok: COMMA, lit: ","},
				{tok: SPACE, lit: " "},
				{tok: IDENT, lit: "arr"},
				{tok: COLON, lit: ":"},
				{tok: SPACE, lit: " "},
				{tok: ARRAY, lit: "array"},
				{tok: BRACKET_OPEN, lit: "["},
				{tok: IDENT, lit: "A"},
				{tok: BRACKET_CLOSE, lit: "]"},
				{tok: PAREN_CLOSE, lit: ")"},
				{tok: SPACE, lit: " "},
				{tok: ARROW, lit: "->"},
				{tok: SPACE, lit: " "},
				{tok: ARRAY, lit: "array"},
				{tok: BRACKET_OPEN, lit: "["},
				{tok: IDENT, lit: "B"},
				{tok: BRACKET_CLOSE, lit: "]"},
				{tok: EOF, lit: ""},
			},
		},
		// edge cases
		{
			name:  "empty input",
			input: "",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: EOF, lit: ""},
			},
		},
		{
			name:  "whitespace only",
			input: "   \t\n   ",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: SPACE, lit: "   \t\n   "},
				{tok: EOF, lit: ""},
			},
		},
		{
			name:  "illegal character",
			input: "@",
			want: []struct {
				tok Token
				lit string
			}{
				{tok: ILLEGAL, lit: "@"},
				{tok: EOF, lit: ""},
			},
		},
		// {
		// 	name:  "identifier starting with number",
		// 	input: "123foo",
		// 	want: []struct {
		// 		tok Token
		// 		lit string
		// 	}{
		// 		{tok: ILLEGAL, lit: "123foo"},
		// 		{tok: EOF, lit: ""},
		// 	},
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newScanner(bytes.NewBufferString(tt.input))
			for i, want := range tt.want {
				tok, lit := s.Scan()
				if tok != want.tok {
					t.Errorf("token %d: got %v, want %v", i, tok, want.tok)
				}
				if lit != want.lit {
					t.Errorf("literal %d: got %v, want %v", i, lit, want.lit)
				}
			}
		})
	}
}
