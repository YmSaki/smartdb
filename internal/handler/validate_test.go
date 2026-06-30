package handler

import (
	"testing"
)

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		shouldErr bool
		reason    string
	}{
		{
			name:      "valid lowercase name",
			input:     "myproject",
			shouldErr: false,
		},
		{
			name:      "valid name with numbers",
			input:     "project123",
			shouldErr: false,
		},
		{
			name:      "valid name with underscore",
			input:     "my_project",
			shouldErr: false,
		},
		{
			name:      "valid name with hyphen",
			input:     "my-project",
			shouldErr: false,
		},
		{
			name:      "valid name with mixed valid chars",
			input:     "proj-123_abc",
			shouldErr: false,
		},
		{
			name:      "invalid empty name",
			input:     "",
			shouldErr: true,
			reason:    "empty name",
		},
		{
			name:      "invalid uppercase letters",
			input:     "MyProject",
			shouldErr: true,
			reason:    "uppercase not allowed",
		},
		{
			name:      "invalid spaces",
			input:     "my project",
			shouldErr: true,
			reason:    "spaces not allowed",
		},
		{
			name:      "invalid special characters",
			input:     "my@project",
			shouldErr: true,
			reason:    "special chars not allowed",
		},
		{
			name:      "path traversal attempt with ../",
			input:     "../etc/passwd",
			shouldErr: true,
			reason:    "path traversal protection",
		},
		{
			name:      "path traversal attempt with ..",
			input:     "proj..ect",
			shouldErr: true,
			reason:    "path traversal protection",
		},
		{
			name:      "unicode characters",
			input:     "プロジェクト",
			shouldErr: true,
			reason:    "unicode not allowed",
		},
		{
			name:      "name too long (exceeds 255 chars)",
			input:     string(make([]byte, 256)),
			shouldErr: true,
			reason:    "exceeds max length",
		},
		{
			name:      "name starts with number",
			input:     "1project",
			shouldErr: false,
		},
		{
			name:      "name is single character",
			input:     "a",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProjectName(tt.input)

			if tt.shouldErr && err == nil {
				t.Errorf("expected error for %q (%s), but got nil", tt.input, tt.reason)
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.input, err)
			}
		})
	}
}

func TestValidateProjectID(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		shouldErr bool
		reason    string
	}{
		{
			name:      "valid project ID",
			input:     "myproject-abc123def",
			shouldErr: false,
		},
		{
			name:      "valid project ID with lowercase",
			input:     "project-id",
			shouldErr: false,
		},
		{
			name:      "invalid empty ID",
			input:     "",
			shouldErr: true,
			reason:    "empty ID",
		},
		{
			name:      "invalid ID with uppercase",
			input:     "MyProject-ID",
			shouldErr: true,
			reason:    "uppercase not allowed",
		},
		{
			name:      "invalid ID with spaces",
			input:     "my project",
			shouldErr: true,
			reason:    "spaces not allowed",
		},
		{
			name:      "path traversal in ID",
			input:     "../../etc/passwd",
			shouldErr: true,
			reason:    "path traversal attempt",
		},
		{
			name:      "path traversal with dot-slash",
			input:     "./project",
			shouldErr: true,
			reason:    "path traversal protection",
		},
		{
			name:      "ID with null byte",
			input:     "project\x00id",
			shouldErr: true,
			reason:    "null byte not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProjectID(tt.input)

			if tt.shouldErr && err == nil {
				t.Errorf("expected error for %q (%s), but got nil", tt.input, tt.reason)
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.input, err)
			}
		})
	}
}
