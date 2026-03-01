package handler

import (
	"bytes"
	"strings"
	"testing"
)

// mockArticle implements readability.Article for testing
type mockArticle struct {
	title string
	html  string
}

func (m mockArticle) Title() string {
	return m.title
}

func (m mockArticle) RenderHTML(w *bytes.Buffer) error {
	w.WriteString(m.html)
	return nil
}

func TestXSSHTMLSanitization(t *testing.T) {
	// A mock article with malicious content
	maliciousHTML := `<p>Hello</p><script>alert("xss")</script><img src="x" onerror="alert('xss')">`
	article := mockArticle{title: "Test", html: maliciousHTML}

	buf := &bytes.Buffer{}
	if err := article.RenderHTML(buf); err != nil {
		t.Fatalf("failed to render html: %v", err)
	}

	sanitizedHTML := sanitizeHTML(buf.String())

	if strings.Contains(sanitizedHTML, "<script>") || strings.Contains(sanitizedHTML, "onerror") {
		t.Errorf("HTML was not sanitized correctly: %s", sanitizedHTML)
	}
}
