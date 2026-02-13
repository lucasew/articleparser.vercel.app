package formatter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

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

/**
 * DefaultTemplate is the parsed Go template instance.
 *
 * It is initialized at startup to avoid the overhead of parsing the template
 * on every request, ensuring faster response times.
 */
var DefaultTemplate = template.Must(template.New("article").Parse(Template))

/**
 * formatHandler defines the function signature for handling different output formats.
 */
type formatHandler func(w http.ResponseWriter, article readability.Article, buf *bytes.Buffer)

var formatters = map[string]formatHandler{
	"html":     formatHTML,
	"md":       formatMarkdown,
	"markdown": formatMarkdown,
	"json":     formatJSON,
	"text":     formatText,
	"txt":      formatText,
}

/**
 * Render outputs the article in the requested format.
 */
func Render(w http.ResponseWriter, article readability.Article, contentBuf *bytes.Buffer, format string) error {
	formatter, found := formatters[format]
	if !found {
		return fmt.Errorf("invalid format: %s", format)
	}
	formatter(w, article, contentBuf)
	return nil
}

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
		log.Printf("error executing HTML template: %v", err)
	}
}

func formatMarkdown(w http.ResponseWriter, _ readability.Article, buf *bytes.Buffer) {
	w.Header().Set("Content-Type", "text/markdown")
	if err := godown.Convert(w, buf, nil); err != nil {
		log.Printf("error converting to markdown: %v", err)
	}
}

func formatJSON(w http.ResponseWriter, article readability.Article, buf *bytes.Buffer) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"title":   article.Title(),
		"content": buf.String(),
	}); err != nil {
		log.Printf("error encoding json: %v", err)
	}
}

func formatText(w http.ResponseWriter, _ readability.Article, buf *bytes.Buffer) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write(buf.Bytes()); err != nil {
		log.Printf("error writing text response: %v", err)
	}
}
