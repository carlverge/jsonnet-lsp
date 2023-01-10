package analysis

import (
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type basicTypeCase struct {
	Name string
	Code string
	Type ValueType
}

func TestBasicType(t *testing.T) {
	cases := []basicTypeCase{
		{"Boolean", "false", BooleanType},
		{"null", "null", NullType},
		{"Number", "1234", NumberType},
		{"String", "\"asdf\"", StringType},
		{"Array Literal", "[1,2,3]", ArrayType},
		{"Array Comprehension", "[f for f in [1,2,3]]", ArrayType},
		{"Object Comprehension", "{[f]: f for f in [1,2,3]}", ObjectType},
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			node, err := jsonnet.SnippetToAST("anon", c.Code)
			require.NoError(t, err)
			typ, ok := simpleToValueType(node)
			require.True(t, ok)
			assert.Equal(t, c.Type, typ)
		})
	}
}
