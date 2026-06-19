## IGNORE: XSS Sanitization with bluemonday

**- Pattern:** Adding the `github.com/microcosm-cc/bluemonday` library to sanitize the HTML output of `go-readability` against XSS.
**- Justification:** This approach is consistently rejected. The project handles XSS sanitization differently, and adding `bluemonday` as a dependency for this specific purpose is not accepted.
**- Files Affected:** `api/index.go`, `api/xss_test.go`, `go.mod`, `go.sum`

## IGNORE: Fragmenting API Handler

**- Pattern:** Splitting the monolithic `api/index.go` into multiple `.go` files inside the `api/` directory (e.g., `api/client.go`, `api/fetch.go`) or into fragmented internal packages (e.g., `internal/article/`, `internal/formatter/`).
**- Justification:** Vercel treats each `.go` file in the `api/` directory as an independent serverless function, causing compilation errors when shared code is split there. Furthermore, business logic must be consolidated into a single `internal/handler/` package, not scattered across multiple arbitrary internal packages.
**- Files Affected:** `api/*.go`, `internal/*/*.go`

## IGNORE: Explicit Subtasks in mise.toml

**- Pattern:** Defining explicit subtasks with `run` commands (e.g., `[tasks."test:go"] run = "go test ./..."`) in `mise.toml`.
**- Justification:** In `mise.toml`, tasks like `lint`, `fmt`, `install`, `test`, and `codegen` must depend only on wildcards (e.g., `["lint:*"]`) and never explicitly list subtasks or define `run` commands directly.
**- Files Affected:** `mise.toml`

## IGNORE: No Release/Artifact Steps in CI for Vercel

**- Pattern:** Adding `Release` or `Artifacts` jobs/steps to the `.github/workflows/autorelease.yml` file.
**- Justification:** Vercel deployments manage their own releases. The `autorelease.yml` workflow should strictly handle CI checks (lint/test) and tag creation, but not artifact uploads or GitHub Releases modification, as these are intentionally omitted for Vercel-based projects.
**- Files Affected:** `.github/workflows/autorelease.yml`
