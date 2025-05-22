package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
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
	DefaultTemplate   = template.Must(template.New("article").Parse(Template))
	ReadabilityParser = readability.NewParser()
	// httpClient used for fetching remote articles with timeouts and redirect policy
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
			}
			return nil
		},
	}
	// limit download size to avoid OOM (2 MiB)
	maxContentBytes = int64(2 * 1024 * 1024)
)

func init() {
	ReadabilityParser.Debug = true
}

// AIMPROV: Extract fetching and parsing logic into a separate function to improve readability and testability.
func fetchAndParse(ctx context.Context, link *url.URL) (readability.Article, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", link.String(), nil)
	if err != nil {
		return readability.Article{}, err
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return readability.Article{}, err
	}
	defer res.Body.Close()

	// limit body size to prevent OOM
	reader := io.LimitReader(res.Body, maxContentBytes)
	node, err := html.Parse(reader)
	if err != nil {
		return readability.Article{}, err
	}

	return ReadabilityParser.ParseDocument(node, link)
}

// AIMPROV: Create a function to handle URL normalization and validation.
func normalizeAndValidateURL(rawLink string) (*url.URL, error) {
	if rawLink == "" {
		return nil, errors.New("url parameter is empty")
	}
	// add scheme if missing
	if !strings.Contains(rawLink, "://") {
		// default to https if no scheme provided
		rawLink = fmt.Sprintf("https://%s", rawLink)
	}
	link, err := url.Parse(rawLink)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	// only allow http(s)
	if link.Scheme != "http" && link.Scheme != "https" {
		return nil, errors.New("unsupported URL scheme")
	}
	host := link.Hostname()
	// resolve and block private IPs
	ips, err := net.LookupIP(host)
	if err == nil {
		for _, ip := range ips {
			if isPrivateIP(ip) {
				return nil, errors.New("refusing private network address")
			}
		}
	}
	return link, nil
}

func Handler(w http.ResponseWriter, r *http.Request) {
	rawLink := r.URL.Query().Get("url")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "html"
	}
	log.Printf("request: %s %s", format, rawLink)

	link, err := normalizeAndValidateURL(rawLink)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	article, err := fetchAndParse(ctx, link)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
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
	if err = DefaultTemplate.Execute(buf, data); err != nil {
		writeError(w, http.StatusInternalServerError, "template render failed")
		return
	}

	switch format {
	case "html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.Copy(w, buf)
	case "md", "markdown":
		w.Header().Set("Content-Type", "text/markdown")
		godown.Convert(w, buf, nil)
	case "json":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"title":   article.Title,
			"content": article.Content,
		})
	default:
		writeError(w, http.StatusBadRequest, "invalid format")
	}
}

// writeError writes a JSON error message with given status
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// isPrivateIP reports whether ip is in a private or loopback range
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		switch ip4[0] {
		case 10:
			return true
		case 172:
			if ip4[1]&0xf0 == 16 {
				return true
			}
		case 192:
			if ip4[1] == 168 {
				return true
			}
		}
	}
	return false
}
