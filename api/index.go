package handler

import (
	"bytes"
	"net/http"
	"net/url"
	"text/template"
	"time"

	"github.com/go-shiori/go-readability"
	"golang.org/x/net/context"
	"golang.org/x/net/html"
)

const Template = `
<html>
    <head>
	<meta charset="utf-8"/>
	<link id="theme" rel="stylesheet" href="https://unpkg.com/sakura.css/css/sakura.css">
    </head>
    <body>
	<script src="https://bookmarklet-theme.vercel.app/script.js"></script>
    </body>
</html>

<h1>{{.Title}}</h1>
{{.Content}}
`

var DefaultTemplate *template.Template

func init() {
    DefaultTemplate = template.Must(template.New("article").Parse(Template))
}

func Handler(w http.ResponseWriter, r *http.Request) {
    link, err := url.Parse(r.URL.Query().Get("url"))
    if err != nil {
        w.WriteHeader(400)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
    defer cancel()
    buf := bytes.NewBuffer([]byte{})
    req, err := http.NewRequestWithContext(ctx, "GET", link.String(), buf)
    if err != nil {
        w.WriteHeader(400)
        return
    }
    res, err := http.DefaultClient.Do(req)
    if err != nil {
        w.WriteHeader(res.StatusCode)
        return
    }
    node, err := html.Parse(res.Body)
    if err != nil {
        w.WriteHeader(500)
    }
    parser := readability.NewParser()
    parser.Debug = true
    article, err := parser.ParseDocument(node, link)
    if err != nil {
        w.WriteHeader(422) // Unprocessable entity
    }
    err = DefaultTemplate.Execute(w, article)
    if err != nil {
        w.WriteHeader(500)
    }
}
