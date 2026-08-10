import { getAuthToken, resolveApiUrl } from './api';

export interface OpenAPIToolInfo {
  name: string;
  operation_id: string;
  method: string;
  path: string;
  description?: string;
  policy?: 'allow' | 'ask' | 'deny';
}

export interface OpenAPIServer {
  id?: string;
  owner_user_id?: string;
  name: string;
  base_url: string;
  spec_json: string;
  enabled: boolean;
  allow_private_network: boolean;
  auth_header?: string;
  auth_prefix?: string;
  api_key?: string;
  has_api_key?: boolean;
  tools?: OpenAPIToolInfo[];
  created_at?: string;
  updated_at?: string;
}

function headers(): Record<string,string> {
  const h: Record<string,string> = { 'Content-Type': 'application/json' };
  const token = getAuthToken(); if (token) h.Authorization = `Bearer ${token}`; return h;
}
async function request<T>(path:string, init?:RequestInit):Promise<T>{ const response=await fetch(resolveApiUrl(`/v1${path}`),{...init,headers:{...headers(),...(init?.headers||{})},credentials:'include'});if(response.status===204)return undefined as T;const body=await response.json().catch(()=>({error:response.statusText}));if(!response.ok)throw new Error(body.error||`Request failed (${response.status})`);return body as T; }

export const openApiServersApi = {
  list: () => request<OpenAPIServer[]>('/openapi/servers'),
  save: (server: OpenAPIServer) => request<OpenAPIServer>('/openapi/servers', { method: 'PUT', body: JSON.stringify(server) }),
  refresh: (id: string) => request<OpenAPIServer>(`/openapi/servers/${encodeURIComponent(id)}/refresh`, { method: 'POST' }),
  delete: (id: string) => request<void>(`/openapi/servers/${encodeURIComponent(id)}`, { method: 'DELETE' }),
};
