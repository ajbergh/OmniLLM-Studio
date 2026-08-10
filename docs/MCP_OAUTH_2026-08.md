# MCP OAuth 2.1 foundation

OmniLLM-Studio supports standards-aligned OAuth authorization for remote HTTP MCP servers using preregistered OAuth clients or Client ID Metadata Documents (CIMD).

## Security and protocol contract

The client discovers OAuth configuration instead of accepting arbitrary token endpoints from the UI:

1. Discover Protected Resource Metadata for the configured MCP resource (RFC 9728 / MCP authorization flow).
2. Discover authorization-server metadata (RFC 8414, with OpenID Connect discovery compatibility).
3. Verify the discovered issuer with exact RFC 8414 comparison, require PKCE `S256`, and validate RFC 9207 `iss` on authorization responses when present/advertised.
4. Bind both authorization and token requests to the canonical MCP resource using the `resource` parameter.
5. Send resulting Bearer access tokens only in the HTTP `Authorization` header.
6. Encrypt preregistered client secrets and access/refresh tokens at rest.
7. Refresh expiring tokens through the discovered token endpoint with the same resource binding.

All metadata and token traffic uses the V47 hardened MCP HTTP transport: public-network-only by default, DNS-aware private-address blocking, explicit private/local opt-in, bounded response bodies, and no redirects.

## Client registration

The current MCP specification prioritizes preregistered credentials when available, then Client ID Metadata Documents (CIMD). Omni also supports Dynamic Client Registration only as the deprecated fallback when no client is configured and the authorization server advertises `registration_endpoint`. Automatic DCR requests a public PKCE client (`token_endpoint_auth_method=none`) and binds the returned client ID to the exact issuer. Configure the registration method in **Settings → MCP → OAuth 2.1 authorization**.

Preregistered credentials are bound to the exact authorization-server issuer that first validates them. If Protected Resource Metadata later points to a different issuer, Omni rejects reuse instead of silently sending credentials to the new server. CIMD client IDs are HTTPS metadata-document URLs and remain issuer-portable by design.

The current supported token endpoint authentication methods are:

- `none` (public client)
- `client_secret_basic`
- `client_secret_post`

## Redirect URIs

Server/web deployments use `OMNILLM_MCP_OAUTH_REDIRECT_URI`. If it is unset, the local server default is `http://127.0.0.1:<OMNILLM_PORT>/v1/mcp/oauth/callback`.

The Wails desktop app binds a random loopback port at launch and sets its callback to `http://127.0.0.1:<random-port>/v1/mcp/oauth/callback`. That callback is the only loopback API route exposed outside the per-launch desktop capability path; one-time high-entropy OAuth `state` protects the callback. Native-app OAuth registrations should permit loopback redirect ports.

## Connection lifecycle

- **Connect OAuth** creates one-time state + PKCE material and opens the discovered authorization endpoint.
- The callback exchanges the authorization code, stores encrypted tokens, and reconnects the MCP server.
- Access tokens refresh automatically shortly before expiry when a refresh token is available.
- **Disconnect** clears access and refresh tokens and stops the active MCP runtime.
- Reconfiguring the OAuth client invalidates previously issued local tokens.

## Incremental authorization step-up

When a remote MCP operation returns `403` with `WWW-Authenticate: Bearer error="insufficient_scope"`, Omni persists the normalized union of the granted/requested scopes and the newly challenged scopes. Settings shows the pending scope set and offers **Grant additional scopes**, which starts a fresh resource-bound PKCE authorization flow. A successful token grant clears the pending requirement.

Omni does not silently replay a failed tool call after browser authorization; the original action remains failed and should be retried by the user/agent after authorization completes. This avoids hidden side effects and retry loops across an interactive consent boundary.
