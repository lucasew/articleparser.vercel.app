package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler_InvalidURL(t *testing.T) {
	req := httptest.NewRequest("GET", "/api?url=", nil)
	w := httptest.NewRecorder()

	Handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Handler() status = %v; want %v", resp.StatusCode, http.StatusBadRequest)
	}
}
