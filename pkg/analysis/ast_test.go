package analysis

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type printASTCase struct {
	Name   string
	Source string
	Expect string
}

var printASTCases = []printASTCase{
	{
		Name: "ObjectComprehension",
	},
}

func TestPrintAST(t *testing.T) {
	for _, testcase := range printASTCases {
		tc := testcase
		t.Run(tc.Name, func(t *testing.T) {
			source := tc.Source
			if source == "" {
				fname := fmt.Sprintf("testdata/PrintAST/%s.jsonnet", tc.Name)
				data, err := testdataFS.ReadFile(fname)
				require.NoError(t, err, "no Source defined in test case, or could not find testdata at '%s'")
				source = string(data)
			}
			expect := tc.Expect
			if expect == "" {
				fname := fmt.Sprintf("testdata/PrintAST/%s.log", tc.Name)
				data, err := testdataFS.ReadFile(fname)
				require.NoError(t, err, "no Expect defined in test case, or could not find testdata at '%s'")
				expect = string(data)
			}
			resolver, _ := newAnonMockResolver(t, source)
			buf := bytes.NewBuffer(nil)
			PrintAst(resolver.root, buf)
			got := buf.String()
			// use printf so we can easily copy the output into the test file
			if expect != got {
				fmt.Printf("got AST data:\n%s\n", got)
			}
			assert.Equal(t, expect, got)
		})
	}
}
