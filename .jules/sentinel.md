## 2024-08-05 - Add User-Agent to Outgoing Requests

**Vulnerability:** The application's HTTP client did not set a `User-Agent` header on outgoing requests. This could lead to service denials from websites that block default Go HTTP client requests, and it also allows for server fingerprinting.

**Learning:** Omitting the `User-Agent` header is a common oversight that can make an application's requests appear illegitimate to other services. It also unnecessarily reveals the underlying technology (the Go HTTP client) to the target server.

**Prevention:** Always set a descriptive `User-Agent` header on all outgoing HTTP requests. This header should ideally include information that allows the receiving server to identify the client, for example, by providing a link to the project's source code.

## 2024-08-06 - Prevent SSRF via HTTP Redirects
**Vulnerability:** The application was vulnerable to Server-Side Request Forgery (SSRF) because it did not validate the URL of HTTP redirects. An attacker could provide a URL that redirects to an internal or private IP address, bypassing the initial URL validation.
**Learning:** It is crucial to validate not only the initial URL but also any subsequent URLs in a redirect chain. Disabling automatic redirects and handling them manually allows for proper validation at each step.
**Prevention:** Always disable automatic redirects in HTTP clients and implement a manual redirect handling mechanism that re-validates each redirect URL against the same security policies as the initial URL.
