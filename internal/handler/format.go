package handler

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/mattn/godown"
)

/**
 * Template is the raw HTML template string used for rendering the article.
 *
 * It provides a minimal HTML5 structure and includes the Sakura CSS library
 * for a clean, typography-focused reading experience without distractions.
 * The template expects a struct with Title and Content fields.
 */
const Template = `
<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8"/>
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<link id="theme" rel="stylesheet" href="https://unpkg.com/sakura.css/css/sakura.css">
</head>
<body>
	<script src="https://bookmarklet-theme.vercel.app/script.js"></script>
	<h1>{{.Title}}</h1>
	{{.Content}}
</body>
</html>
`

var (
	/**
	 * DefaultTemplate is the parsed Go template instance.
	 *
	 * It is initialized at startup to avoid the overhead of parsing the template
	 * on every request, ensuring faster response times.
	 */
	DefaultTemplate = template.Must(template.New("article").Parse(Template))
)

/**
 * formatHandler defines the function signature for handling different output formats.
 *
 * Implementations are responsible for:
 * 1. Setting the appropriate Content-Type header.
 * 2. Encoding the article content (HTML, JSON, Markdown, etc.) into the response writer.
 * 3. Handling any encoding errors (logging them, as headers are already written).
 */
type formatHandler func(w http.ResponseWriter, article readability.Article, buf *bytes.Buffer)

/**
 * formatHTML renders the article using the standard HTML template.
 * This is the default view for human consumption.
 */
func formatHTML(w http.ResponseWriter, article readability.Article, contentBuf *bytes.Buffer) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// inject safe HTML content
	data := struct {
		Title   string
		Content template.HTML
	}{
		Title:   article.Title(),
		Content: template.HTML(contentBuf.String()),
	}
	if err := DefaultTemplate.Execute(w, data); err != nil {
		// at this point, we can't write a JSON error, so we log it
		log.Printf("error executing HTML template: %v", err)
	}
}

/**
 * formatMarkdown converts the article content to Markdown.
 * Useful for LLMs or note-taking applications.
 */
func formatMarkdown(w http.ResponseWriter, _ readability.Article, buf *bytes.Buffer) {
	w.Header().Set("Content-Type", "text/markdown")
	if err := godown.Convert(w, buf, nil); err != nil {
		log.Printf("error converting to markdown: %v", err)
	}
}

/**
 * formatJSON returns the raw title and HTML content in a JSON object.
 * Useful for programmatic consumption where the client wants to handle rendering.
 */
func formatJSON(w http.ResponseWriter, article readability.Article, buf *bytes.Buffer) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"title":   article.Title(),
		"content": buf.String(),
	}); err != nil {
		log.Printf("error encoding json: %v", err)
	}
}

/**
 * formatText returns the plain text content, stripped of HTML tags.
 */
func formatText(w http.ResponseWriter, _ readability.Article, buf *bytes.Buffer) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write(buf.Bytes()); err != nil {
		log.Printf("error writing text response: %v", err)
	}
}

/**
 * formatters maps format names (including aliases) to their respective handler functions.
 *
 * This design allows for easy extensibility of output formats. New formats can be
 * added by implementing a formatHandler and registering it here.
 */
var formatters = map[string]formatHandler{
	"html":     formatHTML,
	"md":       formatMarkdown,
	"markdown": formatMarkdown,
	"json":     formatJSON,
	"text":     formatText,
	"txt":      formatText,
}

/**
 * isLLM attempts to detect if the request is originated from a known LLM crawler or tool.
 *
 * It checks the User-Agent string against a list of known identifiers (e.g., GPTBot, Claude).
 * This allows the application to default to a machine-friendly format (Markdown) automatically.
 */
func isLLM(r *http.Request) bool {
	ua := strings.ToLower(r.UserAgent())
	for _, s := range llmUserAgents {
		if strings.Contains(ua, s) {
			return true
		}
	}
	return false
}

/**
 * getFormat determines the desired output format based on request signals.
 *
 * Priority order:
 * 1. Query parameter 'format' (explicit override).
 * 2. Accept Header (content negotiation).
 * 3. LLM Detection (auto-switch to Markdown for bots).
 * 4. Default to 'html'.
 */
func getFormat(r *http.Request) string {
	// 1. Priority: Query parameter
	format := r.URL.Query().Get("format")
	if format != "" {
		return format
	}

	// 2. Priority: Accept Header
	accept := strings.ToLower(r.Header.Get("Accept"))
	if strings.Contains(accept, "application/json") {
		return "json"
	}
	if strings.Contains(accept, "text/markdown") || strings.Contains(accept, "text/x-markdown") {
		return "md"
	}
	if strings.Contains(accept, "text/plain") {
		return "text"
	}
	if strings.Contains(accept, "text/html") {
		return "html"
	}

	// 3. Priority: LLM Detection (defaults to markdown)
	if isLLM(r) {
		return "md"
	}

	return "html"
}
