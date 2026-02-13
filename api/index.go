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
	"log"
	"net/http"
	"time"
)

const (
	handlerTimeout = 5 * time.Second
)

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
