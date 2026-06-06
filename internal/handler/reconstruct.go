package handler

import (
	"net/http"
	"net/url"
)

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
