package handler

import "math/rand"

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
