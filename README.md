# article parser

Available at: https://articleparser.vercel.app/

It's a simple site that you give a URL and it gives you the article content of the URL.

This project is basically a duct tape of the following projects:

- [Go Readability](https://codeberg.org/readeck/go-readability) (`codeberg.org/readeck/go-readability/v2`): The actual article parser (Readeck fork of go-shiori).
- [godown](https://github.com/mattn/godown): Converter from HTML to Markdown. Used for Markdown output.
- Golang Standard library to fetch stuff.

## LLM Friendly

You can use this tool directly in your LLM prompts by prefixing any URL:
`https://articleparser.vercel.app/https://example.com/article`

It will automatically return **Markdown** when accessed by LLMs, or you can force a format:

- `/md/https://...` — Markdown
- `/txt/https://...` — Plain text
- `/json/https://...` — JSON

To deploy it just link the project to a Vercel project. Everything should magically work.
