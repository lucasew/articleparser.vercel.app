package handler

import (
	"context"
	"io"
	"math/rand"
	"net/http"
	"net/url"

	"codeberg.org/readeck/go-readability/v2"
	"golang.org/x/net/html"
)

const (
	maxBodySize = int64(2 * 1024 * 1024) // 2 MiB
)

/**
 * ReadabilityParser is the shared instance of the readability parser.
 *
 * It is reusable and thread-safe, allowing concurrent processing of multiple
 * requests without the need to create new parser instances.
 */
var ReadabilityParser = readability.NewParser()

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
