# Authentication transport acceptance matrix (2026-10-12)

Issue: [#339](https://github.com/ajbergh/OmniLLM-Studio/issues/339). The application has a first-party HttpOnly session cookie and a memory-only Wails bearer fallback. Explicit API bearer credentials remain supported. These are different transport contracts and must not be conflated.

## Automated, deterministic acceptance

| Mode | Coverage | Current scope |
| --- | --- | --- |
| Local HTTP API | Go router integration | Cookie scope and SameSite, session rotation, revoked/expired token, Origin/Fetch Metadata CSRF denial, API bearer, unauthorized SSE and cross-user denial |
| Direct HTTPS | Go `httptest.NewTLSServer` integration | Real TLS socket and Secure session cookie; same origin, logout and API bearer behaviors above |
| TLS-terminating reverse proxy | HTTPS ingress + backend HTTP via `httputil.ReverseProxy` | Proxy overwrites `X-Forwarded-Proto` to HTTPS; authenticates through cookie and protects mutation Origin against the HTTPS public Host |
| Vite SPA | Existing Playwright smoke + `authCookie.test.ts` | Proxy Origin allowlist; cookie credential requests with no retained browser bearer; not a substitute for a full browser matrix |
| Wails HTTP transport | New `desktopSession.test.ts` | Dynamic Wails API-base binding, memory-only bearer on requests, logout local invalidation even if offline, restart cannot resurrect bearer |
| Raw API bearer | Real Go router integration | Token rotation/revocation, separate cross-origin bearer client and invalid bearer cannot fall back to a valid cookie |

### Boundary and deployment requirements

- Expose the HTTPS proxy as the public origin. Configure the ingress to **overwrite**, not trust client-provided `X-Forwarded-Proto`; ensure forwarded Host corresponds to the public origin. Restrict direct backend exposure. `requestIsHTTPS` currently honors this header, so any proxy deploying it must sanitize it at the trust boundary.
- Do not set `OMNILLM_CORS_ORIGINS=*` for cookie authentication. Explicitly list SPA origins; a different local port is not the same origin. Preserve HttpOnly, SameSite=Lax, `Path=/v1`, and host-only cookies; Secure must be set when the public transport is HTTPS.
- Cookie-based browser mutations require trusted Origin. Legitimate non-browser bearer API clients may use another origin, but an **invalid explicit bearer must not authenticate with a valid cookie**.
- Never persist Wails capability URLs or bearer tokens to localStorage. Wails desktop's bearer is valid for the current frontend runtime only; a reload requires authentication or a separate explicitly designed refresh strategy.

### Acceptance not yet claimed

This slice does **not** launch a packaged Wails binary, exercise all browser engines through an external HTTPS proxy, prove secure cookie behavior on every supported OS WebView, or run successful audio/video/upload/SSE workloads after a full desktop reboot. Those tests remain open in #339. The TLS reverse proxy test is real HTTPS ingress into an in-process Go HTTP backend, not a production load balancer or certificate issuance test.

Run (from `backend`): `go test ./internal/api -run TestAuthDeploymentTransportMatrix -count=1 -v`.

Run (from `frontend`): `npm run test:unit -- src/desktopSession.test.ts src/authCookie.test.ts`.

Exact-head CI Quality Gate, Security Scan and sandbox tests must pass before merge.
