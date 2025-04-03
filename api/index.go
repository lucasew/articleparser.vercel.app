package handler

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/go-shiori/go-readability"
	"github.com/mattn/godown"
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
	<h1>{{.Title}}</h1>
	{{.Content}}
</body>
</html>
`

var (
	DefaultTemplate  = template.Must(template.New("article").Parse(Template))
	ReadabilityParser = readability.NewParser()
)

func init() {
	ReadabilityParser.Debug = true
}

func fetchAndParse(ctx context.Context, link *url.URL) (readability.Article, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", link.String(), nil)
	if err != nil {
		return readability.Article{}, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return readability.Article{}, err
	}
	defer res.Body.Close()

	node, err := html.Parse(res.Body)
	if err != nil {
		return readability.Article{}, err
	}

	return ReadabilityParser.ParseDocument(node, link)
}

// FixURL vercel for some reason strip out one of the slashes of https:// when normalizing the url
func FixURL(link string) string {
	slashIndex := strings.Index(link, "/")
	if slashIndex >= 0 && slashIndex+1 < len(link) && link[slashIndex+1] != '/' {
		return strings.Replace(link, "/", "//", 1)
	}
	return link
}

func Handler(w http.ResponseWriter, r *http.Request) {
	rawLink := FixURL(r.URL.Query().Get("url"))
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "html"
	}
	log.Printf("request: %s %s", format, rawLink)

	link, err := url.Parse(rawLink)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	article, err := fetchAndParse(ctx, link)
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	buf := bytes.Buffer{}
	if err = DefaultTemplate.Execute(&buf, article); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch format {
	case "html":
		if _, err := io.Copy(w, &buf); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "md", "markdown":
		godown.Convert(w, &buf, nil)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}
