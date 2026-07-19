import { getAuthToken, resolveApiUrl } from './api';

export type ToolRisk = 'low' | 'medium' | 'high' | 'critical';
export type AgentProfile = 'chat' | 'research' | 'agent';

export interface InvocationScope {
  user_id?: string;
  workspace_id?: string;
  conversation_id?: string;
  message_id?: string;
  run_id?: string;
}

export interface PendingToolApproval {
  id: string;
  request: {
    approval_id?: string;
    tool_call_id: string;
    tool_name: string;
    description: string;
    arguments: unknown;
    scope?: InvocationScope;
    risk?: ToolRisk;
    read_only?: boolean;
  };
  status: 'pending' | 'resolved' | 'expired';
  created_at: string;
  expires_at: string;
}

export interface ApprovedToolResult {
  tool_call_id: string;
  content: string;
  is_error: boolean;
  structured?: unknown;
  artifacts?: Array<{
    id?: string;
    name?: string;
    mime_type?: string;
    url?: string;
    bytes?: number;
  }>;
  metadata?: Record<string, unknown>;
}

export interface ResolveApprovalResponse {
  approval_id: string;
  approved: boolean;
  result?: ApprovedToolResult | null;
}

export interface AgentRunBudgets {
  max_steps?: number;
  max_duration_ms?: number;
  max_model_calls?: number;
  max_tool_calls?: number;
  max_cost_usd?: number;
}

export interface AgentStreamEvent {
  type: string;
  data: string;
}

function requestHeaders(): Record<string, string> {
  const next: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = getAuthToken();
  if (token) next.Authorization = `Bearer ${token}`;
  return next;
}

async function toolAction<T>(body: Record<string, unknown>): Promise<T> {
  const response = await fetch(resolveApiUrl('/v1/tools/execute'), {
    method: 'POST',
    headers: requestHeaders(),
    credentials: 'include',
    body: JSON.stringify(body),
  });
  const payload = await response.json().catch(() => ({ error: response.statusText }));
  if (!response.ok) throw new Error(payload.error || `Tool action failed (${response.status})`);
  return payload as T;
}

function streamAgent(path: string, body: Record<string, unknown>, onEvent: (event: AgentStreamEvent) => void) {
  const controller = new AbortController();
  const promise = new Promise<void>((resolve, reject) => {
    fetch(resolveApiUrl(path), {
      method: 'POST',
      headers: requestHeaders(),
      credentials: 'include',
      body: JSON.stringify(body),
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) {
          const payload = await response.json().catch(() => ({ error: response.statusText }));
          throw new Error(payload.error || `Agent request failed (${response.status})`);
        }
        const reader = response.body?.getReader();
        if (!reader) throw new Error('No response body');
        const decoder = new TextDecoder();
        let buffer = '';
        let currentType = '';
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          buffer = lines.pop() || '';
          for (const line of lines) {
            if (line.startsWith('event: ')) currentType = line.slice(7).trim();
            if (line.startsWith('data: ')) onEvent({ type: currentType, data: line.slice(6) });
          }
        }
        resolve();
      })
      .catch((error) => {
        if (error?.name === 'AbortError') resolve();
        else reject(error);
      });
  });
  return { promise, abort: () => controller.abort() };
}

export const agentRuntimeApi = {
  listApprovals: () => toolAction<PendingToolApproval[]>({ action: 'list_approvals' }),

  resolveApproval: (approvalId: string, approved: boolean, args?: unknown) =>
    toolAction<ResolveApprovalResponse>({
      action: 'resolve_approval', approval_id: approvalId, approved,
      ...(args !== undefined ? { arguments: args } : {}),
    }),

  startRun: (
    conversationId: string,
    request: { goal: string; provider?: string; model?: string; profile?: AgentProfile; budgets?: AgentRunBudgets },
    onEvent: (event: AgentStreamEvent) => void,
  ) => streamAgent(`/v1/conversations/${encodeURIComponent(conversationId)}/agent/run`, request, onEvent),

  resumeRun: (runId: string, onEvent: (event: AgentStreamEvent) => void) =>
    streamAgent(`/v1/agent/runs/${encodeURIComponent(runId)}/resume`, {}, onEvent),

  pauseRun: async (runId: string) => {
    const response = await fetch(resolveApiUrl(`/v1/agent/runs/${encodeURIComponent(runId)}/cancel?mode=pause`), {
      method: 'POST', headers: requestHeaders(), credentials: 'include', body: '{}',
    });
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    if (!response.ok) throw new Error(payload.error || 'Pause failed');
    return payload as { paused: boolean };
  },
};
