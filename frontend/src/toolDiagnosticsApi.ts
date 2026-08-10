import { getAuthToken, resolveApiUrl } from './api';

export interface ToolMetricSummary {
  tool_name: string;
  calls: number;
  successes: number;
  failures: number;
  timeouts: number;
  cancellations: number;
  retries: number;
  total_duration_ms: number;
  last_duration_ms: number;
  last_event: string;
}

export interface ToolInvocationSummary {
  id: string;
  tool_call_id: string;
  tool_name: string;
  status: string;
  approval_status: string;
  conversation_id?: string;
  run_id?: string;
  duration_ms: number;
  result_bytes: number;
  retry_count: number;
  created_at: string;
  started_at?: string;
  completed_at?: string;
}

export interface ToolDiagnosticsResponse {
  scope: 'user';
  metrics: ToolMetricSummary[];
  invocations: ToolInvocationSummary[];
}

function headers(): Record<string, string> {
  const out: Record<string, string> = {};
  const token = getAuthToken();
  if (token) out.Authorization = `Bearer ${token}`;
  return out;
}

export const toolDiagnosticsApi = {
  get: async (options?: { limit?: number; toolName?: string; status?: string }): Promise<ToolDiagnosticsResponse> => {
    const params = new URLSearchParams();
    if (options?.limit) params.set('limit', String(options.limit));
    if (options?.toolName) params.set('tool_name', options.toolName);
    if (options?.status) params.set('status', options.status);
    const query = params.toString();
    const response = await fetch(resolveApiUrl(`/v1/tools/diagnostics${query ? `?${query}` : ''}`), {
      headers: headers(),
      credentials: 'include',
    });
    const body = await response.json().catch(() => ({ error: response.statusText }));
    if (!response.ok) throw new Error(body.error || `Request failed (${response.status})`);
    return body as ToolDiagnosticsResponse;
  },
};
