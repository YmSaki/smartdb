package ats

import "fmt"

type SQLCategory string

const (
	CategoryRead   SQLCategory = "read"
	CategoryEdit   SQLCategory = "edit"
	CategoryManage SQLCategory = "manage"
	CategoryAdmin  SQLCategory = "admin"
)

func ClassifySQL(input string) (SQLCategory, error) {
	l := New(input)

	// Collect all top-level statement-leading keywords (split by semicolons).
	var stmtKeywords []TokenType
	findingLeader := true

	for {
		tok := l.NextToken()
		if tok.Type == EOF {
			break
		}
		if tok.Type == SEMICOLON {
			findingLeader = true
			continue
		}
		if findingLeader {
			// ATTACH lets a key open any file the process can read/write —
			// including other projects' database.db or system.db — as a
			// schema it can then query/modify, bypassing project isolation
			// entirely. There's no safe subset of ATTACH to allow, so it's
			// rejected outright rather than merely categorized.
			if tok.Type == ATTACH {
				return "", fmt.Errorf("ATTACH is not permitted")
			}
			// VACUUM INTO writes a full copy of the database to an
			// arbitrary path (unlike bare VACUUM, which only rewrites the
			// current database file in place), making it an arbitrary-file
			// write primitive with the same isolation-bypass risk as ATTACH.
			if tok.Type == VACUUM && hasIntoClause(l) {
				return "", fmt.Errorf("VACUUM INTO is not permitted")
			}
			stmtKeywords = append(stmtKeywords, tok.Type)
			if tok.Type == WITH {
				// WITH is a CTE prefix; the real statement keyword follows.
				// Skip until we find the actual DML keyword after WITH.
				stmtKeywords = append(stmtKeywords, resolveWithBody(l)...)
			}
			findingLeader = false
		}
	}

	if len(stmtKeywords) == 0 {
		return "", fmt.Errorf("empty query")
	}

	// Highest privilege required across all statements wins.
	highest := CategoryRead
	for _, kw := range stmtKeywords {
		cat := categorizeKeyword(kw)
		if priority(cat) > priority(highest) {
			highest = cat
		}
	}
	return highest, nil
}

// resolveWithBody scans past CTE definitions to find the actual DML keyword.
func resolveWithBody(l *Lexer) []TokenType {
	depth := 0
	for {
		tok := l.NextToken()
		if tok.Type == EOF {
			return nil
		}
		if tok.Literal == "(" {
			depth++
			continue
		}
		if tok.Literal == ")" {
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth > 0 {
			continue
		}
		// At top level after WITH, look for the real DML keyword
		switch tok.Type {
		case SELECT, INSERT, UPDATE, DELETE, REPLACE:
			return []TokenType{tok.Type}
		}
	}
}

// hasIntoClause reports whether a VACUUM statement includes an INTO
// clause: `VACUUM [schema-name] INTO filename`. It looks ahead at most two
// tokens (the optional schema-name, then INTO) and always pushes what it
// consumed back onto the lexer, so callers can keep scanning normally
// regardless of the result.
//
// schema-name may be a bare identifier (WORD), a quoted identifier
// (IDENT — "name", `name`, or [name]), or — per SQLite's documented
// single-quoted-string-as-identifier fallback, confirmed against a real
// SQLite engine — a STRING ('name'). All three are equally valid ways to
// spell the same schema-name and must be treated alike here; missing any
// one of them lets `VACUUM <quoted-form> INTO 'path'` slip past as if it
// were a harmless bare VACUUM.
func hasIntoClause(l *Lexer) bool {
	first := l.NextToken()
	if first.Type == INTO {
		l.unread(first)
		return true
	}
	if first.Type != WORD && first.Type != IDENT && first.Type != STRING {
		l.unread(first)
		return false
	}
	second := l.NextToken()
	l.unread(second)
	l.unread(first)
	return second.Type == INTO
}

func categorizeKeyword(tt TokenType) SQLCategory {
	switch tt {
	case SELECT, WITH:
		return CategoryRead
	case INSERT, UPDATE, DELETE, REPLACE:
		return CategoryEdit
	case PRAGMA, ANALYZE, REINDEX, EXPLAIN:
		return CategoryManage
	case CREATE, DROP, ALTER, ATTACH:
		return CategoryAdmin
	default:
		return CategoryAdmin
	}
}

func priority(c SQLCategory) int {
	switch c {
	case CategoryRead:
		return 0
	case CategoryEdit:
		return 1
	case CategoryManage:
		return 2
	case CategoryAdmin:
		return 3
	default:
		return 3
	}
}
