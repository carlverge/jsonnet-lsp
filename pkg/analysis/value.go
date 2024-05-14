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

	StringValue *string

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

func functionToValue(node *ast.Function) *Value {
	res := &Value{
		Type:     FunctionType,
		Range:    node.LocRange,
		Node:     node,
		Comment:  foddersToComment(node, node.ParenLeftFodder, node.ParenRightFodder),
		Function: &Function{Params: make([]Param, len(node.Parameters))},
	}

	// The range for functions defined in objects isn't defined on the node itself, only the body
	if !res.Range.IsSet() && node.Body.Loc() != nil {
		res.Range = *node.Body.Loc()
	}

	_, res.Function.Return = UnwindLocals(node.Body)
	res.Function.ReturnType, _ = simpleToValueType(res.Function.Return)

	for i, param := range node.Parameters {
		var comments []string
		if i+1 == len(node.Parameters) {
			comments = foddersToComment(param.DefaultArg, param.NameFodder, param.EqFodder, node.ParenRightFodder)
		} else {
			comments = foddersToComment(param.DefaultArg, param.NameFodder, param.EqFodder, param.CommaFodder)
		}

		res.Function.Params[i] = Param{
			Name:    string(param.Name),
			Default: param.DefaultArg,
			Range:   param.LocRange,
			Comment: comments,
			Type:    commentsToType(comments),
		}
	}

	return res
}

func objectToValue(node *ast.DesugaredObject, resolver Resolver) *Value {
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
		fieldName := ""

		if nt, ok := fld.Name.(*ast.LiteralString); ok {
			fieldName = nt.Value
		} else if ov := NodeToValue(fld.Name, resolver); ov.StringValue != nil {
			fieldName = *ov.StringValue
		} else {
			logf("unknown fld name: %T %v", fld.Name, fld.Name)
			unknownFields = true
			continue
		}

		ft, _ := simpleToValueType(fld.Body)
		rng := fld.LocRange
		if fldfn, ok := fld.Body.(*ast.Function); ok && fldfn.Body.Loc() != nil {
			rng = *fldfn.Body.Loc()
		}

		res.Object.Fields = append(res.Object.Fields, Field{
			Name:    fieldName,
			Type:    ft,
			Comment: foddersToComment(fld.Body), // XXX: Name comments?
			Range:   rng,
			Node:    fld.Body,
			Hidden:  fld.Hide == ast.ObjectFieldHidden,
		})
		res.Object.FieldMap[fieldName] = &(res.Object.Fields[len(res.Object.Fields)-1])
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
	res := &Value{
		Node:    node,
		Comment: foddersToComment(node),
	}
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

var maxStackDepth = 300

func nodeToValue(node ast.Node, resolver Resolver, stackDepth int) (res *Value) {
	if stackDepth > maxStackDepth {
		return defaultToValue(node)
	}
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
			Type:        StringType,
			Node:        node,
			Range:       node.LocRange,
			Comment:     []string{node.Value},
			StringValue: &node.Value,
		}
	case *ast.LiteralNumber:
		return &Value{
			Type:    NumberType,
			Node:    node,
			Range:   node.LocRange,
			Comment: []string{node.OriginalString},
		}
	case *ast.LiteralBoolean:
		return &Value{
			Type:    BooleanType,
			Node:    node,
			Range:   node.LocRange,
			Comment: []string{strconv.FormatBool(node.Value)},
		}
	case *ast.Local:
		// ignore varbinds when getting the value
		return nodeToValue(node.Body, resolver, stackDepth + 1)
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
			return nodeToValue(v.Node, resolver, stackDepth + 1)
		}
		return defaultToValue(node)
	case *ast.Apply:
		targfn := nodeToValue(node.Target, resolver, stackDepth + 1)
		if targfn.Function == nil || targfn.Function.Return == nil {
			return defaultToValue(node)
		}
		return nodeToValue(targfn.Function.Return, resolver, stackDepth + 1)
	case *ast.Index:
		switch idx := node.Index.(type) {
		case *ast.LiteralNumber:
			// Number index of an array

			target := nodeToValue(node.Target, resolver, stackDepth + 1)
			idxInt, intErr := strconv.ParseInt(idx.OriginalString, 10, 64)
			targArr, _ := target.Node.(*ast.Array)

			if targArr == nil || intErr != nil || int(idxInt) >= len(targArr.Elements) {
				return defaultToValue(node)
			}

			return nodeToValue(targArr.Elements[idxInt].Expr, resolver, stackDepth + 1)
		case *ast.LiteralString:
			// String index of an object
			lhs := nodeToValue(node.Target, resolver, stackDepth + 1)

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
				return nodeToValue(lhs.Object.FieldMap[idx.Value].Node, resolver, stackDepth + 1)
			}
		}
		return defaultToValue(node)
	case *ast.Binary:
		if node.Op == ast.BopPlus {
			// object templates
			lhs, rhs := nodeToValue(node.Left, resolver, stackDepth + 1), nodeToValue(node.Right, resolver, stackDepth + 1)
			if lhs.Object != nil && rhs.Object != nil {
				return mergeObjectValues(lhs, rhs)
			}
			// resolve the addition of strings, which is a common operation that affects
			// lookup resolution
			if lhs.StringValue != nil && rhs.StringValue != nil {
				sval := *lhs.StringValue + *rhs.StringValue
				return &Value{
					Type:        StringType,
					Range:       node.LocRange,
					Node:        node,
					StringValue: &sval,
				}
			}
		}
		return defaultToValue(node)
	case *ast.DesugaredObject:
		return objectToValue(node, resolver)
	case *ast.Function:
		return functionToValue(node)
	case *ast.Import:
		return importToValue(node, resolver)
	default:
		return defaultToValue(node)
	}
}

func NodeToValue(node ast.Node, resolver Resolver) (res *Value) {
	return nodeToValue(node, resolver, 0)
}
