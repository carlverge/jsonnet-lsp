package annotation

import (
	"bytes"
	"fmt"
	"io"
)

// parser represents a parser.
type parser struct {
	s   *scanner
	buf struct {
		tok Token  // last read token
		lit string // last read literal
		n   int    // buffer size (max=1)
	}
}

// newParser returns a new instance of Parser.
func newParser(r io.Reader) *parser {
	return &parser{s: newScanner(r)}
}

// scan returns the next token from the underlying scanner.
// If a token has been unscanned then read that instead.
func (p *parser) scanWithWhitespace() (tok Token, lit string) {
	// If we have a token on the buffer, then return it.
	if p.buf.n != 0 {
		p.buf.n = 0
		return p.buf.tok, p.buf.lit
	}

	// Otherwise read the next token from the scanner.
	tok, lit = p.s.Scan()

	// Save it to the buffer in case we unscan later.
	p.buf.tok, p.buf.lit = tok, lit

	return
}

// errorf panics -- the parser does not return errors
// the top level caller calling parse needs to recover
func (p *parser) errorf(msg string, a ...interface{}) {
	panic(fmt.Errorf(msg, a...))
}

// consume token tok and return value or panic
func (p *parser) consume(tok Token) string {
	got, lit := p.scan()
	if got != tok {
		p.errorf("expected token %v but got %v", tok, got)
	}
	return lit
}

// scanIgnoreWhitespace scans the next non-whitespace token.
func (p *parser) scan() (tok Token, lit string) {
	tok, lit = p.scanWithWhitespace()
	if tok == SPACE {
		tok, lit = p.scanWithWhitespace()
	}
	return
}

// unscan pushes the previously read token back onto the buffer.
func (p *parser) unscan() {
	p.buf.n = 1
}

// peek returns the next token from the underlying scanner without advancing.
func (p *parser) peek() (tok Token, lit string) {
	tok, lit = p.scan()
	p.unscan()
	return
}

func (p *parser) peekToken() Token {
	tok, _ := p.scan()
	p.unscan()
	return tok
}

// Parse starts the parsing of the type declaration and returns AST and possible error
func (p *parser) Parse() (node Node, err error) {
	defer func() {
		if v := recover(); v != nil {
			err = v.(error)
		}
	}()
	return p.parseTypeHint(), nil
}

func (p *parser) parseTypeHint() Node {
	node := p.parseTypeHintNoUnion()

	if p.peekToken() != UNION {
		return node
	}

	union := &UnionNode{Types: []Node{node}}
	for {
		if p.peekToken() != UNION {
			return union
		}
		_ = p.consume(UNION)
		union.Types = append(union.Types, p.parseTypeHintNoUnion())
	}
}

func (p *parser) parseDottedIdent(start string) Node {
	_ = p.consume(DOT)
	res := &DottedIdentNode{Names: []string{start}}

	for {
		res.Names = append(res.Names, p.consume(IDENT))
		switch p.peekToken() {
		case DOT:
			_ = p.consume(DOT)
			continue
		default:
			return res
		}
	}
}

func isLitTypeParam(lit string) bool {
	return len(lit) == 1 && lit[0] >= 'A' && lit[0] <= 'Z'
}

func (p *parser) parseTypeHintNoUnion() Node {
	tok, lit := p.scan()

	switch tok {
	case IDENT:
		if isLitTypeParam(lit) {
			return &TypeParameterNode{Name: lit}
		}
		switch lit {
		case "string":
			return constString
		case "boolean":
			return constBoolean
		case "number":
			return constNumber
		case "null":
			return constNull
		}

		if p.peekToken() == DOT {
			return p.parseDottedIdent(lit)
		}

		return &IdentNode{Name: lit}
	case ARRAY:
		next, _ := p.peek()
		switch next {
		case BRACKET_OPEN:
			_ = p.consume(BRACKET_OPEN)
			res := &ArrayNode{ElementType: p.parseTypeHint()}
			_ = p.consume(BRACKET_CLOSE)
			return res
		default:
			return &ArrayNode{}
		}
	case BRACE_OPEN:
		return &ObjectNode{Fields: p.parseObjectParams()}
	case OBJECT:
		switch p.peekToken() {
		case BRACKET_OPEN:
			_ = p.consume(BRACKET_OPEN)
			res := &ObjectNode{ElementType: p.parseTypeHint()}
			_ = p.consume(BRACKET_CLOSE)
			return res
		default:
			return &ObjectNode{}
		}
	case FUNCTION:
		res := &FunctionNode{}
		if p.peekToken() != PAREN_OPEN {
			return res
		}

		res.Params = p.parseFunctionParams()
		if p.peekToken() == ARROW {
			_ = p.consume(ARROW)
			res.Return = p.parseTypeHint()
		}
		return res
	default:
		p.errorf("unexpected token: '%s'", tok)
		panic("unreachable")
	}
}

func (p *parser) parseObjectParams() []ParamNode {
	params := []ParamNode{}
	for {
		// object params must always have a type
		name := p.consume(IDENT)
		_ = p.consume(COLON)
		params = append(params, ParamNode{
			Name: name,
			Type: p.parseTypeHint(),
		})

		switch p.peekToken() {
		case COMMA:
			_ = p.consume(COMMA)
			continue
		default:
			_ = p.consume(BRACE_CLOSE)
			return params
		}
	}
}

func (p *parser) parseFunctionParams() []ParamNode {
	params := []ParamNode{}
	_ = p.consume(PAREN_OPEN)
	for {
		name := p.consume(IDENT)

		// function params dont always have a type
		if p.peekToken() == COLON {
			_ = p.consume(COLON)
			params = append(params, ParamNode{
				Name: name,
				Type: p.parseTypeHint(),
			})
		} else {
			params = append(params, ParamNode{Name: name})
		}

		switch p.peekToken() {
		case COMMA:
			_ = p.consume(COMMA)
			continue
		default:
			_ = p.consume(PAREN_CLOSE)
			return params
		}
	}
}

func Parse(text string) (Node, error) {
	// short circuit common cases to avoid doing full parsing
	if isLitTypeParam(text) {
		return &TypeParameterNode{Name: text}, nil
	}
	switch text {
	case "string":
		return constString, nil
	case "boolean":
		return constBoolean, nil
	case "number":
		return constNumber, nil
	case "null":
		return constNull, nil
	case "function":
		return constFunction, nil
	case "object":
		return constObject, nil
	case "array":
		return constArray, nil
	case "array[string]":
		return &ArrayNode{ElementType: constString}, nil
	case "array[number]":
		return &ArrayNode{ElementType: constNumber}, nil
	case "array[boolean]":
		return &ArrayNode{ElementType: constBoolean}, nil
	case "array[E]":
		return &ArrayNode{ElementType: &TypeParameterNode{Name: "E"}}, nil
	case "array[T]":
		return &ArrayNode{ElementType: &TypeParameterNode{Name: "T"}}, nil
	case "array[A]":
		return &ArrayNode{ElementType: &TypeParameterNode{Name: "A"}}, nil
	default:
		p := newParser(bytes.NewBufferString(text))
		return p.Parse()
	}
}
