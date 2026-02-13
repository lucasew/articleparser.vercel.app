package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

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
