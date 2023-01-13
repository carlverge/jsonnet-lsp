package linter_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlverge/jsonnet-lsp/pkg/analysis"
	"github.com/carlverge/jsonnet-lsp/pkg/linter"
	"github.com/carlverge/jsonnet-lsp/pkg/testdata"
	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lsp.dev/protocol"
)

type linterCase struct {
	File   string
	Expect []string
}

var linterCases = []linterCase{
	{
		File: "unused_vars.jsonnet",
		Expect: []string{
			"[Warning|UnusedVar|2:7-2:17] unused local variable 'x'",
		},
	},
	{
		File: "functions.jsonnet",
		Expect: []string{
			"[Error|ArgumentCardinality|1:20-1:36] too few arguments in function call (1 arguments for 2 required parameters)",
			"[Error|ArgumentCardinality|2:21-2:45] too many arguments in function call (3 arguments for 2 parameters)",
			"[Warning|TypeMismatch|3:22-3:32] mismatched argument type for 'arr' expected 'array' got 'number'",
			"[Error|TypeMismatch|5:24-5:35] calling non-function type 'string'",
			"[Warning|ArgumentCardinality|7:29-7:50] duplicate named argument 'a'",
			"[Warning|TypeMismatch|9:26-9:43] mismatched argument type for 'a' expected 'string' got 'number'",
			"[Warning|TypeMismatch|9:26-9:43] mismatched argument type for 'b' expected 'number' got 'boolean'",
		},
	},
}

func fmtDiags(diags []protocol.Diagnostic) string {
	res := []string{}
	for _, d := range diags {
		res = append(res, fmt.Sprintf("%q,\n", linter.FmtDiag(d)))
	}
	return strings.Join(res, "")
}

func TestLinter(t *testing.T) {
	for _, c := range linterCases {
		t.Run(c.File, func(t *testing.T) {
			vm := jsonnet.MakeVM()
			vm.Importer(&FSImporter{FS: testdata.TestDataFS})
			root, _, err := vm.ImportAST(c.File, c.File)
			require.NoError(t, err, "must be able to import root AST")

			resolver := NewResolver(root, vm)
			diags := linter.LintAST(root, resolver)
			require.Equal(t, len(c.Expect), len(diags), "mismatch in expected length of diags, got:\n%s", fmtDiags(diags))
			for i, d := range diags {
				assert.Equal(t, c.Expect[i], linter.FmtDiag(d), "mismatch on diag %d", i)
			}
		})
	}
}

// FSImporter imports data from the filesystem.
type FSImporter struct {
	FS      fs.FS
	JPaths  []string
	fsCache map[string]*fsCacheEntry
}

type fsCacheEntry struct {
	contents jsonnet.Contents
	exists   bool
}

func (importer *FSImporter) tryPath(dir, importedPath string) (found bool, contents jsonnet.Contents, foundHere string, err error) {
	if importer.fsCache == nil {
		importer.fsCache = make(map[string]*fsCacheEntry)
	}
	var absPath string
	if filepath.IsAbs(importedPath) {
		absPath = importedPath
	} else {
		absPath = filepath.Join(dir, importedPath)
	}
	var entry *fsCacheEntry
	if cacheEntry, isCached := importer.fsCache[absPath]; isCached {
		entry = cacheEntry
	} else {
		contentBytes, err := fs.ReadFile(importer.FS, absPath)
		if err != nil {
			if os.IsNotExist(err) {
				entry = &fsCacheEntry{
					exists: false,
				}
			} else {
				return false, jsonnet.Contents{}, "", err
			}
		} else {
			entry = &fsCacheEntry{
				exists:   true,
				contents: jsonnet.MakeContentsRaw(contentBytes),
			}
		}
		importer.fsCache[absPath] = entry
	}
	return entry.exists, entry.contents, absPath, nil
}

// This is copied from go-jsonnet and modified to support fs.FS
func (importer *FSImporter) Import(importedFrom, importedPath string) (contents jsonnet.Contents, foundAt string, err error) {
	dir, _ := filepath.Split(importedFrom)
	found, content, foundHere, err := importer.tryPath(dir, importedPath)
	if err != nil {
		return jsonnet.Contents{}, "", err
	}

	for i := len(importer.JPaths) - 1; !found && i >= 0; i-- {
		found, content, foundHere, err = importer.tryPath(importer.JPaths[i], importedPath)
		if err != nil {
			return jsonnet.Contents{}, "", err
		}
	}

	if !found {
		return jsonnet.Contents{}, "", fmt.Errorf("couldn't open import %#v: no match locally or in the Jsonnet library paths", importedPath)
	}
	return content, foundHere, nil
}

type resolver struct {
	// rootURI uri.URI
	root ast.Node
	// A map of filenames from node.Loc().Filename to the root AST node
	// This is used to find the root AST node of any node.
	stackCache map[ast.Node][]ast.Node
	roots      map[string]ast.Node
	vm         *jsonnet.VM
}

var _ = (analysis.Resolver)(new(resolver))

func NewResolver(root ast.Node, vm *jsonnet.VM) *resolver {

	return &resolver{
		root:       root,
		roots:      map[string]ast.Node{root.Loc().FileName: root},
		vm:         vm,
		stackCache: map[ast.Node][]ast.Node{},
	}
}

func (r *resolver) NodeAt(loc ast.Location) (node ast.Node, stack []ast.Node) {
	stack = analysis.StackAtLoc(r.root, loc)
	if len(stack) == 0 {
		return nil, nil
	}
	node = stack[len(stack)-1]
	r.stackCache[node] = stack
	return node, stack
}

func (r *resolver) Vars(from ast.Node) analysis.VarMap {
	if from == nil || from.Loc() == nil {
		return analysis.VarMap{}
	}
	root := r.roots[from.Loc().FileName]
	if root == nil {
		panic(fmt.Errorf("invariant: resolving var from %T where no root was imported", from))
	}
	if stk := r.stackCache[from]; len(stk) > 0 {
		return analysis.StackVars(stk)
	}
	stk := analysis.StackAtNode(root, from)
	return analysis.StackVars(stk)
}

func (r *resolver) Import(from, path string) ast.Node {
	root, _, err := r.vm.ImportAST(from, path)
	if err != nil {
		return nil
	}
	if root != nil {
		r.roots[root.Loc().FileName] = root
	}
	return root
}
