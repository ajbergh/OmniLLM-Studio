# MCP OAuth 2.1 foundation

OmniLLM-Studio supports standards-aligned OAuth authorization for remote HTTP MCP servers using a preregistered OAuth client.

## Security and protocol contract

The client discovers OAuth configuration instead of accepting arbitrary token endpoints from the UI:

1. Discover Protected Resource Metadata for the configured MCP resource (RFC 9728 / MCP authorization flow).
2. Discover authorization-server metadata (RFC 8414, with OpenID Connect discovery compatibility).
3. Verify the discovered issuer and require PKCE `S256`.
4. Bind both authorization and token requests to the canonical MCP resource using the `resource` parameter.
5. Send resulting Bearer access tokens only in the HTTP `Authorization` header.
6. Encrypt preregistered client secrets and access/refresh tokens at rest.
7. Refresh expiring tokens through the discovered token endpoint with the same resource binding.

All metadata and token traffic uses the V47 hardened MCP HTTP transport: public-network-only by default, DNS-aware private-address blocking, explicit private/local opt-in, bounded response bodies, and no redirects.

## Preregistered clients

Phase A intentionally supports preregistered client IDs rather than guessing dynamic-registration behavior. Configure the client ID and token endpoint authentication method in **Settings → MCP → OAuth 2.1 authorization**. Confidential clients may store a client secret; it is encrypted and never returned by the API.

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

## Deliberate follow-on work

Client ID Metadata Documents / dynamic client registration and incremental authorization-scope UX are separate follow-on phases. Keeping them separate makes the initial authorization boundary easier to review and test.
