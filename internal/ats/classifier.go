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
