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
<!DOCTYPE html>
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
var ReadabilityParser readability.Parser

func init() {
	DefaultTemplate = template.Must(template.New("article").Parse(Template))

	ReadabilityParser = readability.NewParser()
	ReadabilityParser.Debug = true
}

func NewParser(ctx context.Context, link *url.URL) (*Parser, error) {
	buf := bytes.NewBuffer([]byte{})
	req, err := http.NewRequestWithContext(ctx, "GET", link.String(), buf)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	node, err := html.Parse(res.Body)
	ret := &Parser{
		link: link,
		page: node,
	}
	return ret, nil
}

type Parser struct {
	link *url.URL
	page *html.Node
}

func (p *Parser) ParseArticle() (readability.Article, error) {
	return ReadabilityParser.ParseDocument(p.page, p.link)
}

func Handler(w http.ResponseWriter, r *http.Request) {
	link, err := url.Parse(r.URL.Query().Get("url"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()

	parser, err := NewParser(ctx, link)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	article, err := parser.ParseArticle()
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	err = DefaultTemplate.Execute(w, article)
	if err != nil {
		w.WriteHeader(500)
	}
}
