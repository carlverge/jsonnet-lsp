package analysis

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-jsonnet/ast"
)

type ValueType int

const (
	AnyType      ValueType = 0
	FunctionType ValueType = 1
	ObjectType   ValueType = 2
	ArrayType    ValueType = 3
	BooleanType  ValueType = 4
	NumberType   ValueType = 5
	StringType   ValueType = 6
	NullType     ValueType = 7
)

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
	default:
		return "<invalid value type>"
	}
}

type Param struct {
	Name    string            `json:"name"`
	Comment []string          `json:"comment,omitempty"`
	Range   ast.LocationRange `json:"-"`
	Type    ValueType         `json:"type"`
	Default ast.Node          `json:"-"`
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
	Comment    []string  `json:"comment,omitempty"`
	Params     []Param   `json:"params,omitempty"`
	Return     ast.Node  `json:"-"`
	ReturnType ValueType `json:"returnType"`
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
	if f.ReturnType != AnyType {
		res += " -> " + f.ReturnType.String()
	}
	return res
}

type Field struct {
	Name    string            `json:"name,omitempty"`
	Type    ValueType         `json:"type"`
	Range   ast.LocationRange `json:"-"`
	Comment []string          `json:"comment,omitempty"`
	Hidden  bool              `json:"hidden,omitempty"`
	Node    ast.Node          `json:"-"`
}

type Object struct {
	Fields         []Field           `json:"fields"`
	FieldMap       map[string]*Field `json:"-"`
	AllFieldsKnown bool              `json:"allFieldsKnown"`
}

type Value struct {
	Type    ValueType         `json:"type"`
	Range   ast.LocationRange `json:"-"`
	Comment []string          `json:"comment,omitempty"`
	Node    ast.Node          `json:"-"`

	Object   *Object   `json:"object,omitempty"`
	Function *Function `json:"function,omitempty"`
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

func functionToValue(node *ast.Function) *Value {
	res := &Value{
		Type:     FunctionType,
		Range:    node.LocRange,
		Node:     node,
		Comment:  foddersToComment(node, node.ParenLeftFodder, node.ParenRightFodder),
		Function: &Function{Params: make([]Param, len(node.Parameters))},
	}
	_, res.Function.Return = UnwindLocals(node.Body)
	res.Function.ReturnType, _ = simpleToValueType(res.Function.Return)

	for i, param := range node.Parameters {
		res.Function.Params[i] = Param{
			Name:    string(param.Name),
			Default: param.DefaultArg,
			Range:   param.LocRange,
			Comment: foddersToComment(param.DefaultArg, param.NameFodder, param.EqFodder, param.CommaFodder),
		}
	}

	return res
}

func objectToValue(node *ast.DesugaredObject) *Value {
	res := &Value{
		Type:    ObjectType,
		Range:   node.LocRange,
		Node:    node,
		Comment: foddersToComment(node, node.Fodder),
		Object: &Object{
			FieldMap: map[string]*Field{},
		},
	}

	unknownFields := false
	for _, fld := range node.Fields {
		nt, ok := fld.Name.(*ast.LiteralString)
		if !ok {
			unknownFields = true
			continue
		}

		ft, _ := simpleToValueType(fld.Body)
		res.Object.Fields = append(res.Object.Fields, Field{
			Name:    nt.Value,
			Type:    ft,
			Comment: foddersToComment(fld.Body, nt.Fodder), // XXX: Name comments?
			Range:   fld.LocRange,
			Node:    fld.Body,
			Hidden:  fld.Hide == ast.ObjectFieldHidden,
		})
		res.Object.FieldMap[nt.Value] = &(res.Object.Fields[len(res.Object.Fields)-1])
	}
	res.Object.AllFieldsKnown = !unknownFields

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

	return &Value{Type: AnyType, Range: node.LocRange}
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
	case *ast.LiteralBoolean:
		return BooleanType, true
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
	res.Type, _ = simpleToValueType(node)
	if node.Loc() != nil {
		res.Range = *node.Loc()
	}
	return res
}

func mergeObjectValues(lhs, rhs *Value) *Value {
	// make a new value object
	res := &Value{
		Type:    ObjectType,
		Range:   rhs.Range,
		Comment: rhs.Comment,
		Node:    rhs.Node,
		Object: &Object{
			FieldMap:       map[string]*Field{},
			AllFieldsKnown: lhs.Object.AllFieldsKnown && rhs.Object.AllFieldsKnown,
		},
	}
	for name, fld := range lhs.Object.FieldMap {
		// add only if not in the RHS
		if rhv := rhs.Object.FieldMap[name]; rhv == nil {
			res.Object.Fields = append(res.Object.Fields, *fld)
			res.Object.FieldMap[name] = fld
		}
	}
	for name, fld := range rhs.Object.FieldMap {
		res.Object.Fields = append(res.Object.Fields, *fld)
		res.Object.FieldMap[name] = fld
	}
	return res
}

type Resolver interface {
	// Gets the variable with name `name` the ast node `from`
	// We need from, as the available variables change depending
	// on where in the document the caller is
	Vars(from ast.Node) VarMap
	NodeAt(loc ast.Location) (node ast.Node, stack []ast.Node)
	Import(from, path string) ast.Node
}

func NodeToValue(node ast.Node, resolver Resolver) (res *Value) {
	// short circuit the more complicated logic if it's a known leaf value
	// that cannot have more complex values
	if _, isLeaf := simpleToValueType(node); isLeaf {
		return defaultToValue(node)
	}

	switch node := node.(type) {
	case *ast.Array:
		return &Value{
			Type:    ArrayType,
			Node:    node,
			Range:   node.LocRange,
			Comment: foddersToComment(node, node.Fodder, node.CloseFodder),
		}
	case *ast.LiteralString:
		return &Value{
			Type:    StringType,
			Node:    node,
			Range:   node.LocRange,
			Comment: []string{node.Value},
		}
	case *ast.LiteralNumber:
		return &Value{
			Type:    StringType,
			Node:    node,
			Range:   node.LocRange,
			Comment: []string{node.OriginalString},
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

		v := resolver.Vars(node).Get(string(node.Id))
		if v != nil && v.Node != nil {
			return NodeToValue(v.Node, resolver)
		}
		return defaultToValue(node)
	case *ast.Apply:
		targfn := NodeToValue(node.Target, resolver)
		if targfn.Function == nil || targfn.Function.Return == nil {
			return defaultToValue(node)
		}
		return NodeToValue(targfn.Function.Return, resolver)
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
				stdfn := StdLibFunctions[idx.Value]
				if stdfn != nil {
					return &Value{Type: FunctionType, Comment: stdfn.Comment, Function: stdfn}
				}
				return defaultToValue(node)
			}

			// object dotted access
			if lhs.Object != nil && lhs.Object.FieldMap[idx.Value] != nil {
				return NodeToValue(lhs.Object.FieldMap[idx.Value].Node, resolver)
			}
		}
		return defaultToValue(node)
	case *ast.Binary:
		if node.Op == ast.BopPlus {
			// object templates
			lhs, rhs := NodeToValue(node.Left, resolver), NodeToValue(node.Right, resolver)
			if lhs.Object != nil && rhs.Object != nil {
				return mergeObjectValues(lhs, rhs)
			}
		}
		return defaultToValue(node)
	case *ast.DesugaredObject:
		return objectToValue(node)
	case *ast.Function:
		return functionToValue(node)
	case *ast.Import:
		return importToValue(node, resolver)
	default:
		return defaultToValue(node)
	}
}
