## 2024-08-05 - Add User-Agent to Outgoing Requests

**Vulnerability:** The application's HTTP client did not set a `User-Agent` header on outgoing requests. This could lead to service denials from websites that block default Go HTTP client requests, and it also allows for server fingerprinting.

**Learning:** Omitting the `User-Agent` header is a common oversight that can make an application's requests appear illegitimate to other services. It also unnecessarily reveals the underlying technology (the Go HTTP client) to the target server.

**Prevention:** Always set a descriptive `User-Agent` header on all outgoing HTTP requests. This header should ideally include information that allows the receiving server to identify the client, for example, by providing a link to the project's source code.

## 2026-01-20 - Fix SSRF Bypass via Unspecified IP (0.0.0.0)

**Vulnerability:** The SSRF protection mechanism relied on `net.Dialer` checking for `IsLoopback` and `IsPrivate`. However, it failed to block "Unspecified" addresses like `0.0.0.0` or `::`. In many environments, `net.Dial` resolves `0.0.0.0` to localhost, allowing an attacker to bypass the protection and access internal services by requesting `http://0.0.0.0:PORT`.

**Learning:** `ip.IsPrivate()` and `ip.IsLoopback()` are not sufficient to block all local traffic. The concept of "Unspecified" addresses (all zeros) must also be explicitly handled when validating IPs for SSRF protection in Go.

**Prevention:** When implementing a safe dialer to prevent SSRF, always include `ip.IsUnspecified()` in the list of blocked IP characteristics, in addition to private, loopback, and link-local addresses.

- 2025-02-20: [XSS] Always sanitize HTML output from readability libraries with bluemonday before rendering.
