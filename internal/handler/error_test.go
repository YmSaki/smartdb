package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		errorCode      string
		errorMessage   string
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "400 Bad Request",
			statusCode:     http.StatusBadRequest,
			errorCode:      "INVALID_PROJECT_NAME",
			errorMessage:   "Project name must match [a-z0-9][a-z0-9_-]*",
			expectedStatus: 400,
			expectedCode:   "INVALID_PROJECT_NAME",
		},
		{
			name:           "401 Unauthorized",
			statusCode:     http.StatusUnauthorized,
			errorCode:      "INVALID_TOKEN",
			errorMessage:   "Invalid or missing authentication token",
			expectedStatus: 401,
			expectedCode:   "INVALID_TOKEN",
		},
		{
			name:           "403 Forbidden",
			statusCode:     http.StatusForbidden,
			errorCode:      "INSUFFICIENT_PERMISSIONS",
			errorMessage:   "Your role does not permit this operation",
			expectedStatus: 403,
			expectedCode:   "INSUFFICIENT_PERMISSIONS",
		},
		{
			name:           "404 Not Found",
			statusCode:     http.StatusNotFound,
			errorCode:      "PROJECT_NOT_FOUND",
			errorMessage:   "Project does not exist",
			expectedStatus: 404,
			expectedCode:   "PROJECT_NOT_FOUND",
		},
		{
			name:           "500 Internal Server Error",
			statusCode:     http.StatusInternalServerError,
			errorCode:      "INTERNAL_ERROR",
			errorMessage:   "Internal server error",
			expectedStatus: 500,
			expectedCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteError(w, tt.statusCode, tt.errorCode, tt.errorMessage)

			if w.Code != tt.expectedStatus {
				t.Errorf("status code: got %d, want %d", w.Code, tt.expectedStatus)
			}

			var resp ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&resp)
			if err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Error.Code != tt.expectedCode {
				t.Errorf("error code: got %q, want %q", resp.Error.Code, tt.expectedCode)
			}

			if resp.Error.Message != tt.errorMessage {
				t.Errorf("error message: got %q, want %q", resp.Error.Message, tt.errorMessage)
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name           string
		data           interface{}
		expectedStatus int
	}{
		{
			name:           "write simple JSON object",
			data:           map[string]interface{}{"key": "value"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "write JSON array",
			data:           []string{"a", "b", "c"},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteJSON(w, tt.expectedStatus, tt.data)

			if w.Code != tt.expectedStatus {
				t.Errorf("status code: got %d, want %d", w.Code, tt.expectedStatus)
			}

			if w.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type: got %q, want %q", w.Header().Get("Content-Type"), "application/json")
			}
		})
	}
}

func TestWriteErrorStatusCodes(t *testing.T) {
	statusCodes := []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
	}

	for _, code := range statusCodes {
		t.Run(("status code "+http.StatusText(code)), func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteError(w, code, "TEST_ERROR", "Test error message")

			if w.Code != code {
				t.Errorf("status code: got %d, want %d", w.Code, code)
			}

			var resp ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&resp)
			if err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Error.Code == "" {
				t.Error("error code is empty")
			}

			if resp.Error.Message == "" {
				t.Error("error message is empty")
			}
		})
	}
}
