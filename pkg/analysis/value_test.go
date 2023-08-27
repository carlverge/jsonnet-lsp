package analysis

import (
	"embed"
	"fmt"
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata
var testdataFS embed.FS

type basicTypeCase struct {
	Name     string
	Code     string
	Type     ValueType
	Terminal bool
}

func TestBasicType(t *testing.T) {
	cases := []basicTypeCase{
		{"Boolean", "false", BooleanType, false},
		{"null", "null", NullType, true},
		{"Number", "1234", NumberType, false},
		{"String", "\"asdf\"", StringType, false},
		{"Array Literal", "[1,2,3]", ArrayType, false},
		{"Array Comprehension", "[f for f in [1,2,3]]", ArrayType, true},
		{"Object Comprehension", "{[f]: f for f in [1,2,3]}", ObjectType, true},
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			node, err := jsonnet.SnippetToAST("anon", c.Code)
			require.NoError(t, err)
			typ, ok := simpleToValueType(node)
			require.Equal(t, c.Terminal, ok)
			assert.Equal(t, c.Type, typ)
		})
	}
}

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

type valueCase struct {
	Name   string
	Code   string
	Expect valueResult
}

// These cases will read the corresponding file in testdata/NodeToValue/<name>.jsonnet
// The exported value will have NodeToValue called on it
var valuesCases = []valueCase{
	{
		Name: "NestedLocal",
		Expect: valueResult{
			Type:    NumberType,
			Range:   valueRange{3, 3, 3, 4},
			Comment: []string{"2"},
		},
	},
	{
		Name: "ObjectFieldResolution",
		Expect: valueResult{
			Type:    NumberType,
			Range:   valueRange{1, 26, 1, 30},
			Comment: []string{"1234"},
		},
	},
	{
		Name: "ComputedObjectField",
		Expect: valueResult{
			Type:    NumberType,
			Range:   valueRange{1, 26, 1, 28},
			Comment: []string{"24"},
		},
	},
	{
		Name: "FunctionReturnBasic",
		Expect: valueResult{
			Type:    BooleanType,
			Range:   valueRange{1, 14, 1, 19},
			Comment: []string{"false"},
		},
	},
	{
		Name: "FunctionBasic",
		Expect: valueResult{
			Type:  FunctionType,
			Range: valueRange{1, 7, 1, 29},
		},
	},
}

func TestNodeToValue(t *testing.T) {
	for _, tc := range valuesCases {
		t.Run(tc.Name, func(t *testing.T) {
			source, err := testdataFS.ReadFile(fmt.Sprintf("testdata/NodeToValue/%s.jsonnet", tc.Name))
			require.NoError(t, err)
			resolver, out := newAnonMockResolver(t, string(source))
			res := NodeToValue(out, resolver)
			require.NotNil(t, res, "expected resolved value but got nil")
			assert.Equal(t, tc.Expect, valueToTestResult(res))
		})
	}
}

type valueRange struct {
	BeginLine, BeginCol, EndLine, EndCol int
}

type valueResult struct {
	Type    ValueType
	Range   valueRange
	Comment []string
}

func valueToTestResult(v *Value) valueResult {
	return valueResult{
		Type: v.Type,
		Range: valueRange{
			BeginLine: v.Range.Begin.Line,
			BeginCol:  v.Range.Begin.Column,
			EndLine:   v.Range.End.Line,
			EndCol:    v.Range.End.Column,
		},
		Comment: v.Comment,
	}
}
