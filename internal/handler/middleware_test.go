package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type handleJsonTestReq struct {
	Name string `json:"name"`
}

func TestHandleJsonWithinLimitSucceeds(t *testing.T) {
	SetMaxBodyBytes(1024)
	defer SetMaxBodyBytes(defaultMaxBodyBytes)

	called := false
	h := HandleJson(func(w http.ResponseWriter, r *http.Request, req handleJsonTestReq) {
		called = true
		if req.Name != "ok" {
			t.Errorf("name: got %q, want ok", req.Name)
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"ok"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	if !called {
		t.Error("handler was not called")
	}
}

func TestHandleJsonRejectsOversizedBody(t *testing.T) {
	SetMaxBodyBytes(16)
	defer SetMaxBodyBytes(defaultMaxBodyBytes)

	called := false
	h := HandleJson(func(w http.ResponseWriter, r *http.Request, req handleJsonTestReq) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	oversized := `{"name":"` + strings.Repeat("a", 100) + `"}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(oversized))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}
	if called {
		t.Error("handler should not have been called for an oversized body")
	}
}

func TestHandleJsonDefaultLimitIsOneMegabyte(t *testing.T) {
	if defaultMaxBodyBytes != 1<<20 {
		t.Errorf("defaultMaxBodyBytes: got %d, want %d (1MB, per spec.md)", defaultMaxBodyBytes, 1<<20)
	}
}

func TestHandleJsonStillRejectsInvalidJSON(t *testing.T) {
	SetMaxBodyBytes(defaultMaxBodyBytes)

	h := HandleJson(func(w http.ResponseWriter, r *http.Request, req handleJsonTestReq) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/", strings.NewReader(`not json`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}
