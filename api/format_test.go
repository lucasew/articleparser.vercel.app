package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/readeck/go-readability/v2"
	"golang.org/x/net/html"
)

func TestFormatTextRendersPlainText(t *testing.T) {
	// Minimal article node: <p>Plain body</p>
	p := &html.Node{Type: html.ElementNode, Data: "p"}
	p.AppendChild(&html.Node{Type: html.TextNode, Data: "Plain body"})
	article := readability.Article{Node: p}

	rec := httptest.NewRecorder()
	// Pass HTML-looking buffer deliberately: formatText must ignore it.
	htmlBuf := bytes.NewBufferString("<p>should not appear</p>")
	formatText(rec, article, htmlBuf)

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("Content-Type = %q; want text/plain", ct)
	}
	body := rec.Body.String()
	if strings.Contains(body, "<p>") || strings.Contains(body, "should not appear") {
		t.Fatalf("formatText returned HTML or buffer contents: %q", body)
	}
	if !strings.Contains(body, "Plain body") {
		t.Fatalf("formatText missing plain text, got: %q", body)
	}
}

func TestWriteJSONStatusAndPayload(t *testing.T) {
	okRec := httptest.NewRecorder()
	writeJSON(okRec, http.StatusOK, map[string]string{"title": "T", "content": "C"})
	if okRec.Code != http.StatusOK {
		t.Fatalf("writeJSON 200: status = %d", okRec.Code)
	}
	if ct := okRec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("writeJSON 200: Content-Type = %q", ct)
	}
	var okBody map[string]string
	if err := json.Unmarshal(okRec.Body.Bytes(), &okBody); err != nil {
		t.Fatalf("writeJSON 200: decode: %v", err)
	}
	if okBody["title"] != "T" || okBody["content"] != "C" {
		t.Fatalf("writeJSON 200: payload = %v", okBody)
	}

	errRec := httptest.NewRecorder()
	writeError(errRec, http.StatusBadRequest, "invalid format")
	if errRec.Code != http.StatusBadRequest {
		t.Fatalf("writeError: status = %d; want 400", errRec.Code)
	}
	var errBody map[string]string
	if err := json.Unmarshal(errRec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("writeError: decode: %v", err)
	}
	if errBody["error"] != "invalid format" {
		t.Fatalf("writeError: payload = %v", errBody)
	}
}
