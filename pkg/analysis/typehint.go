package analysis

import (
	"fmt"
	"strings"

	"github.com/carlverge/jsonnet-lsp/pkg/analysis/annotation"
	"github.com/google/go-jsonnet/ast"
)

type TypeInfo struct {
	ValueType     ValueType   `json:"valueType"`
	Union         []*TypeInfo `json:"union,omitempty"`
	Element       *TypeInfo   `json:"element,omitempty"`
	Function      *Function   `json:"function,omitempty"`
	Object        *Object     `json:"object,omitempty"`
	TypeParam     string      `json:"typeParam,omitempty"`
	TypeHintError error       `json:"typeHintError,omitempty"`
}

// const typeDeclPreamble = `
// local string = null;
// local number = null;
// local boolean = null;
// local array = null;
// local object = null;
// `

func (t *TypeInfo) String() string {
	if t == nil {
		return "<nil>"
	}
	if t.TypeHintError != nil {
		return fmt.Sprintf("error %q", t.TypeHintError.Error())
	}
	switch t.ValueType {
	case AnyType:
		fallthrough
	case StringType:
		fallthrough
	case NullType:
		fallthrough
	case BooleanType:
		fallthrough
	case NumberType:
		return t.ValueType.String()
	case ArrayType:
		fallthrough
	case ObjectType:
		if t.Element != nil {
			return fmt.Sprintf("%s[%s]", t.ValueType, t.Element.String())
		}
		return t.ValueType.String()
	case TypeParameterType:
		return t.TypeParam
	case UnionType:
		res := make([]string, len(t.Union))
		for i := range t.Union {
			res[i] = t.Union[i].String()
		}
		return strings.Join(res, " | ")
	case FunctionType:
		if t.Function == nil {
			return t.ValueType.String()
		}
		fn := t.Function
		params := make([]string, len(fn.Params))
		for i, pr := range fn.Params {
			if pr.TypeHint == nil {
				params[i] = pr.Name
			} else {
				params[i] = fmt.Sprintf("%s: %s", pr.Name, pr.TypeHint)
			}
		}
		ret := ""
		if fn.ReturnHint != nil {
			ret = " -> " + fn.ReturnHint.String()
		}
		return fmt.Sprintf("function(%s)%s", strings.Join(params, ", "), ret)
	default:
		panic("invariant: unknown value type " + t.ValueType.String())
	}
}

func (t *TypeInfo) isSubtypeOf(o *TypeInfo) bool {
	if t.ValueType == AnyType || o.ValueType == AnyType {
		return true
	}
	if o.ValueType == UnionType {
		for _, u := range o.Union {
			if t.isSubtypeOf(u) {
				return true
			}
		}
		return false
	}
	switch o.ValueType {
	case StringType:
		fallthrough
	case NullType:
		fallthrough
	case BooleanType:
		fallthrough
	case NumberType:
		return t.ValueType == o.ValueType
	case ArrayType:
		if t.Element != nil && o.Element != nil {
			return t.Element.isSubtypeOf(o.Element)
		}
		// if either one has no element decl, then just compare value types
		return t.ValueType == o.ValueType
	case ObjectType:
		// TODO: add case for type decl object
		if t.Element != nil && o.Element != nil {
			return t.Element.isSubtypeOf(o.Element)
		}
		// if either one has no element decl, then just compare value types
		return t.ValueType == o.ValueType
	case FunctionType:
		if t.Function == nil || o.Function == nil {
			return t.ValueType == o.ValueType
		}
		if len(t.Function.Params) != len(o.Function.Params) {
			return false
		}
		for i := range t.Function.Params {
			tp, to := t.Function.Params[i], o.Function.Params[i]
			if tp.TypeHint == nil || to.TypeHint == nil {
				continue
			}
			if !tp.TypeHint.isSubtypeOf(to.TypeHint) {
				return false
			}
		}
		if t.Function.ReturnHint != nil && o.Function.ReturnHint != nil {
			return t.Function.ReturnHint.isSubtypeOf(o.Function.ReturnHint)
		}
		return true
	case TypeParameterType:
		return true
	}
	return false
}

func solveTypeParam(caller *TypeInfo, param *TypeInfo, resolver Resolver) map[string]*TypeInfo {
	solutions := make(map[string]*TypeInfo)

	if param.ValueType == TypeParameterType {
		// This is a direct solution, the whole type is a type parameter.
		solutions[param.TypeParam] = caller
		return solutions
	}

	if param.ValueType != caller.ValueType {
		// The types don't match, this is a problem.
		return solutions
	}

	switch param.ValueType {
	case ObjectType:
		fallthrough
	case ArrayType:
		if caller.Element == nil || param.Element == nil {
			break
		}
		// If the param is an array, solve for the array's element type.
		elementSolutions := solveTypeParam(caller.Element, param.Element, resolver)
		for k, v := range elementSolutions {
			solutions[k] = v
		}
	case FunctionType:
		// If the param is a function, solve for the function's return type and each parameter type.
		if caller.Function.ReturnHint != nil && param.Function.ReturnHint != nil {
			returnSolutions := solveTypeParam(caller.Function.ReturnHint, param.Function.ReturnHint, resolver)
			for k, v := range returnSolutions {
				solutions[k] = v
			}
		}
		for i, p := range param.Function.Params {
			callerParam := caller.Function.Params[i]
			if callerParam.TypeHint != nil {
				paramSolutions := solveTypeParam(callerParam.TypeHint, p.TypeHint, resolver)
				for k, v := range paramSolutions {
					solutions[k] = v
				}
			}
		}
	}

	return solutions
}

func needsTypeInference(caller *ast.Apply, callee *Value, resolver Resolver) bool {
	if callee.Type.Function == nil || callee.Type.Function.ReturnHint == nil {
		return false
	}
	return callee.Type.Function.ReturnHint.hasTypeParam()
}

func inferTypeParameters(caller *ast.Apply, target *Value, resolver Resolver) (map[string]*TypeInfo, error) {
	// res := &TypeInfo{}
	// preconditions:
	//  - typeparam validation already done
	//  - target has a Function
	//  - return hint exists and has type param

	fn := target.Type.Function

	// Iterate over each positional argument in the caller.
	solutions := map[string]*TypeInfo{}

	for i, argNode := range caller.Arguments.Positional {
		// Get the corresponding parameter from the callee's function.
		if i >= len(fn.Params) {
			return nil, fmt.Errorf("too many arguments for function")
		}

		if fn.Params[i].TypeHint == nil || !fn.Params[i].TypeHint.hasTypeParam() {
			continue
		}

		paramHint := fn.Params[i].TypeHint
		argVal := NodeToValue(argNode.Expr, resolver)

		if argVal.TypeHint != nil {
			soln := solveTypeParam(argVal.TypeHint, paramHint, resolver)
			for k, t := range soln {
				if seen, ok := solutions[k]; ok {
					if seen.String() != t.String() {
						return nil, fmt.Errorf("type parameter '%s' has conflicting inferred types '%s' and '%s'", k, seen.String(), t.String())
					}
				} else {
					solutions[k] = t
				}
			}
		} else {
			// If we don't have a hint, use the inferred type
			soln := solveTypeParam(&argVal.Type, paramHint, resolver)
			for k, t := range soln {
				if seen, ok := solutions[k]; ok {
					if seen.String() != t.String() {
						return nil, fmt.Errorf("type parameter '%s' has conflicting inferred types '%s' and '%s'", k, seen.String(), t.String())
					}
				} else {
					solutions[k] = t
				}
			}
		}
	}

	return solutions, nil
}

func solveTypeParameterInfo(th *TypeInfo, typeparams map[string]*TypeInfo, resolver Resolver) (*TypeInfo, error) {

	switch th.ValueType {
	case TypeParameterType:
		soln := typeparams[th.TypeParam]
		if soln == nil {
			return nil, fmt.Errorf("unable to resolve type parameter '%s'", th.TypeParam)
		}
		return soln, nil
	case ObjectType:
		fallthrough
	case ArrayType:
		if th.Element != nil {
			res := *th
			soln, err := solveTypeParameterInfo(th.Element, typeparams, resolver)
			if err != nil {
				return nil, err
			}
			res.Element = soln
			return &res, nil
		}
		// no type parameter
		return th, nil
	case FunctionType:
		// make a copy of the function and parameters, as we will be replacing the types
		copyFn := *th.Function
		copyFn.Params = make([]Param, len(th.Function.Params))
		copy(copyFn.Params, th.Function.Params)

		for i, pr := range th.Function.Params {
			if pr.TypeHint == nil {
				continue
			}
			soln, err := solveTypeParameterInfo(pr.TypeHint, typeparams, resolver)
			if err != nil {
				return nil, err
			}
			copyFn.Params[i].TypeHint = soln
		}

		if copyFn.ReturnHint != nil {
			soln, err := solveTypeParameterInfo(copyFn.ReturnHint, typeparams, resolver)
			if err != nil {
				return nil, err
			}
			copyFn.ReturnHint = soln
		}

		res := *th
		res.Function = &copyFn
		return &res, nil
	// in all other cases, we cannot have a type parameter so return the original type
	default:
		return th, nil
	}
}

// func solveApplyTypeParams(caller *ast.Apply, callee *Value, resolver Resolver) (*TypeInfo, error) {

// 	// Create a map to store the solutions for type parameters.
// 	solutions := make(map[string]*TypeInfo)

// 	// // If the callee is not a function, return an error.
// 	// if callee.TypeInfo.ValueType != FunctionType {
// 	//     return nil, errors.New("Callee is not a function")
// 	// }
// 	target := callee.Type.Function

// 	// Iterate over each positional argument in the caller.
// 	for i, argNode := range caller.Arguments.Positional {

// 		// Get the corresponding parameter from the callee's function.
// 		if i >= len(target.Params) {
// 			return nil, fmt.Errorf("Too many arguments for function")
// 		}
// 		param := target.Params[i]
// 		if !param.TypeHint.hasTypeParam() {
// 			continue
// 		}

// 		// Convert the ast.Node to a *Value
// 		argValue := NodeToValue(argNode.Expr, resolver)

// 		// If the parameter or the argument contain a type parameter, solve it.
// 		paramSolutions := solveTypeParam(argValue.TypeHint, param.TypeHint, resolver)
// 		for k, v := range paramSolutions {
// 			solutions[k] = v
// 		}
// 	}

// 	// For now, ignore named arguments (caller.Arguments.Named). If your function
// 	// supports named arguments, you would need to match these by name instead of
// 	// by position.

// 	// Create a copy of the callee's type info, to avoid modifying the original.
// 	resolvedTypeInfo := *callee.TypeHint

// 	// Use the solutions to replace type parameters in the resolved type info.
// 	// Note: This will require another function, not shown here, which would be a
// 	// recursive function that replaces type parameters throughout a TypeInfo.
// 	// replaceTypeParameters(&resolvedTypeInfo, solutions)

// 	return &resolvedTypeInfo, nil
// }

// func solveTypeParams(app *ast.Apply, fn *Value, resolver Resolver) map[string]map[string]*TypeInfo {
// 	res := map[string]map[string]*TypeInfo{}
// 	app.Arguments.Named[0].
// }

func (t *TypeInfo) hasTypeParam() bool {
	if t == nil {
		return false
	}
	if t.ValueType == TypeParameterType {
		return true
	}
	for _, u := range t.Union {
		if u.hasTypeParam() {
			return true
		}
	}
	if t.Element.hasTypeParam() {
		return true
	}
	if t.Function != nil {
		if t.Function.ReturnHint.hasTypeParam() {
			return true
		}
		for _, pr := range t.Function.Params {
			if pr.TypeHint.hasTypeParam() {
				return true
			}
		}
	}
	// can an object have a type param field? I don't think so
	return false
}

// var regexUnknownVarErr = regexp.MustCompile(`.* Unknown variable: ([A-Za-z0-9_]+)$`)

// func isUnknownVarErr(msg string) string {
// 	match := regexUnknownVarErr.FindStringSubmatch(msg)
// 	if match == nil {
// 		return ""
// 	}
// 	return match[1]
// }

func isTypeDeclComments(comments []string) (string, bool) {
	if len(comments) == 0 {
		return "", false
	}
	c := comments[0]
	if !(strings.HasPrefix(c, "/*:") && strings.HasSuffix(c, "*/")) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(c, "/*:"), "*/")), true
}

// func parseTypeDeclAST(decl string) (ast.Node, error) {
// 	seen := map[string]bool{}
// 	preamble := typeDeclPreamble
// 	out, err := jsonnet.SnippetToAST("anon", preamble+decl)
// 	for err != nil {
// 		// This is a bit of a hack. Instead of implementing a whole new parser, re-use the jsonnet AST parser.
// 		// However, if the AST contains unknown variables the parsing fails with an error (which is not ideal)
// 		// So if we see an unknown variable error, just declare that variable in a pre-amble so the parsing works.
// 		msg := err.Error()
// 		unknownVar := isUnknownVarErr(msg)
// 		// just in case, guard against infinite loops where we don't handle the error
// 		if seen[msg] || unknownVar == "" {
// 			return nil, err
// 		}

// 		preamble += fmt.Sprintf("local %s = null;\n", unknownVar)
// 		out, err = jsonnet.SnippetToAST("anon", preamble+decl)
// 	}

// 	if out != nil {
// 		_, body := UnwindLocals(out)
// 		return body, nil
// 	}
// 	return nil, err
// }

// func unwindTypeDeclBinOp(node *ast.Binary) ([]ast.Node, error) {
// 	if node.Op != ast.BopBitwiseOr {
// 		return nil, fmt.Errorf("unexpected binary op in type hint: '%s'", node.Op.String())
// 	}

// 	res := []ast.Node{}
// 	switch n := node.Left.(type) {
// 	case *ast.Binary:
// 		elems, err := unwindTypeDeclBinOp(n)
// 		if err != nil {
// 			return nil, err
// 		}
// 		res = append(res, elems...)
// 	default:
// 		res = append(res, node.Left)
// 	}

// 	switch n := node.Right.(type) {
// 	case *ast.Binary:
// 		elems, err := unwindTypeDeclBinOp(n)
// 		if err != nil {
// 			return nil, err
// 		}
// 		res = append(res, elems...)
// 	default:
// 		res = append(res, node.Right)
// 	}

// 	return res, nil
// }

func annotationNodeToTypeDecl(orig ast.Node, node annotation.Node, resolver Resolver) (*TypeInfo, error) {
	res := &TypeInfo{}
	switch n := node.(type) {
	// TODO: literal values, f.ex for enums
	case *annotation.NullNode:
		return &TypeInfo{ValueType: NullType}, nil
	case *annotation.StringNode:
		return &TypeInfo{ValueType: StringType}, nil
	case *annotation.BooleanNode:
		return &TypeInfo{ValueType: BooleanType}, nil
	case *annotation.NumberNode:
		return &TypeInfo{ValueType: NumberType}, nil
	case *annotation.TypeParameterNode:
		return &TypeInfo{ValueType: TypeParameterType, TypeParam: n.Name}, nil
	case *annotation.ArrayNode:
		if n.ElementType == nil {
			return &TypeInfo{ValueType: ArrayType}, nil
		}
		elem, err := annotationNodeToTypeDecl(orig, n.ElementType, resolver)
		if err != nil {
			return nil, err
		}
		return &TypeInfo{ValueType: ArrayType, Element: elem}, nil
	case *annotation.UnionNode:
		res := &TypeInfo{ValueType: UnionType, Union: make([]*TypeInfo, len(n.Types))}
		for i, u := range n.Types {
			tp, err := annotationNodeToTypeDecl(orig, u, resolver)
			if err != nil {
				return nil, err
			}
			res.Union[i] = tp
		}
		return res, nil
	case *annotation.FunctionNode:
		res := &TypeInfo{ValueType: FunctionType, Function: &Function{
			Params: make([]Param, len(n.Params)),
		}}

		if n.Return != nil {
			tp, err := annotationNodeToTypeDecl(orig, n.Return, resolver)
			if err != nil {
				return nil, err
			}
			res.Function.ReturnHint = tp
		}

		for i, p := range n.Params {
			if p.Type != nil {
				tp, err := annotationNodeToTypeDecl(orig, p.Type, resolver)
				if err != nil {
					return nil, err
				}
				res.Function.Params[i] = Param{Name: p.Name, TypeHint: tp}
			} else {
				res.Function.Params[i] = Param{Name: p.Name}
			}
		}

		return res, nil
	case *annotation.ObjectNode:

		if n.ElementType != nil {
			tp, err := annotationNodeToTypeDecl(orig, n.ElementType, resolver)
			if err != nil {
				return nil, err
			}
			return &TypeInfo{
				ValueType: ObjectType,
				Element:   tp,
				Object: &Object{
					AllFieldsKnown: false,
					FieldMap:       map[string]*Field{},
				},
			}, nil
		}

		if n.Fields != nil {
			res := &TypeInfo{ValueType: ObjectType, Object: &Object{
				Fields:         make([]Field, len(n.Fields)),
				FieldMap:       map[string]*Field{},
				AllFieldsKnown: true,
			}}
			for i, p := range n.Fields {
				tp, err := annotationNodeToTypeDecl(orig, p.Type, resolver)
				if err != nil {
					return nil, err
				}

				res.Object.Fields[i] = Field{
					Name:     p.Name,
					TypeHint: tp,
				}
				res.Object.FieldMap[p.Name] = &res.Object.Fields[i]
			}
			return res, nil
		}

		return &TypeInfo{ValueType: ObjectType, Object: &Object{
			AllFieldsKnown: false,
			FieldMap:       map[string]*Field{},
		}}, nil

	// TODO: dotted variable access
	case *annotation.IdentNode:
		// TODO: Resolver can be nil during variable resolution
		if resolver == nil {
			return &TypeInfo{}, nil
		}

		v := resolver.Vars(orig).Get(string(n.Name))
		if v == nil {
			return nil, fmt.Errorf("unknown variable in type hint '%s'", n.Name)
		}
		val := NodeToValue(v.Node, resolver)
		if val == nil {
			return nil, fmt.Errorf("unknown variable in type hint '%s'", n.Name)
		}
		switch val.Type.ValueType {
		case ObjectType:
			res.ValueType = ObjectType
			res.Object = val.Type.Object
		default:
			return nil, fmt.Errorf("cannot use non-object variable in type hint '%s'", n.Name)
		}
	}
	return res, nil
}

// func isFuncNode(node ast.Node) bool {
// 	_, ok := node.(*ast.Function)
// 	return ok
// }

// func isTypeParamFormat(s string) bool {
// 	return len(s) == 1 && s[0] >= 'A' && s[0] <= 'Z'
// }

func typeHintCommentsToInfo(orig ast.Node, resolver Resolver, comments []string) *TypeInfo {
	hint, ok := isTypeDeclComments(comments)
	if !ok {
		return nil
	}
	hintNode, err := annotation.Parse(hint)
	// hintNode, err := parseTypeDeclAST(hint)
	if err != nil || hintNode == nil {
		return &TypeInfo{TypeHintError: err}
	}
	th, err := annotationNodeToTypeDecl(orig, hintNode, resolver)
	if err != nil {
		return &TypeInfo{TypeHintError: err}
	}
	return th
}

// func functionTypeHints(node *ast.Function, resolver Resolver) *TypeInfo {
// 	res := &TypeInfo{
// 		ValueType:  FunctionType,
// 		Definition: node,
// 		Function: &Function{
// 			Params: make([]Param, len(node.Parameters)),
// 		},
// 	}
// 	// t.Logf("fn body comments: %+v", foddersToComment(fn.Body))
// 	for idx, pr := range node.Parameters {
// 		var comments []string
// 		if pr.DefaultArg != nil {
// 			comments = foddersToComment(nil, pr.EqFodder)
// 			// t.Logf("fn-arg-%d:%s comments: %+v", idx, pr.Name, )
// 		} else if idx == len(node.Parameters)-1 {
// 			comments = foddersToComment(nil, node.ParenRightFodder)
// 			// t.Logf("fn-arg-%d:%s comments: %+v", idx, pr.Name, )
// 		} else {
// 			comments = foddersToComment(nil, pr.CommaFodder)
// 			// t.Logf("fn-arg-%d:%s comments: %+v", idx, pr.Name, foddersToComment(nil, pr.CommaFodder))
// 		}
// 		res.Function.Params[idx].TypeHint, _ = typeHintCommentsToInfo(node, resolver, comments)
// 	}

// 	res.Function.ReturnHint, _ = typeHintCommentsToInfo(node, resolver, foddersToComment(node.Body))

// 	return res
// }

// func objectTypeHints(node *ast.DesugaredObject, resolver Resolver) *TypeInfo {
// 	res := &TypeInfo{
// 		ValueType:  ObjectType,
// 		Definition: node,
// 		Object: &Object{
// 			FieldMap:       map[string]*Field{},
// 			AllFieldsKnown: true,
// 		},
// 	}

// 	for _, fld := range node.Fields {
// 		nt, ok := fld.Name.(*ast.LiteralString)
// 		if !ok {
// 			res.Object.AllFieldsKnown = false
// 			continue
// 		}

// 		// ft, _ := simpleToValueType(fld.Body)
// 		th, _ := typeHintCommentsToInfo(fld.Body, resolver, foddersToComment(fld.Body))

// 		res.Type.Object.Fields = append(res.Type.Object.Fields, Field{
// 			Name: nt.Value,
// 			Comment: foddersToComment(fld.Body, nt.Fodder), // XXX: Name comments?
// 			Range:   fld.LocRange,
// 			Node:    fld.Body,
// 			Hidden:  fld.Hide == ast.ObjectFieldHidden,
// 		})
// 		res.Type.Object.FieldMap[nt.Value] = &(res.Type.Object.Fields[len(res.Type.Object.Fields)-1])
// 	}

// 	return res
// }
