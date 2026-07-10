package ats

import "strings"

type TokenType string

const (
	EOF     TokenType = "EOF"
	ILLEGAL TokenType = "ILLEGAL"

	WORD   TokenType = "WORD"
	STRING TokenType = "STRING"

	ALTER   TokenType = "ALTER"
	ANALYZE TokenType = "ANALYZE"
	ATTACH  TokenType = "ATTACH"
	CREATE  TokenType = "CREATE"
	DELETE  TokenType = "DELETE"
	DROP    TokenType = "DROP"
	EXPLAIN TokenType = "EXPLAIN"
	INSERT  TokenType = "INSERT"
	INTO    TokenType = "INTO"
	PRAGMA  TokenType = "PRAGMA"
	REINDEX TokenType = "REINDEX"
	REPLACE TokenType = "REPLACE"
	SELECT  TokenType = "SELECT"
	UPDATE  TokenType = "UPDATE"
	VACUUM  TokenType = "VACUUM"
	VALUES  TokenType = "VALUES"
	WITH    TokenType = "WITH"

	SEMICOLON TokenType = ";"

	SYMBOL TokenType = "SYMBOL"
)

var keywords = map[string]TokenType{
	"ALTER":   ALTER,
	"ANALYZE": ANALYZE,
	"ATTACH":  ATTACH,
	"CREATE":  CREATE,
	"DELETE":  DELETE,
	"DROP":    DROP,
	"EXPLAIN": EXPLAIN,
	"INSERT":  INSERT,
	"INTO":    INTO,
	"PRAGMA":  PRAGMA,
	"REINDEX": REINDEX,
	"REPLACE": REPLACE,
	"SELECT":  SELECT,
	"UPDATE":  UPDATE,
	"VACUUM":  VACUUM,
	"VALUES":  VALUES,
	"WITH":    WITH,
}

type Token struct {
	Type    TokenType
	Literal string
}

type Lexer struct {
	input string
	pos   int
	buf   []Token
}

func New(input string) *Lexer {
	return &Lexer{input: input, pos: 0}
}

// unread pushes tok back onto the lexer so the next NextToken call
// returns it again. Tokens are replayed LIFO, so a caller that reads
// several tokens ahead and wants to put them all back must unread them
// in reverse of the order it read them.
func (l *Lexer) unread(tok Token) {
	l.buf = append(l.buf, tok)
}

func (l *Lexer) NextToken() Token {
	if n := len(l.buf); n > 0 {
		tok := l.buf[n-1]
		l.buf = l.buf[:n-1]
		return tok
	}

	l.skipSpace()
	if l.pos >= len(l.input) {
		return Token{Type: EOF}
	}

	ch := l.input[l.pos]

	switch {
	case ch == ';':
		l.pos++
		return Token{Type: SEMICOLON, Literal: ";"}
	case ch == '\'':
		return l.readString()
	case isWordChar(ch):
		word := l.readWord()
		return classify(word)
	default:
		l.pos++
		return Token{Type: SYMBOL, Literal: string(ch)}
	}
}

func (l *Lexer) readString() Token {
	l.pos++ // skip opening quote
	var buf strings.Builder
	buf.WriteByte('\'')
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		l.pos++
		buf.WriteByte(ch)
		if ch == '\'' {
			if l.pos < len(l.input) && l.input[l.pos] == '\'' {
				buf.WriteByte('\'')
				l.pos++
				continue
			}
			return Token{Type: STRING, Literal: buf.String()}
		}
	}
	return Token{Type: STRING, Literal: buf.String()}
}

func (l *Lexer) readWord() string {
	start := l.pos
	for l.pos < len(l.input) && isWordChar(l.input[l.pos]) {
		l.pos++
	}
	return l.input[start:l.pos]
}

func classify(word string) Token {
	upper := strings.ToUpper(word)
	if tt, ok := keywords[upper]; ok {
		return Token{Type: tt, Literal: word}
	}
	return Token{Type: WORD, Literal: word}
}

func isWordChar(ch byte) bool {
	return 'a' <= ch && ch <= 'z' ||
		'A' <= ch && ch <= 'Z' ||
		'0' <= ch && ch <= '9' ||
		ch == '_'
}

func (l *Lexer) skipSpace() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			l.pos++
			continue
		}
		if l.pos+1 < len(l.input) && ch == '-' && l.input[l.pos+1] == '-' {
			l.pos += 2
			for l.pos < len(l.input) && l.input[l.pos] != '\n' {
				l.pos++
			}
			continue
		}
		if l.pos+1 < len(l.input) && ch == '/' && l.input[l.pos+1] == '*' {
			l.pos += 2
			for l.pos+1 < len(l.input) {
				if l.input[l.pos] == '*' && l.input[l.pos+1] == '/' {
					l.pos += 2
					break
				}
				l.pos++
			}
			continue
		}
		break
	}
}
