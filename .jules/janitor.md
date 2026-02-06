## 2024-08-01 - Fix compilation error from incomplete refactor

**Issue:** The `api/index.go` file contained two functions with the same name, `handler`, which is a compilation error in Go. One of the functions was an incomplete refactor that was not being used, and it called a non-existent function `getFormat`.
**Root Cause:** The presence of a duplicate function and a call to a non-existent function was the result of an incomplete refactoring effort. The developer likely intended to extract the format-determining logic into a separate `getFormat` function but failed to complete the task, leaving the old `handler` function in place and causing a build failure.
**Solution:** I removed the duplicate, incomplete `handler` function, created the missing `getFormat` function by extracting the format-determining logic from the old `handler`, and updated the main `handler` function to correctly call the new `getFormat` function. I also corrected the `formatter` call to use the correct buffer (`contentBuf`).
**Pattern:** When refactoring, it is important to ensure that all old code is removed and that all calls to new functions are correctly updated. Leaving behind remnants of old code can lead to compilation errors and make the codebase more difficult to understand and maintain.

## 2024-08-02 - Remove dead code and fix HTML rendering

**Issue:** The `renderArticle` function in `api/index.go` was unused, and the `formatHTML` function did not correctly render the full HTML template, resulting in a malformed response.
**Root Cause:** The `renderArticle` function was likely a remnant of a previous refactoring effort, and the `formatHTML` function was incomplete, failing to use the HTML template.
**Solution:** I removed the unused `renderArticle` function and refactored `formatHTML` to correctly execute the HTML template, ensuring a well-formed HTML response.
**Pattern:** Regularly scan for and remove dead code to improve maintainability. Ensure all output formatters correctly and completely render their expected content.

## 2026-01-20 - Extract URL reconstruction logic into helper

**Issue:** The `handler` function in `api/index.go` contained complex logic for reconstructing URLs split by Vercel rewrites, mixing request processing with low-level string manipulation.
**Root Cause:** The logic for handling Vercel's query parameter splitting was implemented inline within the main handler, increasing its cognitive load and complexity.
**Solution:** I extracted the URL reconstruction logic into a dedicated helper function `reconstructTargetURL`. This adheres to the Single Responsibility Principle and makes the main handler cleaner and easier to read.
**Pattern:** Extract complex, self-contained logic blocks from main handlers into helper functions to improve readability and testability.

## 2026-01-26 - Extract LLM user agents to package variable

**Issue:** The `isLLM` function contained a hardcoded list of user agent substrings, mixing configuration data with logic.
**Root Cause:** The list of LLM identifiers was defined inside the function scope.
**Solution:** I extracted the list into a package-level variable `llmUserAgents`.
**Pattern:** Separate configuration data (like lists of magic strings) from business logic to improve readability and maintainability.

- 2025-02-18: Remove redundant tests that validate copy-pasted logic instead of the actual function.
