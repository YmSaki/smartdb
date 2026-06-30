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
