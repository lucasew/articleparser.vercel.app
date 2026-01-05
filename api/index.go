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
		Timeout:       10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	// limit download size to avoid OOM (2 MiB)
	maxContentBytes = int64(2 * 1024 * 1024)
)

// AIMPROV: Extract fetching and parsing logic into a separate function to improve readability and testability.
func fetchAndParse(ctx context.Context, link *url.URL) (readability.Article, error) {
	var res *http.Response
	var err error
	// Manually handle redirects to prevent SSRF
	for i := 0; i < 5; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", link.String(), nil)
		if err != nil {
			return readability.Article{}, err
		}

		res, err = httpClient.Do(req)
		if err != nil {
			return readability.Article{}, err
		}

		if res.StatusCode >= 300 && res.StatusCode <= 399 {
			redirectURL, err := res.Location()
			if err != nil {
				return readability.Article{}, fmt.Errorf("failed to get redirect location: %w", err)
			}
			link, err = normalizeAndValidateURL(redirectURL.String())
			if err != nil {
				return readability.Article{}, fmt.Errorf("redirect to invalid URL blocked: %w", err)
			}
			continue // follow the redirect
		}
		// not a redirect, break the loop
		break
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return readability.Article{}, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

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
			if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				return nil, errors.New("refusing private network address")
			}
		}
	}
	return link, nil
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://bookmarklet-theme.vercel.app; style-src 'self' https://unpkg.com;")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer-when-downgrade")
		next.ServeHTTP(w, r)
	})
}

// Handler is the main entrypoint, we wrap it with security middleware
func Handler(w http.ResponseWriter, r *http.Request) {
	securityHeadersMiddleware(http.HandlerFunc(handler)).ServeHTTP(w, r)
}

// getFormat extracts the 'format' query param, defaulting to "html"
func getFormat(r *http.Request) string {
	format := r.URL.Query().Get("format")
	if format == "" {
		return "html"
	}
	return format
}

// handler is the actual logic
func handler(w http.ResponseWriter, r *http.Request) {
	rawLink := r.URL.Query().Get("url")
	format := getFormat(r)
	log.Printf("request: %s %s", format, rawLink)

	link, err := normalizeAndValidateURL(rawLink)
	if err != nil {
		log.Printf("error normalizing URL %q: %v", rawLink, err)
		writeError(w, http.StatusBadRequest, "Invalid URL provided")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	article, err := fetchAndParse(ctx, link)
	if err != nil {
		log.Printf("error fetching or parsing URL %q: %v", rawLink, err)
		writeError(w, http.StatusUnprocessableEntity, "Failed to process URL")
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
