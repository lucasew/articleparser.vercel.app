package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/mattn/godown"
	"golang.org/x/net/context"
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
				if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
					return errors.New("refusing to connect to private network address")
				}
			}
			return nil
		},
	}
	return dialer
}

var userAgentPool = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36 Edg/134.0.0.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:135.0) Gecko/20100101 Firefox/135.0",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3 Mobile/15E148 Safari/604.1",
}

func getRandomUserAgent() string {
	return userAgentPool[rand.Intn(len(userAgentPool))]
}

func fetchAndParse(ctx context.Context, link *url.URL, r *http.Request) (readability.Article, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", link.String(), nil)
	if err != nil {
		return readability.Article{}, err
	}

	// Always spoof everything to look like a real browser
	ua := getRandomUserAgent()
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")

	// Fallback headers from client request
	if lang := r.Header.Get("Accept-Language"); lang != "" {
		req.Header.Set("Accept-Language", lang)
	} else {
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	}

	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

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

	// Fix browser/proxy normalization of :// to :/
	if strings.HasPrefix(rawLink, "http:/") && !strings.HasPrefix(rawLink, "http://") {
		rawLink = "http://" + rawLink[6:]
	} else if strings.HasPrefix(rawLink, "https:/") && !strings.HasPrefix(rawLink, "https://") {
		rawLink = "https://" + rawLink[7:]
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

func formatText(w http.ResponseWriter, _ readability.Article, buf *bytes.Buffer) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(buf.String()))
}

var formatters = map[string]formatHandler{
	"html":     formatHTML,
	"md":       formatMarkdown,
	"markdown": formatMarkdown,
	"json":     formatJSON,
	"text":     formatText,
	"txt":      formatText,
}

// isLLM attempts to detect if the request is coming from an LLM or a tool used by one.
func isLLM(r *http.Request) bool {
	ua := strings.ToLower(r.UserAgent())
	llmStrings := []string{
		"gptbot",
		"chatgpt",
		"claude",
		"googlebot",
		"bingbot",
		"anthropic",
		"perplexity",
		"claudebot",
		"github-copilot",
	}
	for _, s := range llmStrings {
		if strings.Contains(ua, s) {
			return true
		}
	}
	return false
}

// getFormat determines the output format from the request, defaulting to "html".
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

// handler is the actual logic
func handler(w http.ResponseWriter, r *http.Request) {
	rawLink := r.URL.Query().Get("url")
	if rawLink != "" {
		// Reconstruct URL if it was split by query parameters during rewrite
		u, err := url.Parse(rawLink)
		if err == nil {
			targetQuery := u.Query()
			originalQuery := r.URL.Query()
			hasChanges := false
			for k, vs := range originalQuery {
				if k == "url" || k == "format" {
					continue
				}
				hasChanges = true
				for _, v := range vs {
					targetQuery.Add(k, v)
				}
			}
			if hasChanges {
				u.RawQuery = targetQuery.Encode()
				rawLink = u.String()
			}
		}
	}

	format := getFormat(r)
	log.Printf("request: %s %s", format, rawLink)

	link, err := normalizeAndValidateURL(rawLink)
	if err != nil {
		log.Printf("error normalizing URL %q: %v", rawLink, err)
		writeError(w, http.StatusBadRequest, "Invalid URL provided")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), handlerTimeout)
	defer cancel()

	article, err := fetchAndParse(ctx, link, r)
	if err != nil {
		log.Printf("error fetching or parsing URL %q: %v", rawLink, err)
		writeError(w, http.StatusUnprocessableEntity, "Failed to process URL")
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
