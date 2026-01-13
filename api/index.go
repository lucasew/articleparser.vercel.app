package handler

import (
	"bytes"
	"context"
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
	"syscall"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/mattn/godown"
	"golang.org/x/net/html"
)

const (
	maxRedirects      = 5
	httpClientTimeout = 10 * time.Second
	maxBodySize       = int64(2 * 1024 * 1024) // 2 MiB
	dialerTimeout     = 30 * time.Second
	dialerKeepAlive   = 30 * time.Second
	handlerTimeout    = 5 * time.Second
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
		Transport: &http.Transport{
			DialContext: newSafeDialer().DialContext,
		},
		Timeout: httpClientTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		},
	}
)

func newSafeDialer() *net.Dialer {
	dialer := &net.Dialer{
		Timeout:   dialerTimeout,
		KeepAlive: dialerKeepAlive,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ips, err := net.LookupIP(host)
			if err != nil {
				return err
			}
			for _, ip := range ips {
				if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
					return errors.New("refusing to connect to private network address")
				}
			}
			return nil
		},
	}
	return dialer
}

const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36 Edg/134.0.0.0"

func fetchAndParse(ctx context.Context, link *url.URL, userAgent string) (readability.Article, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", link.String(), nil)
	if err != nil {
		return readability.Article{}, err
	}
	if userAgent == "" {
		// use a generic user-agent as fallback
		userAgent = defaultUserAgent
	}
	req.Header.Set("User-Agent", userAgent)

	res, err := httpClient.Do(req)
	if err != nil {
		return readability.Article{}, err
	}
	defer res.Body.Close()

	// limit body size to prevent OOM
	reader := io.LimitReader(res.Body, maxBodySize)
	node, err := html.Parse(reader)
	if err != nil {
		return readability.Article{}, err
	}

	return ReadabilityParser.ParseDocument(node, link)
}

func normalizeAndValidateURL(rawLink string) (*url.URL, error) {
	if rawLink == "" {
		return nil, errors.New("url parameter is empty")
	}
	u, err := url.Parse(rawLink)
	u.Scheme = "https"
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	return u, nil
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

// formatHandler defines the function signature for handling different output formats.
type formatHandler func(w http.ResponseWriter, article readability.Article, buf *bytes.Buffer)

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

func formatMarkdown(w http.ResponseWriter, _ readability.Article, buf *bytes.Buffer) {
	w.Header().Set("Content-Type", "text/markdown")
	godown.Convert(w, buf, nil)
}

func formatJSON(w http.ResponseWriter, article readability.Article, buf *bytes.Buffer) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"title":   article.Title(),
		"content": buf.String(),
	})
}

var formatters = map[string]formatHandler{
	"html":     formatHTML,
	"md":       formatMarkdown,
	"markdown": formatMarkdown,
	"json":     formatJSON,
}

// getFormat determines the output format from the request, defaulting to "html".
func getFormat(r *http.Request) string {
	// /api/:format/:url*
	// /api/html/https://example.com
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) > 2 && parts[2] != "" {
		return parts[2]
	}
	return "html"
}

// getFormatFromAcceptHeader determines the output format from the Accept header.
func getFormatFromAcceptHeader(r *http.Request) string {
	acceptHeader := r.Header.Get("Accept")
	switch {
	case strings.Contains(acceptHeader, "application/json"):
		return "json"
	case strings.Contains(acceptHeader, "text/markdown"):
		return "md"
	case strings.Contains(acceptHeader, "text/html"):
		return "html"
	default:
		// default to html if no specific format is requested
		return "html"
	}
}

func getURL(r *http.Request) string {
	// /api/:format/:url*
	// /api/html/https://example.com
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) > 3 {
		return strings.Join(parts[3:], "/")
	}
	return r.URL.Query().Get("url")
}

// handler is the actual logic
func handler(w http.ResponseWriter, r *http.Request) {
	rawLink := getURL(r)
	var format string
	if r.URL.Query().Get("source") == "extract" {
		format = getFormatFromAcceptHeader(r)
	} else {
		format = getFormat(r)
	}
	log.Printf("request: %s %s", format, rawLink)

	link, err := normalizeAndValidateURL(rawLink)
	if err != nil {
		log.Printf("error normalizing URL %q: %v", rawLink, err)
		writeError(w, http.StatusBadRequest, "Invalid URL provided")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), handlerTimeout)
	defer cancel()

	article, err := fetchAndParse(ctx, link, r.UserAgent())
	if err != nil {
		log.Printf("error fetching or parsing URL %q: %v", rawLink, err)
		writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("Failed to process URL: %s", link))
		return
	}

	contentBuf := &bytes.Buffer{}
	if err := article.RenderHTML(contentBuf); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to render article content")
		return
	}

	formatter, found := formatters[format]
	if !found {
		writeError(w, http.StatusBadRequest, "invalid format")
		return
	}
	formatter(w, article, contentBuf)
}

// writeError writes a JSON error message with given status
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
