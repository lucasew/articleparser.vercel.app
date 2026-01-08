## 2024-08-05 - Add User-Agent to Outgoing Requests

**Vulnerability:** The application's HTTP client did not set a `User-Agent` header on outgoing requests. This could lead to service denials from websites that block default Go HTTP client requests, and it also allows for server fingerprinting.

**Learning:** Omitting the `User-Agent` header is a common oversight that can make an application's requests appear illegitimate to other services. It also unnecessarily reveals the underlying technology (the Go HTTP client) to the target server.

**Prevention:** Always set a descriptive `User-Agent` header on all outgoing HTTP requests. This header should ideally include information that allows the receiving server to identify the client, for example, by providing a link to the project's source code.
