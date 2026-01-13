# Agent Maintenance Guide

This document contains instructions for AI agents or developers maintaining this project.

## User-Agents (Spoofing)

The project uses a pool of User-Agents in `api/index.go` to bypass bot detection. 

**MAINTENANCE TASK:** Periodically check if the User-Agents in `userAgentPool` are becoming outdated. Sites often block versions that are several months old to prevent scraping. 

When updating:
- Use recent stable versions of Chrome, Firefox, Safari, and Edge.
- Include a mix of Operating Systems (Windows, macOS, Linux, iOS).
- Ensure headers like `Sec-Ch-Ua` (if added) match the User-Agent version.
