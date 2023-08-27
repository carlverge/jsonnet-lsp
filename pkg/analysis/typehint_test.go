package analysis

import (
	"embed"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata
var testdataFS embed.FS

type mockResolver struct {
	root ast.Node
	vars VarMap
}

func newAnonMockResolver(t *testing.T, source string) (*mockResolver, ast.Node) {
	root, err := jsonnet.SnippetToAST("anon", source)
	require.NoError(t, err)
	vars, out := UnwindLocals(root)
	require.NotNil(t, out)
	return &mockResolver{root: root, vars: vars}, out
}

func (r *mockResolver) NodeAt(loc ast.Location) (node ast.Node, stack []ast.Node) {
	stack = StackAtLoc(r.root, loc)
	if len(stack) == 0 {
		return nil, nil
	}
	node = stack[len(stack)-1]
	return node, stack
}

func (r *mockResolver) Vars(from ast.Node) VarMap {
	if from == nil || from.Loc() == nil {
		return VarMap{}
	}
	stk := StackAtNode(r.root, from)
	return StackVars(stk)
}

func (r *mockResolver) Import(from, path string) ast.Node {
	panic("cannot import from mockResolver")
}

type valueTypeCase struct {
	Name          string
	Source        string
	Expect        string
	TypeStr       string
	UseReturnHint bool
}

var valueTypeCases = []valueTypeCase{
	{
		Name:    "NullLiteral",
		Source:  `null`,
		Expect:  `{"valueType": "null"}`,
		TypeStr: `null`,
	},
	{
		Name:    "StringLiteral",
		Source:  `"asdf"`,
		Expect:  `{"valueType": "string"}`,
		TypeStr: `string`,
	},
	{
		Name:    "NumberLiteral",
		Source:  `1234`,
		Expect:  `{"valueType": "number"}`,
		TypeStr: `number`,
	},
	{
		Name:    "BooleanLiteral",
		Source:  `false`,
		Expect:  `{"valueType": "boolean"}`,
		TypeStr: `boolean`,
	},
	{
		Name:    "EmptyArrayLiteral",
		Source:  `[]`,
		Expect:  `{"valueType": "array"}`,
		TypeStr: `array`,
	},
	{
		Name:    "EmptyObjectLiteral",
		Source:  `{}`,
		Expect:  `{"valueType": "object", "object": {"allFieldsKnown": true}}`,
		TypeStr: `object`,
	},
	{
		Name:    "FunctionNoHints",
		Source:  "function(a, b, c=null) 123",
		TypeStr: `function(a, b, c)`,
	},
	{
		Name:    "FunctionBasicHints",
		Source:  "function(a/*:string*/, b/*:null*/, c/*:boolean*/) /*:number*/ 123",
		TypeStr: `function(a: string, b: null, c: boolean) -> number`,
	},
	{
		Name:    "FunctionBasicHintsDefault",
		Source:  "function(a/*:string*/=null, b/*:null*/=123) null",
		TypeStr: `function(a: string, b: null)`,
	},
	{
		Name: "FunctionTypeParams",
		Source: `
			function(fn/*:function(elem: A) -> B*/, arr/*:array[A]*/) /*:array[B]*/ null
		`,
		TypeStr: `function(fn: function(elem: A) -> B, arr: array[A]) -> array[B]`,
	},
	{
		Name:          "HintArrayElem",
		UseReturnHint: true,
		Source:        "function() /*:array[string]*/ null ",
		TypeStr:       "array[string]",
		Expect: `{
			"valueType": "array",
			"element": {
			 "valueType": "string"
			}
		}`,
	},
	{
		Name:          "HintArrayElemArray",
		UseReturnHint: true,
		Source:        "function() /*:array[array[string]]*/ null ",
		TypeStr:       "array[array[string]]",
		Expect: `{
			"valueType": "array",
			"element": {
			 "valueType": "array",
			 "element": {
			  "valueType": "string"
			 }
			}
		}`,
	},
	{
		Name:          "HintUnionBasic",
		UseReturnHint: true,
		Source:        "function() /*:string | null*/ null ",
		TypeStr:       "string | null",
		Expect: `{
			"valueType": "union",
			"union": [{"valueType": "string"}, {"valueType": "null"}]
		}`,
	},
	{
		Name:          "HintUnionElem",
		UseReturnHint: true,
		TypeStr:       "array[string | null]",
		Source:        "function() /*:array[string | null]*/ null ",
		Expect: `{"valueType": "array", "element": {
			"valueType": "union",
			"union": [{"valueType": "string"}, {"valueType": "null"}]
		}}`,
	},
	{
		Name:          "TypeDefFunction",
		UseReturnHint: true,
		TypeStr:       "function(a: number) -> boolean",
		Source: `
			function() /*:function(a: number) -> boolean*/ null
		`,
	},
	// {
	// 	Name:          "TypeDefObject",
	// 	TypeStr:       "array[object]",
	// 	UseReturnHint: true,
	// },
	{
		Name:    "ObjectTypeHintsBasic",
		TypeStr: "object",
	},
	{
		Name: "InferTypeParamReturn",
		Source: `
			local map(fn/*:function(elem: A) -> B*/, arr/*:array[A]*/) = /*:array[B]*/ [];
			map(function(x/*:number*/) /*:boolean*/ false, [1, 2, 3])
		`,
		Expect:  `{"valueType": "array"}`,
		TypeStr: `array`,
	},
}

func TestValueTypeCases(t *testing.T) {
	for _, testcase := range valueTypeCases {
		tc := testcase
		t.Run(tc.Name, func(t *testing.T) {
			source := tc.Source
			if source == "" {
				fname := fmt.Sprintf("testdata/typehint_value/%s_source.jsonnet", tc.Name)
				data, err := testdataFS.ReadFile(fname)
				require.NoError(t, err, "no Source defined in test case, or could not find testdata at '%s'")
				source = string(data)
			}
			expect := tc.Expect
			if expect == "" {
				fname := fmt.Sprintf("testdata/typehint_value/%s_expect.json", tc.Name)
				data, err := testdataFS.ReadFile(fname)
				require.NoError(t, err, "no Expect defined in test case, or could not find testdata at '%s'")
				expect = string(data)
			}
			resolver, out := newAnonMockResolver(t, source)
			res := NodeToValue(out, resolver)
			got := mustJSON(res.Type)
			typeStr := res.Type.String()
			// use this to get smaller answers back
			if tc.UseReturnHint {
				require.NotNil(t, res.Type.Function)
				got = mustJSON(res.Type.Function.ReturnHint)
				typeStr = res.Type.Function.ReturnHint.String()
			}
			assert.JSONEq(t, expect, got, "got JSON instead:\n%s", got)
			assert.Equal(t, typeStr, tc.TypeStr, "got type string instead: %s", typeStr)
		})
	}
}

func mustJSON(v interface{}) string {
	out, err := json.MarshalIndent(v, " ", "  ")
	if err != nil {
		panic(err)
	}
	return string(out)
}
