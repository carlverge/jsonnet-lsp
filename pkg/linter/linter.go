package linter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/carlverge/jsonnet-lsp/pkg/analysis"
	"github.com/google/go-jsonnet/ast"
	"go.lsp.dev/protocol"
)

func FmtDiag(diag protocol.Diagnostic) string {
	return fmt.Sprintf(
		"[%s|%s|%d:%d-%d:%d] %s",
		diag.Severity,
		diag.Code,
		diag.Range.Start.Line+1,
		diag.Range.Start.Character+1,
		diag.Range.End.Line+1,
		diag.Range.End.Character+1,
		diag.Message,
	)
}

type Diagnostic = protocol.Diagnostic

func posToProto(p ast.Location) protocol.Position {
	line, col := p.Line, p.Column
	if line > 0 {
		line--
	}
	if col > 0 {
		col--
	}
	return protocol.Position{Line: uint32(line), Character: uint32(col)}
}

func rangeToProto(r ast.LocationRange) protocol.Range {
	return protocol.Range{Start: posToProto(r.Begin), End: posToProto(r.End)}
}

type varbind struct {
	def  ast.Node
	name string
}

type varbindInfo struct {
	refs  int
	loc   ast.LocationRange
	body  ast.Node
	param bool
}

func findVarbindInStack(v string, stack []ast.Node) *varbind {
	for i := len(stack) - 1; i >= 0; i-- {
		switch n := stack[i].(type) {
		case *ast.Local:
			for _, b := range n.Binds {
				if string(b.Variable) == v {
					return &varbind{n, v}
				}
			}
		case *ast.DesugaredObject:
			for _, b := range n.Locals {
				if string(b.Variable) == v {
					return &varbind{n, v}
				}
			}
		case *ast.Function:
			for _, b := range n.Parameters {
				if string(b.Name) == v {
					return &varbind{n, v}
				}
			}
		}
	}
	return nil
}

func sortDiags(diags []Diagnostic) []Diagnostic {
	sort.Slice(diags, func(i, j int) bool {
		if diags[i].Range.Start.Line != diags[j].Range.Start.Line {
			return diags[i].Range.Start.Line < diags[j].Range.Start.Line
		}
		if diags[i].Range.Start.Character != diags[j].Range.Start.Character {
			return diags[i].Range.Start.Character < diags[j].Range.Start.Character
		}
		if diags[i].Range.End.Line != diags[j].Range.End.Line {
			return diags[i].Range.End.Line < diags[j].Range.End.Line
		}
		if diags[i].Range.End.Character != diags[j].Range.End.Character {
			return diags[i].Range.End.Character < diags[j].Range.End.Character
		}
		return diags[i].Message < diags[j].Message
	})
	return diags
}

func argDefaultNull(arg ast.NamedArgument) bool {
	_, ok := arg.Arg.(*ast.LiteralNull)
	return ok
}

func checkFunctionCall(fn *analysis.Value, call *ast.Apply, resolver analysis.Resolver) []Diagnostic {
	diags := []Diagnostic{}

	// Simple duplicate named arguments, only run if we don't have more function information
	if fn.Function == nil {
		seenNamed := map[string]bool{}
		for _, arg := range call.Arguments.Named {
			if seenNamed[string(arg.Name)] {
				diags = append(diags, Diagnostic{
					Range:    rangeToProto(call.LocRange),
					Code:     ArgumentCardinality,
					Severity: protocol.DiagnosticSeverityWarning,
					Message:  "duplicate named argument",
				})
			}
			seenNamed[string(arg.Name)] = true
		}
	}

	if fn.Type != analysis.AnyType && fn.Type != analysis.FunctionType {
		diags = append(diags, Diagnostic{
			Range:    rangeToProto(call.LocRange),
			Code:     TypeMismatch,
			Severity: protocol.DiagnosticSeverityError,
			Message:  fmt.Sprintf("calling non-function type '%s'", fn.Type),
		})
		return diags
	}

	fndef := fn.Function
	// we need more function information for the rest of the diags
	if fndef == nil {
		return diags
	}

	// too many arguments
	if args, nparam := (len(call.Arguments.Named) + len(call.Arguments.Positional)), len(fn.Function.Params); args > nparam {
		diags = append(diags, Diagnostic{
			Range:    rangeToProto(call.LocRange),
			Code:     ArgumentCardinality,
			Severity: protocol.DiagnosticSeverityError,
			Message:  fmt.Sprintf("too many arguments in function call (%d arguments for %d parameters)", args, nparam),
		})
	}

	paramsByName := map[string]*analysis.Param{}
	{
		minArgs := 0
		for _, param := range fn.Function.Params {
			paramsByName[param.Name] = &param
			if param.Default == nil {
				minArgs++
			}
		}

		// too few arguments
		if args := len(call.Arguments.Named) + len(call.Arguments.Positional); args < minArgs {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(call.LocRange),
				Code:     ArgumentCardinality,
				Severity: protocol.DiagnosticSeverityError,
				Message:  fmt.Sprintf("too few arguments in function call (%d arguments for %d required parameters)", args, minArgs),
			})
		}
	}

	usedParams := map[string]bool{}
	params := fn.Function.Params
	for idx, arg := range call.Arguments.Positional {
		if idx >= len(params) {
			break
		}
		param := params[idx]
		usedParams[param.Name] = true
		if param.Type == analysis.AnyType {
			continue
		}

		argVal := analysis.NodeToValue(arg.Expr, resolver)
		if argVal.Type == analysis.AnyType {
			continue
		}

		if param.Type != argVal.Type {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(call.LocRange),
				Code:     TypeMismatch,
				Severity: protocol.DiagnosticSeverityWarning,
				Message:  fmt.Sprintf("mismatched argument type for '%s' expected '%s' got '%s'", param.Name, param.Type, argVal.Type),
			})
		}
	}

	for _, arg := range call.Arguments.Named {

		if usedParams[string(arg.Name)] {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(call.LocRange),
				Code:     ArgumentCardinality,
				Severity: protocol.DiagnosticSeverityWarning,
				Message:  fmt.Sprintf("duplicate named argument '%s'", arg.Name),
			})
		}
		usedParams[string(arg.Name)] = true

		param := paramsByName[string(arg.Name)]
		if param == nil {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(call.LocRange),
				Code:     UnknownArgument,
				Severity: protocol.DiagnosticSeverityError,
				Message:  fmt.Sprintf("unknown named argument '%s'", arg.Name),
			})
			continue
		}

		if param.Type == analysis.AnyType {
			continue
		}

		argVal := analysis.NodeToValue(arg.Arg, resolver)
		if argVal.Type == analysis.AnyType {
			continue
		}

		if param.Type != argVal.Type && !(param.Type == analysis.NullType && argDefaultNull(arg)) {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(call.LocRange),
				Code:     TypeMismatch,
				Severity: protocol.DiagnosticSeverityWarning,
				Message:  fmt.Sprintf("mismatched argument type for '%s' expected '%s' got '%s'", param.Name, param.Type, argVal.Type),
			})
		}
	}

	return diags
}

func checkUnaryOp(lhs *analysis.Value, node *ast.Unary) []Diagnostic {
	if lhs.Type == analysis.AnyType {
		return nil
	}
	diags := []Diagnostic{}
	switch node.Op {
	case ast.UopNot:
		if lhs.Type != analysis.BooleanType {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(node.LocRange),
				Severity: protocol.DiagnosticSeverityWarning,
				Message:  fmt.Sprintf("expected boolean for operand of binary operator '%s' but got type '%s'", node.Op, lhs.Type),
			})
		}
	case ast.UopBitwiseNot, ast.UopPlus, ast.UopMinus:
		if lhs.Type != analysis.NumberType {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(node.LocRange),
				Severity: protocol.DiagnosticSeverityWarning,
				Message:  fmt.Sprintf("expected number for operand of binary operator '%s' but got type '%s'", node.Op, lhs.Type),
			})
		}
	}
	return diags
}

func checkIndex(target, idx *analysis.Value, node *ast.Index) []Diagnostic {
	if target.Type == analysis.AnyType || idx.Type == analysis.AnyType || target.Type == analysis.NullType {
		return nil
	}
	diags := []Diagnostic{}

	switch target.Type {
	case analysis.ArrayType:
		if idx.Type != analysis.NumberType {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(node.LocRange),
				Code:     TypeMismatch,
				Severity: protocol.DiagnosticSeverityError,
				Message:  fmt.Sprintf("cannot index array with type '%s' (expected number)", target.Type),
			})
		}
	case analysis.ObjectType:
		if idx.Type != analysis.StringType {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(node.LocRange),
				Code:     TypeMismatch,
				Severity: protocol.DiagnosticSeverityError,
				Message:  fmt.Sprintf("cannot index object with type '%s' (expected string)", target.Type),
			})
		}
		if sl, ok := idx.Node.(*ast.LiteralString); ok && target.Object != nil && target.Object.AllFieldsKnown && target.Object.FieldMap != nil {
			if _, hasfld := target.Object.FieldMap[sl.Value]; !hasfld {
				diags = append(diags, Diagnostic{
					Range:    rangeToProto(node.LocRange),
					Code:     UnknownField,
					Severity: protocol.DiagnosticSeverityWarning,
					Message:  fmt.Sprintf("object has no field '%s'", sl.Value),
				})
			}
		}
	case analysis.StringType:
		if idx.Type != analysis.NumberType {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(node.LocRange),
				Code:     TypeMismatch,
				Severity: protocol.DiagnosticSeverityError,
				Message:  fmt.Sprintf("cannot index string with type '%s' (expected number)", target.Type),
			})
		}
	default:
		diags = append(diags, Diagnostic{
			Range:    rangeToProto(target.Range),
			Code:     TypeMismatch,
			Severity: protocol.DiagnosticSeverityError,
			Message:  fmt.Sprintf("cannot index type '%s'", target.Type),
		})
	}

	return diags
}

func checkBinaryOp(lhs, rhs *analysis.Value, node *ast.Binary) []Diagnostic {
	if lhs.Type == analysis.AnyType || rhs.Type == analysis.AnyType {
		return nil
	}
	diags := []Diagnostic{}

	switch node.Op {
	case ast.BopDiv, ast.BopMult, ast.BopMinus, ast.BopShiftL, ast.BopShiftR, ast.BopBitwiseAnd, ast.BopBitwiseOr, ast.BopBitwiseXor:
		if lhs.Type != analysis.NumberType {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(lhs.Range),
				Code:     TypeMismatch,
				Severity: protocol.DiagnosticSeverityWarning,
				Message:  fmt.Sprintf("expected number for lhs of operator '%s' but got type '%s'", node.Op, lhs.Type),
			})
		}
		if rhs.Type != analysis.NumberType {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(rhs.Range),
				Code:     TypeMismatch,
				Severity: protocol.DiagnosticSeverityWarning,
				Message:  fmt.Sprintf("expected number for rhs of operator '%s' but got type '%s'", node.Op, rhs.Type),
			})
		}
	case ast.BopLess, ast.BopLessEq, ast.BopGreater, ast.BopGreaterEq:
		if !(lhs.Type == analysis.ArrayType || lhs.Type == analysis.StringType || lhs.Type == analysis.NumberType) {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(lhs.Range),
				Code:     TypeMismatch,
				Severity: protocol.DiagnosticSeverityWarning,
				Message:  fmt.Sprintf("expected number, array, or string for lhs of operator '%s' but got type '%s'", node.Op, lhs.Type),
			})
		}
		if !(rhs.Type == analysis.ArrayType || rhs.Type == analysis.StringType || rhs.Type == analysis.NumberType) {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(rhs.Range),
				Code:     TypeMismatch,
				Severity: protocol.DiagnosticSeverityWarning,
				Message:  fmt.Sprintf("expected number, array, or string for rhs of operator '%s' but got type '%s'", node.Op, rhs.Type),
			})
		}
		if lhs.Type != rhs.Type {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(node.LocRange),
				Code:     TypeMismatch,
				Severity: protocol.DiagnosticSeverityWarning,
				Message:  fmt.Sprintf("%s operator cannot compare different types '%s' and '%s'", node.Op, lhs.Type, rhs.Type),
			})
		}
	case ast.BopManifestEqual:
		if lhs.Type != rhs.Type {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(node.LocRange),
				Code:     TypeMismatch,
				Severity: protocol.DiagnosticSeverityWarning,
				Message:  fmt.Sprintf("equal comparison is never true for different types '%s' and '%s'", lhs.Type, rhs.Type),
			})
		}
	case ast.BopPlus:
		if lhs.Type != rhs.Type {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(node.LocRange),
				Code:     TypeMismatch,
				Severity: protocol.DiagnosticSeverityWarning,
				Message:  fmt.Sprintf("+ operator cannot add different types '%s' and '%s'", lhs.Type, rhs.Type),
			})
		}

	case ast.BopManifestUnequal:
		if lhs.Type != rhs.Type {
			diags = append(diags, Diagnostic{
				Range:    rangeToProto(node.LocRange),
				Code:     TypeMismatch,
				Severity: protocol.DiagnosticSeverityWarning,
				Message:  fmt.Sprintf("not equal comparison is always true for different types '%s' and '%s'", lhs.Type, rhs.Type),
			})
		}
	}

	return diags
}

func LintAST(root ast.Node, resolver analysis.Resolver) []Diagnostic {
	diags := []Diagnostic{}
	declaredVars := map[varbind]*varbindInfo{}

	analysis.WalkStack(root, func(n ast.Node, stack []ast.Node) bool {
		switch n := n.(type) {
		case *ast.Local:
			for _, b := range n.Binds {
				declaredVars[varbind{n, string(b.Variable)}] = &varbindInfo{loc: b.LocRange, body: b.Body}
			}
		case *ast.DesugaredObject:
			// add $
			declaredVars[varbind{n, "self"}] = &varbindInfo{loc: n.LocRange, body: n}
			for _, b := range n.Locals {
				declaredVars[varbind{n, string(b.Variable)}] = &varbindInfo{loc: b.LocRange, body: b.Body}
			}
		case *ast.Function:
			for _, b := range n.Parameters {
				declaredVars[varbind{n, string(b.Name)}] = &varbindInfo{loc: b.LocRange, body: b.DefaultArg, param: true}
			}
		case *ast.Var:
			// unknown variables references result in AST errors, so this should always succeed
			if bound := findVarbindInStack(string(n.Id), stack); bound != nil {
				declaredVars[*bound].refs++
			}
		case *ast.Import:
			val := analysis.NodeToValue(n, resolver)
			if val.Node == nil && val.Type == analysis.AnyType {
				diags = append(diags, Diagnostic{
					Range:    rangeToProto(n.LocRange),
					Code:     ImportNotFound,
					Severity: protocol.DiagnosticSeverityWarning,
					Message:  fmt.Sprintf("import not found: '%s'", n.File.Value),
				})
			}
		case *ast.Apply:
			targFn := analysis.NodeToValue(n.Target, resolver)
			diags = append(diags, checkFunctionCall(targFn, n, resolver)...)
		case *ast.Index:
			target := analysis.NodeToValue(n.Target, resolver)
			idx := analysis.NodeToValue(n.Index, resolver)
			diags = append(diags, checkIndex(target, idx, n)...)
		case *ast.Unary:
			lhs := analysis.NodeToValue(n.Expr, resolver)
			diags = append(diags, checkUnaryOp(lhs, n)...)
		case *ast.Binary:
			lhs := analysis.NodeToValue(n.Left, resolver)
			rhs := analysis.NodeToValue(n.Right, resolver)
			diags = append(diags, checkBinaryOp(lhs, rhs, n)...)
		}
		return true
	})

	for bind, info := range declaredVars {
		if info.refs == 0 && !info.param && !strings.HasPrefix(bind.name, "$") && bind.name != "self" {
			diags = append(diags, protocol.Diagnostic{
				Range:    rangeToProto(info.loc),
				Code:     UnusedVar,
				Severity: protocol.DiagnosticSeverityWarning,
				Message:  fmt.Sprintf("unused local variable '%s'", bind.name),
			})
		}
	}

	return sortDiags(diags)
}
