package annotation

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// TokenType represents the type of a token.
type Token int

const (
	ILLEGAL Token = iota
	EOF
	SPACE // whitespace
	IDENT // main, foo, bar, x, y, etc.
	// operators and delimiters
	DOT           // .
	COMMA         // ,
	BRACE_OPEN    // {
	BRACE_CLOSE   // }
	BRACKET_OPEN  // [
	BRACKET_CLOSE // ]
	PAREN_OPEN    // (
	PAREN_CLOSE   // )
	COLON         // :
	ARROW         // ->
	UNION         // |
	// keywords
	ARRAY
	OBJECT
	FUNCTION
)

func (t Token) String() string {
	switch t {
	case ILLEGAL:
		return "ILLEGAL"
	case EOF:
		return "EOF"
	case SPACE:
		return "WS"
	case IDENT:
		return "IDENT"
	case DOT:
		return "DOT"
	case COMMA:
		return "COMMA"
	case BRACE_OPEN:
		return "BRACE_OPEN"
	case BRACE_CLOSE:
		return "BRACE_CLOSE"
	case BRACKET_OPEN:
		return "BRACKET_OPEN"
	case BRACKET_CLOSE:
		return "BRACKET_CLOSE"
	case PAREN_OPEN:
		return "PAREN_OPEN"
	case PAREN_CLOSE:
		return "PAREN_CLOSE"
	case COLON:
		return "COLON"
	case ARROW:
		return "ARROW"
	case UNION:
		return "UNION"
	case ARRAY:
		return "ARRAY"
	case OBJECT:
		return "OBJECT"
	case FUNCTION:
		return "FUNCTION"
	default:
		return fmt.Sprintf("UNKNOWN_TOKEN_TYPE_%d", int(t))
	}
}

const eof = rune(0)

func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n'
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch rune) bool {
	return (ch >= '0' && ch <= '9')
}

func isIdentChar(ch rune) bool {
	// Valid: [0-9,a-z,A-Z$_]
	return isLetter(ch) || isDigit(ch) || ch == '_' || ch == '$'
}

type scanner struct {
	r *bufio.Reader
}

func newScanner(r io.Reader) *scanner {
	return &scanner{r: bufio.NewReader(r)}
}

// read reads the next rune from the bufferred reader.
// Returns the rune(0) if an error occurs (or io.EOF is returned).
func (s *scanner) read() rune {
	ch, _, err := s.r.ReadRune()
	if err != nil {
		return eof
	}
	return ch
}

func (s *scanner) unread() { _ = s.r.UnreadRune() }

// scanWhitespace consumes the current rune and all contiguous whitespace.
func (s *scanner) scanWhitespace() (tok Token, lit string) {
	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every subsequent whitespace character into the buffer.
	// Non-whitespace characters and EOF will cause the loop to exit.
	for {
		if ch := s.read(); ch == eof {
			break
		} else if !isWhitespace(ch) {
			s.unread()
			break
		} else {
			buf.WriteRune(ch)
		}
	}

	return SPACE, buf.String()
}

// Scan returns the next token and literal value.
func (s *scanner) Scan() (tok Token, lit string) {
	// Read the next rune.
	ch := s.read()

	// If we see whitespace then consume all contiguous whitespace.
	// If we see a letter then consume as an ident or reserved word.
	if isWhitespace(ch) {
		s.unread()
		return s.scanWhitespace()
	} else if isIdentChar(ch) {
		s.unread()
		return s.scanIdent()
	}

	// Otherwise read the individual character.
	switch ch {
	case eof:
		return EOF, ""
	case '(':
		return PAREN_OPEN, "("
	case ')':
		return PAREN_CLOSE, ")"
	case '{':
		return BRACE_OPEN, "{"
	case '}':
		return BRACE_CLOSE, "}"
	case '[':
		return BRACKET_OPEN, "["
	case ']':
		return BRACKET_CLOSE, "]"
	case ',':
		return COMMA, ","
	case '.':
		return DOT, "."
	case ':':
		return COLON, ":"
	case '|':
		return UNION, "|"
	case '-':
		next := s.read()
		switch next {
		case '>':
			return ARROW, "->"
		default:
			s.unread()
			return ILLEGAL, string(ch)
		}
	}

	return ILLEGAL, string(ch)
}

// scanIdent consumes the current rune and all contiguous ident runes.
func (s *scanner) scanIdent() (tok Token, lit string) {
	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every subsequent ident character into the buffer.
	// Non-ident characters and EOF will cause the loop to exit.
	for {
		if ch := s.read(); ch == eof {
			break
		} else if !isIdentChar(ch) {
			s.unread()
			break
		} else {
			_, _ = buf.WriteRune(ch)
		}
	}

	// If the string matches a keyword then return that keyword.
	switch buf.String() {
	case "object":
		return OBJECT, "object"
	case "function":
		return FUNCTION, "function"
	case "array":
		return ARRAY, "array"
	default:
		return IDENT, buf.String()
	}
}
