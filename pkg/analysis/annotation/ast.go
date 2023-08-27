package annotation

import (
	"fmt"
	"strings"
)

type Node interface {
	isASTNode()
	String() string
	TypeParameters() []string
}

type rootNode struct{}

func (*rootNode) isASTNode()               {}
func (*rootNode) TypeParameters() []string { return nil }

type TypeParameterNode struct {
	rootNode
	Name string
}

func (n *TypeParameterNode) String() string { return n.Name }

type StringNode struct{ rootNode }

var constString = &StringNode{}

func (n *StringNode) String() string { return "string" }

type NumberNode struct{ rootNode }

var constNumber = &NumberNode{}

func (n *NumberNode) String() string { return "number" }

type BooleanNode struct{ rootNode }

var constBoolean = &BooleanNode{}

func (n *BooleanNode) String() string { return "boolean" }

type NullNode struct{ rootNode }

var constNull = &NullNode{}

func (n *NullNode) String() string { return "null" }

type IdentNode struct {
	rootNode
	Name string
}

func (n *IdentNode) String() string { return n.Name }

type DottedIdentNode struct {
	rootNode
	Names []string
}

func (n *DottedIdentNode) String() string {
	return strings.Join(n.Names, ".")
}

type ParamNode struct {
	rootNode
	Name string
	Type Node
}

func (n *ParamNode) String() string {
	if n.Type == nil {
		return n.Name
	}
	return fmt.Sprintf("%s: %s", n.Name, n.Type)
}

func (n *ParamNode) TypeParameters() []string {
	if n.Type == nil {
		return nil
	}
	return n.Type.TypeParameters()
}

type FunctionNode struct {
	rootNode
	Params []ParamNode
	Return Node
}

var constFunction = &FunctionNode{}

func (n *FunctionNode) String() string {
	if n.Params == nil && n.Return == nil {
		return "function"
	}
	paramStrings := []string{}
	for _, param := range n.Params {
		paramStrings = append(paramStrings, param.String())
	}
	ret := ")"
	if n.Return != nil {
		ret = ") -> " + n.Return.String()
	}
	return "function(" + strings.Join(paramStrings, ", ") + ret
}

func (n *FunctionNode) TypeParameters() []string {
	var res []string
	for _, p := range n.Params {
		res = appendDedup(res, p.TypeParameters())
	}
	if n.Return != nil {
		res = appendDedup(res, n.Return.TypeParameters())
	}
	return res
}

type UnionNode struct {
	rootNode
	Types []Node
}

func (n *UnionNode) String() string {
	typeStrings := []string{}
	for _, t := range n.Types {
		typeStrings = append(typeStrings, t.String())
	}
	return strings.Join(typeStrings, " | ")
}

func (n *UnionNode) TypeParameters() []string {
	var res []string
	for _, p := range n.Types {
		res = appendDedup(res, p.TypeParameters())
	}
	return res
}

type ArrayNode struct {
	rootNode
	ElementType Node
}

var constArray = &ArrayNode{}

func (n *ArrayNode) String() string {
	if n.ElementType == nil {
		return "array"
	}
	return fmt.Sprintf("array[%s]", n.ElementType)
}

func (n *ArrayNode) TypeParameters() []string {
	if n.ElementType == nil {
		return nil
	}
	return n.ElementType.TypeParameters()
}

type ObjectNode struct {
	rootNode
	ElementType Node
	Fields      []ParamNode
}

var constObject = &ObjectNode{}

func (n *ObjectNode) String() string {
	if n.ElementType == nil && n.Fields == nil {
		return "object"
	}
	if n.ElementType != nil {
		return fmt.Sprintf("object[%s]", n.ElementType)
	}
	fieldStrings := []string{}
	for _, field := range n.Fields {
		fieldStrings = append(fieldStrings, field.String())
	}
	return "{" + strings.Join(fieldStrings, ", ") + "}"
}

func (n *ObjectNode) TypeParameters() []string {
	if n.ElementType != nil {
		return n.ElementType.TypeParameters()
	}
	if n.Fields == nil {
		return nil
	}

	var res []string
	for _, p := range n.Fields {
		res = appendDedup(res, p.TypeParameters())
	}
	return res
}

func elemIn(a []string, elem string) bool {
	for _, x := range a {
		if elem == x {
			return true
		}
	}
	return false
}

// This is O(N^2), but given the low numbers of type parameters I'm considering
// this preferable to using maps and continuously allocating and throwing them away
func appendDedup(a, b []string) []string {
	if b == nil {
		return a
	}
	if a == nil {
		return b
	}
	res := a
	for _, elem := range b {
		if elemIn(a, elem) {
			continue
		}
		res = append(res, elem)
	}
	return res
}
