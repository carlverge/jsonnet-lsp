package analysis

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/carlverge/jsonnet-lsp/pkg/analysis/annotation"
	"github.com/google/go-jsonnet/ast"
)

type ValueType int

const (
	AnyType           ValueType = 0
	FunctionType      ValueType = 1
	ObjectType        ValueType = 2
	ArrayType         ValueType = 3
	BooleanType       ValueType = 4
	NumberType        ValueType = 5
	StringType        ValueType = 6
	NullType          ValueType = 7
	TypeParameterType ValueType = 8
	UnionType         ValueType = 9
)

func NewValueType(v string) (ValueType, bool) {
	switch v {
	case "any":
		return AnyType, true
	case "function":
		return FunctionType, true
	case "object":
		return ObjectType, true
	case "array":
		return ArrayType, true
	case "boolean":
		return BooleanType, true
	case "number":
		return NumberType, true
	case "string":
		return StringType, true
	case "union":
		return UnionType, false
	case "null":
		return NullType, true
	default:
		return AnyType, false
	}
}

func (v ValueType) String() string {
	switch v {
	case AnyType:
		return "any"
	case FunctionType:
		return "function"
	case ObjectType:
		return "object"
	case ArrayType:
		return "array"
	case BooleanType:
		return "boolean"
	case NumberType:
		return "number"
	case StringType:
		return "string"
	case NullType:
		return "null"
	case UnionType:
		return "union"
	case TypeParameterType:
		return "typeparam"
	default:
		return "<invalid value type>"
	}
}

func (v ValueType) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

type Param struct {
	Name     string            `json:"name"`
	Comment  []string          `json:"-"`
	Range    ast.LocationRange `json:"-"`
	Type     ValueType         `json:"type"`
	Default  ast.Node          `json:"-"`
	TypeHint *TypeInfo         `json:"typeHint,omitempty"`
}

func (p *Param) String() string {
	res := p.Name
	if p.Type != AnyType {
		res += ": " + p.Type.String()
	}
	if p.Default != nil {
		res += "=null"
	}
	return res
}

type Function struct {
	Comment []string `json:"-"`
	Params  []Param  `json:"params,omitempty"`
	Return  ast.Node `json:"-"`
	// ReturnType ValueType `json:"returnType"`
	ReturnType TypeInfo  `json:"returnType,omitempty"`
	ReturnHint *TypeInfo `json:"returnHint,omitempty"`
}

func (f *Function) String() string {
	if f == nil {
		return "()"
	}
	params := make([]string, len(f.Params))
	for i := range f.Params {
		params[i] = f.Params[i].String()
	}
	res := fmt.Sprintf("(%s)", strings.Join(params, ", "))
	if f.ReturnType.ValueType != AnyType {
		res += " -> " + f.ReturnType.ValueType.String()
	}
	return res
}

type Field struct {
	Name     string            `json:"name,omitempty"`
	Type     TypeInfo          `json:"type"`
	TypeHint *TypeInfo         `json:"typeHint,omitempty"`
	Range    ast.LocationRange `json:"-"`
	Comment  []string          `json:"-"`
	Hidden   bool              `json:"hidden,omitempty"`
	Node     ast.Node          `json:"-"`
}

type Object struct {
	Fields         []Field           `json:"fields,omitempty"`
	FieldMap       map[string]*Field `json:"-"`
	AllFieldsKnown bool              `json:"allFieldsKnown"`
	Supers         []*Value          `json:"supers,omitempty"`
}

func (o *Object) GetField(name string) *Field {
	if o.FieldMap != nil {
		if v, ok := o.FieldMap[name]; ok {
			return v
		}
	}
	// check supers in reverse order
	for i := len(o.Supers) - 1; i >= 0; i-- {
		if o.Supers[i].Type.Object != nil && o.Supers[i].Type.Object.FieldMap != nil {
			if v, ok := o.Supers[i].Type.Object.FieldMap[name]; ok {
				return v
			}
		}
	}
	return nil
}

type Value struct {
	Range   ast.LocationRange `json:"-"`
	Comment []string          `json:"-"`
	Node    ast.Node          `json:"-"`

	// distinction between what we infer from the values, and what the hint says
	Type TypeInfo
	// this can be propogated from other hints
	// like from a return hint or from an object field
	TypeHint *TypeInfo
}

func foddersToComment(node ast.Node, fodders ...ast.Fodder) []string {
	var res []string
	if node != nil && node.OpenFodder() != nil {
		for _, elem := range *node.OpenFodder() {
			res = append(res, elem.Comment...)
		}
	}
	for _, fod := range fodders {
		for _, elem := range fod {
			res = append(res, elem.Comment...)
		}
	}
	return res
}

func commentsToType(comments []string) ValueType {
	for _, c := range comments {
		if !(strings.HasPrefix(c, "/*:") && strings.HasSuffix(c, "*/")) {
			continue
		}
		typeComment := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(c, "/*:"), "*/"))
		if vt, ok := NewValueType(typeComment); ok {
			return vt
		}
	}
	return AnyType
}

func paramTypeHintComment(idx int, node *ast.Function, param ast.Parameter, resolver Resolver) (*TypeInfo, error) {
	var comments []string
	if param.DefaultArg != nil {
		comments = foddersToComment(nil, param.EqFodder)
	} else if idx == len(node.Parameters)-1 {
		comments = foddersToComment(nil, node.ParenRightFodder)
	} else {
		comments = foddersToComment(nil, param.CommaFodder)
	}

	tc, ok := isTypeDeclComments(comments)
	if !ok {
		return nil, nil
	}

	decl, err := annotation.Parse(tc)
	if err != nil {
		return nil, err
	}

	res, err := annotationNodeToTypeDecl(node, decl, resolver)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func paramTypeComments(idx int, node *ast.Function) []string {
	if node.Parameters[idx].DefaultArg != nil {
		return foddersToComment(nil, node.Parameters[idx].EqFodder)
	} else if idx == len(node.Parameters)-1 {
		return foddersToComment(nil, node.ParenRightFodder)
	}
	return foddersToComment(nil, node.Parameters[idx].CommaFodder)
}

func paramComments(idx int, node *ast.Function, param ast.Parameter) []string {
	var comments []string
	if idx+1 == len(node.Parameters) {
		comments = foddersToComment(param.DefaultArg, param.NameFodder, param.EqFodder, node.ParenRightFodder)
	} else {
		comments = foddersToComment(param.DefaultArg, param.NameFodder, param.EqFodder, param.CommaFodder)
	}
	return comments
}

func functionToValue(node *ast.Function, resolver Resolver) *Value {
	res := &Value{
		Type: TypeInfo{
			ValueType: FunctionType,
			Function: &Function{
				Params:     make([]Param, len(node.Parameters)),
				ReturnHint: typeHintCommentsToInfo(node, resolver, foddersToComment(node.Body)),
			},
		},
		Range:   node.LocRange,
		Node:    node,
		Comment: foddersToComment(node, node.ParenLeftFodder, node.ParenRightFodder),
	}

	_, res.Type.Function.Return = UnwindLocals(node.Body)
	res.Type.Function.ReturnType.ValueType, _ = simpleToValueType(res.Type.Function.Return)

	for i, param := range node.Parameters {
		res.Type.Function.Params[i] = Param{
			Name:    string(param.Name),
			Default: param.DefaultArg,
			Range:   param.LocRange,
			Comment: paramComments(i, node, param),
			// Type:     commentsToType(comments),
			TypeHint: typeHintCommentsToInfo(node, resolver, paramTypeComments(i, node)),
		}
	}
	// For functions, copy the type into the hint as they're the same thing
	res.TypeHint = &res.Type

	return res
}

func objectToValue(node *ast.DesugaredObject, resolver Resolver) *Value {
	res := &Value{
		Type: TypeInfo{
			ValueType: ObjectType,
			Object: &Object{
				FieldMap:       map[string]*Field{},
				AllFieldsKnown: true,
			},
		},
		Range:   node.LocRange,
		Node:    node,
		Comment: foddersToComment(node, node.Fodder),
	}

	for _, fld := range node.Fields {
		nt, ok := fld.Name.(*ast.LiteralString)
		if !ok {
			// logf("unknown fld name: %T %v", fld.Name, fld.Name)
			res.Type.Object.AllFieldsKnown = false
			continue
		}

		ft, _ := simpleToValueType(fld.Body)
		res.Type.Object.Fields = append(res.Type.Object.Fields, Field{
			Name:     nt.Value,
			Type:     TypeInfo{ValueType: ft},
			TypeHint: typeHintCommentsToInfo(fld.Body, resolver, foddersToComment(fld.Body)),
			Comment:  foddersToComment(fld.Body, nt.Fodder), // XXX: Name comments?
			Range:    fld.LocRange,
			Node:     fld.Body,
			Hidden:   fld.Hide == ast.ObjectFieldHidden,
		})
		res.Type.Object.FieldMap[nt.Value] = &(res.Type.Object.Fields[len(res.Type.Object.Fields)-1])
	}

	return res
}

func importToValue(node *ast.Import, resolver Resolver) *Value {
	path := node.File.Value
	from := node.LocRange.FileName
	if root := resolver.Import(from, path); root != nil {
		// import returns the result of the jsonnet file
		// strip the locals/assertions and return the result
		_, ret := UnwindLocals(root)
		return NodeToValue(ret, resolver)
	}

	return &Value{Type: TypeInfo{ValueType: AnyType}, Range: node.LocRange}
}

var intrinsicFuncValueMapping = map[string]map[string]ValueType{
	"$std": {
		// desugared object comprehension
		"$objectFlatMerge": ObjectType,
		// desugared array comprehension
		"flatMap": ArrayType,
		// formatting (%)
		"mod": StringType,
	},
}

// these binary operations always result in the same type
var binopKnownTypes = map[ast.BinaryOp]ValueType{
	ast.BopAnd: BooleanType, ast.BopGreater: BooleanType, ast.BopGreaterEq: BooleanType,
	ast.BopIn: BooleanType, ast.BopLess: BooleanType, ast.BopLessEq: BooleanType,
	ast.BopManifestEqual: BooleanType, ast.BopManifestUnequal: BooleanType,
	ast.BopPercent: StringType,
}

func simpleToValueType(node ast.Node) (typ ValueType, isLeaf bool) {
	switch node := node.(type) {
	case *ast.LiteralNull:
		return NullType, true
	case *ast.ImportStr, *ast.ImportBin:
		return StringType, true
	case *ast.Apply:
		return knownApply(node)
	case *ast.Binary:
		if kt, ok := binopKnownTypes[node.Op]; ok {
			return kt, true
		}
		return AnyType, false
	case *ast.LiteralBoolean:
		return BooleanType, false
	case *ast.LiteralNumber:
		return NumberType, false
	case *ast.LiteralString:
		return StringType, false
	case *ast.Array:
		return ArrayType, false
	case *ast.DesugaredObject:
		return ObjectType, false
	case *ast.Function:
		return FunctionType, false
	case nil:
		return AnyType, false
	default:
		return AnyType, false
	}
}

// knownApply looks for known intrinsic functions or desugared functions
func knownApply(app *ast.Apply) (ValueType, bool) {
	idx, _ := app.Target.(*ast.Index)
	if idx == nil {
		return AnyType, false
	}
	lhs, _ := idx.Target.(*ast.Var)
	rhs, _ := idx.Index.(*ast.LiteralString)
	if lhs == nil || rhs == nil {
		return AnyType, false
	}
	mod := intrinsicFuncValueMapping[string(lhs.Id)]
	if mod == nil {
		return AnyType, false
	}
	typ, ok := mod[rhs.Value]
	return typ, ok
}

func defaultToValue(node ast.Node) *Value {
	res := &Value{Comment: foddersToComment(node)}
	res.Type.ValueType, _ = simpleToValueType(node)
	if node.Loc() != nil {
		res.Range = *node.Loc()
	}
	return res
}

// func mergeObjectValues(lhs, rhs *Value) *Value {
// make a new value object
// res := &Value{
// 	Type:    ObjectType,
// 	Range:   rhs.Range,
// 	Comment: rhs.Comment,
// 	Node:    rhs.Node,
// 	Object: &Object{
// 		FieldMap:       map[string]*Field{},
// 		AllFieldsKnown: lhs.Object != nil && rhs.Object != nil && lhs.Object.AllFieldsKnown && rhs.Object.AllFieldsKnown,
// 	},
// }

// if lhs.Object != nil {
// 	for name, fld := range lhs.Object.FieldMap {
// 		// add only if not in the RHS
// 		if rhs.Object == nil || rhs.Object.FieldMap[name] == nil {
// 			res.Object.Fields = append(res.Object.Fields, *fld)
// 			res.Object.FieldMap[name] = fld
// 		}
// 	}
// }

// if rhs.Object != nil {
// 	for name, fld := range rhs.Object.FieldMap {
// 		res.Object.Fields = append(res.Object.Fields, *fld)
// 		res.Object.FieldMap[name] = fld
// 	}
// }

// rhs.

// return res
// }

type Resolver interface {
	// Gets the variable with name `name` the ast node `from`
	// We need from, as the available variables change depending
	// on where in the document the caller is
	Vars(from ast.Node) VarMap
	NodeAt(loc ast.Location) (node ast.Node, stack []ast.Node)
	Import(from, path string) ast.Node
}

func NodeToValue(node ast.Node, resolver Resolver) (res *Value) {
	defer func() {
		logf("value => node{%s} hint{%s} infer{%s}", FmtNode(node), res.TypeHint.String(), res.Type.String())
	}()
	// short circuit the more complicated logic if it's a known leaf value
	// that cannot have more complex values
	if _, isLeaf := simpleToValueType(node); isLeaf {
		return defaultToValue(node)
	}

	switch node := node.(type) {
	case *ast.Array:
		return &Value{
			Type:    TypeInfo{ValueType: ArrayType},
			Node:    node,
			Range:   node.LocRange,
			Comment: foddersToComment(node, node.Fodder, node.CloseFodder),
		}
	case *ast.LiteralString:
		return &Value{
			Type:    TypeInfo{ValueType: StringType},
			Node:    node,
			Range:   node.LocRange,
			Comment: []string{node.Value},
		}
	case *ast.LiteralNumber:
		return &Value{
			Type:    TypeInfo{ValueType: NumberType},
			Node:    node,
			Range:   node.LocRange,
			Comment: []string{node.OriginalString},
		}
	case *ast.LiteralBoolean:
		return &Value{
			Type:    TypeInfo{ValueType: BooleanType},
			Node:    node,
			Range:   node.LocRange,
			Comment: []string{strconv.FormatBool(node.Value)},
		}
	case *ast.Local:
		if len(node.Binds) == 0 {
			return defaultToValue(node)
		}
		nb := node.Binds[0]
		nv := NodeToValue(nb.Body, resolver)
		// the local var definition will eat comments we'd expect on the child
		nv.Comment = append(nv.Comment, foddersToComment(node, nb.VarFodder, nb.EqFodder, nb.CloseFodder)...)
		return nv
	case *ast.Var:
		// hardcoded return for the stdlib
		if string(node.Id) == "std" {
			return StdLibValue
		}
		if string(node.Id) == "$std" {
			return defaultToValue(node)
		}

		v := resolver.Vars(node).Get(string(node.Id))
		if v != nil && v.Node != nil {
			// If it came from a parameter, we need to rely on the type hint
			if v.ParamFn != nil {
				return &Value{
					Node:     v.Node,
					Range:    v.Loc,
					Type:     v.Type,
					TypeHint: typeHintCommentsToInfo(v.ParamFn, resolver, paramTypeComments(v.ParamPos, v.ParamFn)),
				}
			}
			return NodeToValue(v.Node, resolver)
		}

		// function parameters might not have a backing AST node with no default
		// if v != nil {
		// 	return &Value{
		// 		Node:  node,
		// 		Range: v.Loc,
		// 		Type:  v.Type,
		// 	}
		// }
		return defaultToValue(node)
	case *ast.Apply:
		logf("apply %s", FmtNode(node))
		targfn := NodeToValue(node.Target, resolver)
		if targfn.Type.Function == nil {
			return defaultToValue(node)
		}

		val := NodeToValue(targfn.Type.Function.Return, resolver)

		rh := targfn.Type.Function.ReturnHint
		if rh == nil {
			logf("null rh %s", FmtNode(node))
			return val
		}

		logf("returnhint %s", rh.String())

		if !rh.hasTypeParam() {
			logf("no type param %s", rh.String())
			val.TypeHint = rh
			return val
		}

		logf(" has type param: %s", FmtNode(targfn.Node))

		// The return typehint has a type parameter we need to solve for
		typeparams, err := inferTypeParameters(node, targfn, resolver)
		if err != nil {
			val.TypeHint = &TypeInfo{TypeHintError: err}
			return val
		}

		for k, p := range typeparams {
			logf(" infer %s => %s", k, p.String())
		}

		soln, err := solveTypeParameterInfo(rh, typeparams, resolver)
		if err != nil {
			val.TypeHint = &TypeInfo{TypeHintError: err}
			logf(" error solving return %s", val.TypeHint.String())
			return val
		}

		logf(" solved return %s", soln.String())
		val.TypeHint = soln
		return val
	case *ast.Index:
		switch idx := node.Index.(type) {
		case *ast.LiteralNumber:
			// Number index of an array

			target := NodeToValue(node.Target, resolver)
			idxInt, intErr := strconv.ParseInt(idx.OriginalString, 10, 64)
			targArr, _ := target.Node.(*ast.Array)

			if targArr == nil || intErr != nil || int(idxInt) >= len(targArr.Elements) {
				return defaultToValue(node)
			}

			return NodeToValue(targArr.Elements[idxInt].Expr, resolver)
		case *ast.LiteralString:
			// String index of an object
			lhs := NodeToValue(node.Target, resolver)

			// Hardcoded access of stdlib
			if lhs == StdLibValue {
				stdfn := StdLibValue.Type.Object.FieldMap[idx.Value].Type.Function
				if stdfn != nil {
					return &Value{Type: TypeInfo{ValueType: FunctionType, Function: stdfn}, Comment: stdfn.Comment}
				}
				return defaultToValue(node)
			}

			// object dotted access
			if lhs.Type.Object != nil && lhs.Type.Object.FieldMap[idx.Value] != nil {
				val := NodeToValue(lhs.Type.Object.FieldMap[idx.Value].Node, resolver)
				// TODO: grab type hint here from object field?
				return val
			}
		}
		return defaultToValue(node)
	case *ast.Binary:
		if node.Op == ast.BopPlus {
			// object templates
			lhs, rhs := NodeToValue(node.Left, resolver), NodeToValue(node.Right, resolver)
			if lhs.Type.ValueType == ObjectType && rhs.Type.Object != nil {
				rhs.Type.Object.Supers = append(rhs.Type.Object.Supers, lhs)
				return rhs
			}
		}
		return defaultToValue(node)
	case *ast.DesugaredObject:
		return objectToValue(node, resolver)
	case *ast.Function:
		return functionToValue(node, resolver)
	case *ast.Import:
		return importToValue(node, resolver)
	default:
		return defaultToValue(node)
	}
}
