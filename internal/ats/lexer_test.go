package ats

import "testing"

func collectTypes(t *testing.T, input string) []TokenType {
	t.Helper()
	l := New(input)
	var got []TokenType
	for i := 0; i < 1000; i++ {
		tok := l.NextToken()
		got = append(got, tok.Type)
		if tok.Type == EOF {
			return got
		}
	}
	t.Fatalf("too many tokens, input=%q", input)
	return nil
}

func TestNextTokenTypes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []TokenType
	}{
		{
			name:  "semicolon splits from word",
			input: "SELECT 1;",
			want:  []TokenType{SELECT, WORD, SEMICOLON, EOF},
		},
		{
			name:  "stacked query exposes keyword",
			input: "a;DROP TABLE t",
			want:  []TokenType{WORD, SEMICOLON, DROP, WORD, WORD, EOF},
		},
		{
			name:  "lowercase keywords and symbols",
			input: "select * from t",
			want:  []TokenType{SELECT, SYMBOL, WORD, WORD, EOF},
		},
		{
			name:  "WITH keyword",
			input: "WITH cte AS (SELECT 1) SELECT * FROM cte",
			want:  []TokenType{WITH, WORD, WORD, SYMBOL, SELECT, WORD, SYMBOL, SELECT, SYMBOL, WORD, WORD, EOF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectTypes(t, tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("input=%q\n got =%v\n want=%v", tt.input, got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("input=%q pos %d: got=%q want=%q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSelectedIsWord(t *testing.T) {
	l := New("SELECTED")
	tok := l.NextToken()

	if tok.Type != WORD {
		t.Errorf("SELECTED type: got=%q want=WORD", tok.Type)
	}
	if tok.Literal != "SELECTED" {
		t.Errorf("SELECTED literal: got=%q want=SELECTED", tok.Literal)
	}
}

func TestStringLiteralDoesNotLeakKeyword(t *testing.T) {
	got := collectTypes(t, "x = 'a delete b'")
	for _, ty := range got {
		if ty == DELETE {
			t.Fatalf("DELETE inside string literal was misidentified: %v", got)
		}
	}
}

func TestSkipSpaceMatchesSQLiteWhitespaceSet(t *testing.T) {
	// sqlite3Isspace treats 0x09-0x0D (\t \n \v \f \r) and 0x20 (space) as
	// insignificant whitespace. Any gap here lets that byte surface as a
	// stray SYMBOL/ILLEGAL token instead of being skipped.
	for _, ws := range []byte{'\t', '\n', '\v', '\f', '\r', ' '} {
		l := New(string(ws) + "SELECT")
		tok := l.NextToken()
		if tok.Type != SELECT {
			t.Errorf("byte %#x before SELECT: got type=%q, want SELECT", ws, tok.Type)
		}
	}
}

func TestQuotedIdentifiersRecognized(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		literal string
	}{
		{"double-quoted", `"main"`, `"main"`},
		{"backtick-quoted", "`main`", "`main`"},
		{"bracket-quoted", "[main]", "[main]"},
		{"double-quoted with doubled-quote escape", `"ma""in"`, `"ma""in"`},
		{"backtick-quoted with doubled-backtick escape", "`ma``in`", "`ma``in`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()
			if tok.Type != IDENT {
				t.Fatalf("type: got=%q want=IDENT", tok.Type)
			}
			if tok.Literal != tt.literal {
				t.Errorf("literal: got=%q want=%q", tok.Literal, tt.literal)
			}
			if next := l.NextToken(); next.Type != EOF {
				t.Errorf("expected EOF after identifier, got %q", next.Type)
			}
		})
	}
}

func TestVacuumIntoKeywordsRecognized(t *testing.T) {
	got := collectTypes(t, "VACUUM INTO 'x.db'")
	want := []TokenType{VACUUM, INTO, STRING, EOF}
	if len(got) != len(want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pos %d: got=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestUnreadReplaysInOrder(t *testing.T) {
	l := New("A B C")
	first := l.NextToken()  // A
	second := l.NextToken() // B
	l.unread(second)
	l.unread(first)

	if tok := l.NextToken(); tok.Literal != "A" {
		t.Fatalf("first replayed token: got=%q want=A", tok.Literal)
	}
	if tok := l.NextToken(); tok.Literal != "B" {
		t.Fatalf("second replayed token: got=%q want=B", tok.Literal)
	}
	if tok := l.NextToken(); tok.Literal != "C" {
		t.Fatalf("token after replay resumes from lexer: got=%q want=C", tok.Literal)
	}
}

func TestStringEscapedQuote(t *testing.T) {
	l := New("'it''s ok'")
	tok := l.NextToken()
	if tok.Type != STRING {
		t.Errorf("type: got=%q want=STRING", tok.Type)
	}
	if tok.Literal != "'it''s ok'" {
		t.Errorf("literal: got=%q want='it''s ok'", tok.Literal)
	}
}
