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
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"codeberg.org/readeck/go-readability/v2"
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

var (
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
		rawLink = "http://" + strings.TrimPrefix(rawLink, "http:/")
	} else if strings.HasPrefix(rawLink, "https:/") && !strings.HasPrefix(rawLink, "https://") {
		rawLink = "https://" + strings.TrimPrefix(rawLink, "https:/")
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
	log.Printf("request: %q %q", format, rawLink)

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
