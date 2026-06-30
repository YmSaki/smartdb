package auth

import (
	"regexp"
	"strings"
	"testing"
)

func TestGenerateToken(t *testing.T) {
	tests := []struct {
		name        string
		checkEmpty  bool
		checkPrefix bool
		checkLen    bool
	}{
		{
			name:        "generated token is non-empty",
			checkEmpty:  true,
			checkPrefix: false,
			checkLen:    false,
		},
		{
			name:        "generated token has sdb_ prefix",
			checkEmpty:  false,
			checkPrefix: true,
			checkLen:    false,
		},
		{
			name:        "generated token has sufficient length",
			checkEmpty:  false,
			checkPrefix: false,
			checkLen:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, _ := GenerateToken()

			if tt.checkEmpty {
				if token == "" {
					t.Error("token is empty")
				}
			}

			if tt.checkPrefix {
				if !strings.HasPrefix(token, "sdb_") {
					t.Errorf("token does not start with sdb_ prefix: %q", token)
				}
			}

			if tt.checkLen {
				// Token should be at least 40 chars (4 for prefix + 32+ for random)
				if len(token) < 40 {
					t.Errorf("token too short: got %d chars, want at least 40", len(token))
				}
			}
		})
	}
}

func TestGenerateTokenUnique(t *testing.T) {
	tokens := make(map[string]bool)

	for i := 0; i < 100; i++ {
		token, _ := GenerateToken()

		if tokens[token] {
			t.Fatalf("duplicate token generated: %s", token)
		}
		tokens[token] = true
	}

	if len(tokens) != 100 {
		t.Errorf("expected 100 unique tokens, got %d", len(tokens))
	}
}

func TestHashToken(t *testing.T) {
	token := "sdb_test_token_12345678"

	tests := []struct {
		name            string
		token           string
		checkDeterministic bool
		checkNotEmpty   bool
	}{
		{
			name:               "hash is deterministic",
			token:              token,
			checkDeterministic: true,
		},
		{
			name:           "hash is non-empty",
			token:          token,
			checkNotEmpty:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := HashToken(tt.token)

			if tt.checkNotEmpty {
				if hash1 == "" {
					t.Error("hash is empty")
				}
			}

			if tt.checkDeterministic {
				hash2 := HashToken(tt.token)

				if hash1 != hash2 {
					t.Errorf("hash is not deterministic: got %q, then %q", hash1, hash2)
				}
			}
		})
	}
}

func TestHashTokenLength(t *testing.T) {
	tokens := []string{
		"sdb_token_1",
		"sdb_token_2",
		"different_token",
		"another_test_token",
	}

	for _, token := range tokens {
		t.Run(("hash length for "+token), func(t *testing.T) {
			hash := HashToken(token)

			// SHA-256 produces 64 hex characters
			if len(hash) != 64 {
				t.Errorf("hash length: got %d, want 64", len(hash))
			}

			// Verify it's valid hex
			matched, err := regexp.MatchString("^[0-9a-f]{64}$", hash)
			if err != nil {
				t.Fatalf("regex error: %v", err)
			}

			if !matched {
				t.Errorf("hash is not valid hex (64 chars): got %q", hash)
			}
		})
	}
}

func TestHashTokenDifferent(t *testing.T) {
	token1 := "sdb_token_1"
	token2 := "sdb_token_2"

	hash1 := HashToken(token1)
	hash2 := HashToken(token2)

	if hash1 == hash2 {
		t.Errorf("different tokens produced same hash: %q == %q", hash1, hash2)
	}
}
