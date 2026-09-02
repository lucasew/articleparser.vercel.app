package handler

import (
	"bytes"
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
