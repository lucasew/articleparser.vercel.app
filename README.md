# article parser

Available at: https://articleparser.vercel.app/

It's a simple site that you give a URL and it gives you the article content of the URL.

This project is basically a duct tape of the following projects:
- [Go Readability](https://github.com/go-shiori/go-readability): The actual article parser.
- [godown](https://github.com/mattn/godown): Converter from HTML to Markdown. Not used in the site.
- Golang Standard library to fetch stuff.

To deploy it just link the project to a Vercel project. Everything should magically work.
