package handler

import (
	"fmt"
	"regexp"
	"strings"
)

var projectNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// ValidateProjectName validates a project name against the pattern ^[a-z0-9][a-z0-9_-]*$.
// Names must be lowercase, start with alphanumeric, and contain only alphanumeric, underscore, or hyphen.
// Maximum length is 255 characters.
func ValidateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name must not be empty")
	}
	if len(name) > 255 {
		return fmt.Errorf("project name must not exceed 255 characters")
	}
	if !projectNameRe.MatchString(name) {
		return fmt.Errorf("project name must match ^[a-z0-9][a-z0-9_-]*$ (lowercase only)")
	}
	return nil
}

var projectIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// ValidateProjectID validates a project ID for path safety.
// Rejects empty IDs, null bytes, path traversal sequences, and non-lowercase characters.
func ValidateProjectID(id string) error {
	if id == "" {
		return fmt.Errorf("project ID must not be empty")
	}
	if strings.ContainsRune(id, 0) {
		return fmt.Errorf("invalid project ID: contains null byte")
	}
	if strings.Contains(id, "..") {
		return fmt.Errorf("invalid project ID: path traversal detected")
	}
	if !projectIDRe.MatchString(id) {
		return fmt.Errorf("invalid project ID: must match ^[a-z0-9][a-z0-9_-]*$")
	}
	return nil
}
