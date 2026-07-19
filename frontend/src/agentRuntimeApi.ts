import { getAuthToken, resolveApiUrl } from './api';

export type ToolRisk = 'low' | 'medium' | 'high' | 'critical';

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

async function toolAction<T>(body: Record<string, unknown>): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = getAuthToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  const response = await fetch(resolveApiUrl('/v1/tools/execute'), {
    method: 'POST',
    headers,
    credentials: 'include',
    body: JSON.stringify(body),
  });
  const payload = await response.json().catch(() => ({ error: response.statusText }));
  if (!response.ok) {
    throw new Error(payload.error || `Tool action failed (${response.status})`);
  }
  return payload as T;
}

export const agentRuntimeApi = {
  listApprovals: () =>
    toolAction<PendingToolApproval[]>({ action: 'list_approvals' }),

  resolveApproval: (approvalId: string, approved: boolean, args?: unknown) =>
    toolAction<ResolveApprovalResponse>({
      action: 'resolve_approval',
      approval_id: approvalId,
      approved,
      ...(args !== undefined ? { arguments: args } : {}),
    }),
};
