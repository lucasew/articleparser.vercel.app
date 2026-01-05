package handler

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"net/http"

	"github.com/go-shiori/go-readability"
	"github.com/mattn/godown"
)

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
	DefaultTemplate = template.Must(template.New("article").Parse(Template))
)

// renderer is a function that renders an article in a specific format.
type renderer func(http.ResponseWriter, *bytes.Buffer, readability.Article)

// renderers maps format strings to their corresponding render functions.
var renderers = map[string]renderer{
	"html":     renderHTML,
	"md":       renderMarkdown,
	"markdown": renderMarkdown,
	"json":     renderJSON,
}

// renderHTML renders the article in HTML format.
func renderHTML(w http.ResponseWriter, buf *bytes.Buffer, _ readability.Article) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.Copy(w, buf)
}

// renderMarkdown renders the article in Markdown format.
func renderMarkdown(w http.ResponseWriter, buf *bytes.Buffer, _ readability.Article) {
	w.Header().Set("Content-Type", "text/markdown")
	godown.Convert(w, buf, nil)
}

// renderJSON renders the article in JSON format.
func renderJSON(w http.ResponseWriter, _ *bytes.Buffer, article readability.Article) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"title":   article.Title,
		"content": article.Content,
	})
}

// renderArticle renders the article using the appropriate renderer based on the format.
func renderArticle(w http.ResponseWriter, r *http.Request, article readability.Article) {
	format := getFormat(r)
	renderer, ok := renderers[format]
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid format")
		return
	}

	buf := &bytes.Buffer{}
	// inject safe HTML content
	data := struct {
		Title   string
		Content template.HTML
	}{
		Title:   article.Title,
		Content: template.HTML(article.Content),
	}

	if err := DefaultTemplate.Execute(buf, data); err != nil {
		writeError(w, http.StatusInternalServerError, "template render failed")
		return
	}

	renderer(w, buf, article)
}
