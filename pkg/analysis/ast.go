package analysis

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/go-jsonnet/ast"
)

// Logging helpers for debugging
func logf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "I%s]%s\n", time.Now().Format("0201 15:04:05.00000"), fmt.Sprintf(msg, args...))
}

func LogNodeTrace(n ast.Node, indentLevel int) {
	logf("%s", FmtNodeIndent(n, indentLevel))
}

func LogAst(root ast.Node) {
	walkStack(root, nil, func(n ast.Node, stk []ast.Node) bool {
		logf("%s", FmtNodeIndent(n, len(stk)*2))
		return true
	})
}

func FmtNodeIndent(n ast.Node, level int) string {
	return strings.Repeat(" ", level) + FmtNode(n)
}

func FmtNode(n ast.Node) string {
	loc := "<no location>"
	if n != nil && n.Loc() != nil {
		loc = n.Loc().String()
	}
	switch n := n.(type) {
	case *ast.Var:
		return fmt.Sprintf("(var=%s)[%s]", n.Id, loc)
	case *ast.LiteralString:
		return fmt.Sprintf("string:%q", n.Value)
	case *ast.LiteralNumber:
		return fmt.Sprintf("number:%s", n.OriginalString)
	case *ast.LiteralBoolean:
		return fmt.Sprintf("boolean:%v", n.Value)
	case *ast.LiteralNull:
		return "null"
	default:
		return fmt.Sprintf("(%T)[%s]", n, loc)
	case nil:
		return "<nil ast.Node>"
	}
}

func walkStack(node ast.Node, stk []ast.Node, fn func(n ast.Node, stk []ast.Node) bool) {
	stk = append(stk, node)
	if !fn(node, stk) {
		return
	}

	switch a := node.(type) {
	case *ast.Apply:
		walkStack(a.Target, stk, fn)
		for _, arg := range a.Arguments.Positional {
			walkStack(arg.Expr, stk, fn)
		}
		for _, arg := range a.Arguments.Named {
			walkStack(arg.Arg, stk, fn)
		}
	case *ast.Array:
		for _, elem := range a.Elements {
			walkStack(elem.Expr, stk, fn)
		}
	case *ast.Binary:
		walkStack(a.Left, stk, fn)
		walkStack(a.Right, stk, fn)
	case *ast.Conditional:
		walkStack(a.Cond, stk, fn)
		walkStack(a.BranchTrue, stk, fn)
		walkStack(a.BranchFalse, stk, fn)
	case *ast.Error:
		walkStack(a.Expr, stk, fn)
	case *ast.Function:
		walkStack(a.Body, stk, fn)
	case *ast.InSuper:
		walkStack(a.Index, stk, fn)
	case *ast.SuperIndex:
		walkStack(a.Index, stk, fn)
	case *ast.Index:
		walkStack(a.Target, stk, fn)
		walkStack(a.Index, stk, fn)
	case *ast.Local:
		for _, b := range a.Binds {
			walkStack(b.Body, stk, fn)
		}
		walkStack(a.Body, stk, fn)
	case *ast.DesugaredObject:
		for _, b := range a.Locals {
			walkStack(b.Body, stk, fn)
		}

		for _, field := range a.Fields {
			walkStack(field.Body, stk, fn)
		}
		for _, assert := range a.Asserts {
			walkStack(assert, stk, fn)
		}
	case *ast.Unary:
		walkStack(a.Expr, stk, fn)
	}
}

func locInNode(n ast.Node, pos ast.Location) bool {
	start, end := n.Loc().Begin, n.Loc().End
	if pos.Line < start.Line || pos.Line > end.Line {
		return false
	}
	if pos.Line == start.Line && pos.Column < start.Column {
		return false
	}
	if pos.Line == end.Line && pos.Column > end.Column {
		return false
	}
	return true
}

func unwindLocals(root ast.Node, locs []ast.Node) ([]ast.Node, ast.Node) {
	switch node := root.(type) {
	case *ast.Local:
		locs = append(locs, node)
		return unwindLocals(node.Body, locs)
	case *ast.Conditional:
		if _, falseIsErr := node.BranchFalse.(*ast.Error); falseIsErr {
			// it's an assertion
			return unwindLocals(node.BranchTrue, locs)
		}
		// it's actually a final value
		return locs, root
	default:
		return locs, root
	}
}

func UnwindLocals(root ast.Node) (VarMap, ast.Node) {
	locs, root := unwindLocals(root, nil)
	return StackVars(locs), root
}

func StackAtLoc(root ast.Node, loc ast.Location) (res []ast.Node) {
	// logf("stackAtLoc: %d:%d", loc.Line, loc.Column)
	walkStack(root, nil, func(n ast.Node, stk []ast.Node) bool {
		if n == nil || n.Loc() == nil {
			return true
		}
		if !locInNode(n, loc) {
			return false
		}
		// logNodeTrace(n, len(stk)*2)
		if len(stk) > len(res) {
			res = make([]ast.Node, len(stk))
			copy(res, stk)
		}
		return true
	})
	return res
}

func StackAtNode(root ast.Node, find ast.Node) (found []ast.Node) {
	// logf("stackAtNode: %s", fmtNode(find))
	if find.Loc() == nil {
		return nil
	}
	loc := find.Loc().End
	return StackAtLoc(root, loc)
}

type VarMap map[string]*Var

func (v VarMap) Names() []string {
	if v == nil {
		return []string{}
	}
	res := []string{}
	for name := range v {
		res = append(res, name)
	}
	sort.Strings(res)
	return res
}
func (v VarMap) Get(name string) *Var {
	if v == nil {
		return nil
	}
	return v[name]
}

type Var struct {
	Name string
	Loc  ast.LocationRange
	Node ast.Node
	Type ValueType
	// The position in the stack, used for sorting most
	// relevant autocomplete responses.
	StackPos int
}

func StackVars(stk []ast.Node) VarMap {
	res := map[string]*Var{"std": {Name: "std", StackPos: 0, Type: ObjectType}}
	var firstObject *ast.DesugaredObject
	for pos, n := range stk {
		switch n := n.(type) {
		case *ast.Local:
			for _, b := range n.Binds {
				name := string(b.Variable)
				tp, _ := simpleToValueType(b.Body)
				res[name] = &Var{
					Name:     name,
					Loc:      b.LocRange,
					Node:     b.Body,
					Type:     tp,
					StackPos: pos,
				}
			}
		case *ast.DesugaredObject:
			for _, b := range n.Locals {
				name := string(b.Variable)
				tp, _ := simpleToValueType(b.Body)
				res[name] = &Var{
					Name:     name,
					Loc:      b.LocRange,
					Node:     b.Body,
					Type:     tp,
					StackPos: pos,
				}
			}
			if firstObject == nil {
				firstObject = n
			}
			res["self"] = &Var{Name: "self", Loc: n.LocRange, Node: n, Type: ObjectType}
		case *ast.Function:
			for _, p := range n.Parameters {
				name := string(p.Name)
				res[name] = &Var{
					Name:     name,
					Loc:      p.LocRange,
					Node:     p.DefaultArg,
					StackPos: pos,
				}
			}
		}
	}
	if firstObject != nil {
		res["$"] = &Var{Name: "$", Loc: firstObject.LocRange, Node: firstObject, Type: ObjectType, StackPos: 1}
	}
	return VarMap(res)
}

var regexJsonnetIdent = regexp.MustCompile(`^[_a-zA-Z][_a-zA-Z0-9]*$`)
var jsonnetKeywords = map[string]bool{
	"assert":     true,
	"else":       true,
	"error":      true,
	"false":      true,
	"for":        true,
	"function":   true,
	"if":         true,
	"import":     true,
	"importstr":  true,
	"in":         true,
	"local":      true,
	"null":       true,
	"tailstrict": true,
	"then":       true,
	"self":       true,
	"super":      true,
	"true":       true,
}

func SafeIdent(name string) string {
	if jsonnetKeywords[name] || !regexJsonnetIdent.MatchString(name) {
		return fmt.Sprintf("[%q]", name)
	}
	return name
}
