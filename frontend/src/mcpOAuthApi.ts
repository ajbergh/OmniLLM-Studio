import { getAuthToken, resolveApiUrl } from './api';

export type MCPOAuthAuthMethod = 'none' | 'client_secret_basic' | 'client_secret_post';
export type MCPOAuthRegistrationMethod = 'preregistered' | 'cimd' | 'dcr';

export interface MCPOAuthStatus {
  server_id: string;
  configured: boolean;
  connected: boolean;
  client_id?: string;
  registration_method?: MCPOAuthRegistrationMethod;
  client_issuer?: string;
  has_client_secret: boolean;
  has_refresh_token: boolean;
  token_endpoint_auth_method?: MCPOAuthAuthMethod;
  scope?: string;
  required_scope?: string;
  expires_at?: string;
  authorization_server?: string;
  authorization_endpoint?: string;
  token_endpoint?: string;
  resource_metadata_url?: string;
  redirect_uri?: string;
}

export interface ConfigureMCPOAuthInput {
  client_id: string;
  client_secret?: string;
  token_endpoint_auth_method: MCPOAuthAuthMethod;
  registration_method: MCPOAuthRegistrationMethod;
}

export interface MCPOAuthAuthorizationStart {
  authorization_url: string;
  authorization_server: string;
  registration_method: MCPOAuthRegistrationMethod;
  scope?: string;
  redirect_uri: string;
}

function authHeaders(): Record<string, string> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = getAuthToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  return headers;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(resolveApiUrl(path), {
    ...init,
    credentials: 'include',
    headers: { ...authHeaders(), ...(init?.headers || {}) },
  });
  if (response.status === 204) return undefined as T;
  const body = await response.json().catch(() => ({ error: response.statusText }));
  if (!response.ok) throw new Error(body.error || `Request failed (${response.status})`);
  return body as T;
}

export const mcpOAuthApi = {
  status: (serverId: string) => request<MCPOAuthStatus>(`/v1/mcp/servers/${encodeURIComponent(serverId)}/oauth`),
  configure: (serverId: string, input: ConfigureMCPOAuthInput) =>
    request<MCPOAuthStatus>(`/v1/mcp/servers/${encodeURIComponent(serverId)}/oauth`, {
      method: 'PUT',
      body: JSON.stringify(input),
    }),
  start: (serverId: string) =>
    request<MCPOAuthAuthorizationStart>(`/v1/mcp/servers/${encodeURIComponent(serverId)}/oauth/start`, { method: 'POST' }),
  disconnect: (serverId: string) =>
    request<void>(`/v1/mcp/servers/${encodeURIComponent(serverId)}/oauth`, { method: 'DELETE' }),
};
