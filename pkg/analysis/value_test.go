package analysis

import (
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type basicTypeCase struct {
	Name     string
	Code     string
	Type     ValueType
	Terminal bool
}

func TestBasicType(t *testing.T) {
	cases := []basicTypeCase{
		{"Boolean", "false", BooleanType, true},
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
