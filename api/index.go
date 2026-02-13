/**
 * Package handler implements the Vercel Serverless Function entrypoint.
 * It handles URL parsing, fetching, readability parsing, and formatting logic for the reader application.
 *
 * It is designed to be deployed as a serverless function, handling requests to process
 * and clean up web articles for easier reading or LLM consumption.
 */
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
	"math/rand"
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

	/**
	 * ReadabilityParser is the shared instance of the readability parser.
	 *
	 * It is reusable and thread-safe, allowing concurrent processing of multiple
	 * requests without the need to create new parser instances.
	 */
	ReadabilityParser = readability.NewParser()

	// httpClient used for fetching remote articles with timeouts and redirect policy
	httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext: newSafeDialer().DialContext,
		},
		Timeout: httpClientTimeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		},
	}
)

/**
 * newSafeDialer creates a custom net.Dialer that prevents Server-Side Request Forgery (SSRF).
 *
 * It validates the resolved IP address before connecting, ensuring that it is not:
 * - A private network address (e.g., 192.168.x.x, 10.x.x.x)
 * - A loopback address (e.g., 127.0.0.1)
 * - An unspecified address (e.g., 0.0.0.0)
 *
 * This validation happens *after* DNS resolution but *before* the connection is established.
 * This prevents Time-of-Check Time-of-Use (TOCTOU) attacks where a domain could
 * resolve to a safe IP during check but switch to a private IP during connection.
 *
 * This is critical for preventing the application from accessing internal services or metadata services
 * (like AWS EC2 metadata) running on the same network.
 */
func newSafeDialer() *net.Dialer {
	dialer := &net.Dialer{
		Timeout:   dialerTimeout,
		KeepAlive: dialerKeepAlive,
		Control: func(_, address string, _ syscall.RawConn) error {
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

/**
 * userAgentPool contains a list of real browser User-Agent strings.
 *
 * We rotate through these to mimic legitimate traffic, as many websites block requests
 * from default HTTP clients (like Go-http-client) or known bot User-Agents.
 * This list requires periodic maintenance to stay current with browser versions.
 */
var userAgentPool = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36 Edg/134.0.0.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:135.0) Gecko/20100101 Firefox/135.0",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3 Mobile/15E148 Safari/604.1",
}

/**
 * llmUserAgents contains a list of substring identifiers for known LLM bots and crawlers.
 *
 * This list is used to detect requests from AI agents (like GPTBot, Claude, etc.)
 * so the application can automatically serve a token-efficient format (Markdown)
 * instead of full HTML.
 */
var llmUserAgents = []string{
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

/**
 * getRandomUserAgent returns a random User-Agent string from the pool.
 *
 * Rotating User-Agents helps to evade simple anti-bot measures that block requests
 * based on static or default Go HTTP client User-Agents.
 */
func getRandomUserAgent() string {
	return userAgentPool[rand.Intn(len(userAgentPool))]
}

/**
 * fetchAndParse retrieves the content from the target URL and parses it using the readability library.
 *
 * Key behaviors:
 * - Spoofs User-Agent and other browser headers to avoid blocking.
 * - Forwards Accept-Language from the client to respect language preferences.
 * - Sets security headers (Sec-Fetch-*) to look like a navigation request.
 * - Limits the response body size to maxBodySize to prevent Out-Of-Memory (OOM) crashes on large pages.
 * - Uses a custom httpClient with SSRF protection.
 */
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

/**
 * normalizeAndValidateURL cleans and validates the user-provided URL.
 *
 * It handles common normalization issues, such as:
 * - Missing scheme (defaults to https://).
 * - Malformed schemes caused by some proxies (e.g., http:/example.com -> http://example.com).
 *
 * It also restricts the scheme to 'http' or 'https' to prevent usage of other protocols like 'file://' or 'gopher://'.
 */
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

/**
 * securityHeadersMiddleware applies a baseline of security headers to every response.
 *
 * Headers set:
 * - Content-Security-Policy: Restricts sources for scripts, styles, and other content to prevent XSS.
 *   - default-src 'self': Only allow content from same origin by default.
 *   - script-src 'self' ...: Whitelists the bookmarklet script.
 *   - style-src 'self' ...: Whitelists external CSS for the Sakura theme (unpkg.com).
 * - X-Content-Type-Options: Prevents MIME-sniffing.
 * - X-Frame-Options: Prevents clickjacking by denying framing.
 * - Referrer-Policy: Controls how much referrer information is sent.
 */
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://bookmarklet-theme.vercel.app; style-src 'self' https://unpkg.com;")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer-when-downgrade")
		next.ServeHTTP(w, r)
	})
}

/**
 * Handler is the Vercel Serverless Function entrypoint.
 *
 * It is invoked by Vercel for all matching routes defined in `vercel.json`.
 * Since Vercel rewrites the path (e.g., `/api/extract` -> `/api/index.go`),
 * we rely on query parameters (like `url` and `format`) or request headers
 * to determine the desired action, rather than parsing the request path directly.
 */
func Handler(w http.ResponseWriter, r *http.Request) {
	securityHeadersMiddleware(http.HandlerFunc(handler)).ServeHTTP(w, r)
}

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

/**
 * reconstructTargetURL handles query parameter extraction quirks caused by Vercel rewrites.
 *
 * When Vercel rewrites a path like `/api/extract?url=http://example.com?foo=bar`,
 * the `url` query parameter might be cleanly separated from `foo=bar`.
 * This function merges stray query parameters back into the target URL to ensure
 * the full original URL is processed.
 */
func reconstructTargetURL(r *http.Request) string {
	rawLink := r.URL.Query().Get("url")
	if rawLink == "" {
		return ""
	}

	// Reconstruct URL if it was split by query parameters during rewrite
	u, err := url.Parse(rawLink)
	if err != nil {
		return rawLink
	}

	targetQuery := u.Query()
	originalQuery := r.URL.Query()
	hasChanges := false
	for k, vs := range originalQuery {
		// Skip 'url' and 'format' as they are control parameters for this API,
		// not part of the target website's query string.
		// Including them would cause recursion or invalid target URLs.
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
		return u.String()
	}
	return rawLink
}

/**
 * handler implements the core request processing pipeline.
 *
 * Flow:
 * 1. Reconstruct Target URL: Merges stray query parameters caused by Vercel rewrites.
 * 2. Determine Format: checks Query params > Accept header > User-Agent (LLM detection).
 * 3. Normalize & Validate: Ensures the target URL is valid and uses http/https.
 * 4. Fetch & Parse: Downloads the page (spoofing a browser) and extracts the main content.
 * 5. Render: Converts the parsed article to a safe HTML buffer.
 * 6. Format: Outputs the result in the requested format (HTML, Markdown, JSON, etc.).
 */
func handler(w http.ResponseWriter, r *http.Request) {
	rawLink := reconstructTargetURL(r)

	format := getFormat(r)
	log.Printf("request: %s %s", format, rawLink)

	formatter, found := formatters[format]
	if !found {
		writeError(w, http.StatusBadRequest, "invalid format")
		return
	}

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

	formatter(w, article, contentBuf)
}

/**
 * writeError writes a structured JSON error response.
 *
 * It enforces a consistent error format {"error": "message"} across the API
 * and sets the correct HTTP status code and Content-Type header.
 */
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		log.Printf("error writing error response: %v", err)
	}
}
