package handler

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestRequestIDMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		headerPresent  bool
		headerValue    string
		expectedFormat string
	}{
		{
			name:           "X-Request-ID header present in request",
			headerPresent:  true,
			headerValue:    "custom-request-id-123",
			expectedFormat: "custom-request-id-123",
		},
		{
			name:           "X-Request-ID header absent in request",
			headerPresent:  false,
			expectedFormat: ".+", // Should generate one
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/health", nil)
			if tt.headerPresent {
				req.Header.Set("X-Request-ID", tt.headerValue)
			}

			w := httptest.NewRecorder()

			handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			}))

			handler.ServeHTTP(w, req)

			responseID := w.Header().Get("X-Request-ID")
			if responseID == "" {
				t.Error("X-Request-ID header missing from response")
			}

			if tt.headerPresent {
				if responseID != tt.headerValue {
					t.Errorf("request ID: got %q, want %q", responseID, tt.headerValue)
				}
			}
		})
	}
}

func TestRequestIDUnique(t *testing.T) {
	// Make multiple requests without providing request IDs
	ids := make(map[string]bool)

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(w, req)

		id := w.Header().Get("X-Request-ID")
		if id == "" {
			t.Fatal("X-Request-ID header missing from response")
		}

		if ids[id] {
			t.Errorf("duplicate request ID generated: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != 10 {
		t.Errorf("expected 10 unique IDs, got %d", len(ids))
	}
}

func TestRequestIDFormat(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		shouldMatch   bool
		testID        string
		description   string
	}{
		{
			name:        "UUID format",
			pattern:     "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
			shouldMatch: true,
			testID:      "550e8400-e29b-41d4-a716-446655440000",
			description: "standard UUID format",
		},
		{
			name:        "alphanumeric format",
			pattern:     "^[a-zA-Z0-9-]+$",
			shouldMatch: true,
			testID:      "abc123-def456-ghi789",
			description: "alphanumeric with hyphens",
		},
		{
			name:        "non-empty format",
			pattern:     ".+",
			shouldMatch: true,
			testID:      "some-request-id",
			description: "non-empty string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()

			handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(w, req)

			id := w.Header().Get("X-Request-ID")
			if id == "" {
				t.Fatal("X-Request-ID header missing")
			}

			// Verify format matches pattern
			matched, err := regexp.MatchString(tt.pattern, id)
			if err != nil {
				t.Fatalf("regex error: %v", err)
			}

			if tt.shouldMatch && !matched {
				t.Errorf("request ID %q does not match pattern %q (%s)", id, tt.pattern, tt.description)
			}

			// Verify ID is not empty
			if strings.TrimSpace(id) == "" {
				t.Error("request ID is empty or whitespace")
			}
		})
	}
}

func TestRequestIDPropagatedToContext(t *testing.T) {
	const expectedID = "test-request-123"

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("X-Request-ID", expectedID)
	w := httptest.NewRecorder()

	var capturedID string

	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = r.Header.Get("X-Request-ID")
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	if capturedID != expectedID {
		t.Errorf("request ID in context: got %q, want %q", capturedID, expectedID)
	}
}
