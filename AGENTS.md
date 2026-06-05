# Agent Maintenance Guide

This document contains instructions for AI agents or developers maintaining this project.

## Architecture / Where To Find Things

- `api/index.go` -> Main entrypoint, HTTP request pipeline, output rendering.
- `api/index_test.go` -> Main integration tests, SSRF checks, orchestration tests.
- `api/llm_test.go` -> Format negotiation and LLM bot detection tests.
- `api/reconstruct_test.go` -> Vercel query-string rewriting edge cases.

## User-Agents (Spoofing)

The project uses a pool of User-Agents in `api/index.go` to bypass bot detection.

**MAINTENANCE TASK:** Periodically check if the User-Agents in `userAgentPool` are becoming outdated. Sites often block versions that are several months old to prevent scraping.

When updating:

- Use recent stable versions of Chrome, Firefox, Safari, and Edge.
- Include a mix of Operating Systems (Windows, macOS, Linux, iOS).
- Ensure headers like `Sec-Ch-Ua` (if added) match the User-Agent version.
