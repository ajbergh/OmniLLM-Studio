import { useCallback, useEffect, useMemo, useState } from 'react';
import { AlertTriangle, Check, ChevronDown, ChevronUp, ShieldCheck, X } from 'lucide-react';
import { toast } from 'sonner';
import { agentRuntimeApi, type ApprovedToolResult, type PendingToolApproval } from '../agentRuntimeApi';
import { useConversationStore, useMessageStore, useSettingsStore } from '../stores';

interface CompletedApproval {
  approval: PendingToolApproval;
  result: ApprovedToolResult;
}

function prettyArguments(value: unknown): string {
  try {
    return JSON.stringify(value ?? {}, null, 2);
  } catch {
    return '{}';
  }
}

export function ToolApprovalCenter() {
  const [approvals, setApprovals] = useState<PendingToolApproval[]>([]);
  const [expanded, setExpanded] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [editedArguments, setEditedArguments] = useState<Record<string, string>>({});
  const [completed, setCompleted] = useState<CompletedApproval | null>(null);
  const selectConversation = useConversationStore((state) => state.selectConversation);
  const setAppMode = useSettingsStore((state) => state.setAppMode);

  const refresh = useCallback(async () => {
    try {
      const next = await agentRuntimeApi.listApprovals();
      setApprovals(next || []);
      setEditedArguments((current) => {
        const updated = { ...current };
        for (const approval of next || []) {
          if (updated[approval.id] === undefined) {
            updated[approval.id] = prettyArguments(approval.request.arguments);
          }
        }
        return updated;
      });
    } catch {
      // Auth may not be initialized yet. Polling will retry after login/startup.
    }
  }, []);

  useEffect(() => {
    void refresh();
    const timer = window.setInterval(() => {
      if (document.visibilityState === 'visible' && !busyId) void refresh();
    }, 3000);
    return () => window.clearInterval(timer);
  }, [busyId, refresh]);

  const active = approvals[0];
  const riskLabel = useMemo(() => {
    const risk = active?.request.risk || 'low';
    return risk.charAt(0).toUpperCase() + risk.slice(1);
  }, [active]);

  const resolve = async (approval: PendingToolApproval, approved: boolean) => {
    setBusyId(approval.id);
    try {
      let args: unknown = undefined;
      const raw = editedArguments[approval.id];
      if (approved && raw?.trim()) {
        try {
          args = JSON.parse(raw);
        } catch {
          toast.error('Tool arguments must be valid JSON');
          return;
        }
      }
      const response = await agentRuntimeApi.resolveApproval(approval.id, approved, args);
      setApprovals((items) => items.filter((item) => item.id !== approval.id));
      if (!approved) {
        toast.success('Tool call rejected');
        return;
      }
      if (response.result) {
        setCompleted({ approval, result: response.result });
        toast.success(response.result.is_error ? 'Approved tool returned an error' : 'Approved tool completed');
      } else {
        toast.success('Approval granted — continuing the original chat turn');
      }
    } catch (error) {
      toast.error((error as Error).message);
    } finally {
      setBusyId(null);
      void refresh();
    }
  };

  const continueInChat = () => {
    if (!completed) return;
    const conversationId = completed.approval.request.scope?.conversation_id;
    if (!conversationId) {
      navigator.clipboard.writeText(completed.result.content).catch(() => {});
      toast.success('Tool result copied');
      setCompleted(null);
      return;
    }
    setAppMode('chat');
    selectConversation(conversationId);
    const toolName = completed.approval.request.tool_name;
    const result = completed.result.content.slice(0, 120000);
    useMessageStore.getState().sendMessage(
      conversationId,
      `The previously approved tool \`${toolName}\` has completed. Continue the original request using this exact tool result. Do not call the same tool again unless the result explicitly requires another call.\n\n<approved_tool_result>\n${result}\n</approved_tool_result>`,
    );
    setCompleted(null);
  };

  if (!active && !completed) return null;

  return (
    <div className="fixed right-3 top-16 z-[90] w-[min(440px,calc(100vw-1.5rem))] rounded-2xl border border-amber-400/30 bg-surface-raised/95 shadow-2xl backdrop-blur">
      {completed ? (
        <div className="p-4">
          <div className="flex items-start gap-3">
            <div className="rounded-xl bg-emerald-500/15 p-2 text-emerald-400"><Check size={16} /></div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center justify-between gap-2">
                <div>
                  <p className="text-sm font-semibold text-text">Tool completed</p>
                  <p className="text-xs text-text-muted">{completed.approval.request.tool_name}</p>
                </div>
                <button onClick={() => setCompleted(null)} className="p-1 text-text-muted hover:text-text" aria-label="Dismiss tool result"><X size={14} /></button>
              </div>
              <pre className="mt-3 max-h-48 overflow-auto whitespace-pre-wrap rounded-xl border border-border bg-surface-alt p-3 text-xs text-text-secondary">{completed.result.content}</pre>
              <button onClick={continueInChat} className="mt-3 w-full rounded-xl bg-primary px-3 py-2 text-sm font-medium text-white hover:bg-primary-hover">
                Continue in chat
              </button>
            </div>
          </div>
        </div>
      ) : active ? (
        <div className="p-4">
          <div className="flex items-start gap-3">
            <div className="rounded-xl bg-amber-500/15 p-2 text-amber-400"><ShieldCheck size={16} /></div>
            <div className="min-w-0 flex-1">
              <div className="flex items-start justify-between gap-2">
                <div>
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-semibold text-text">Approval required</p>
                    <span className="rounded-full border border-amber-400/30 bg-amber-400/10 px-2 py-0.5 text-[10px] font-medium text-amber-300">{riskLabel} risk</span>
                    {approvals.length > 1 && <span className="text-[10px] text-text-muted">+{approvals.length - 1}</span>}
                  </div>
                  <p className="mt-1 text-xs font-medium text-text-secondary">{active.request.tool_name}</p>
                  <p className="mt-1 text-xs text-text-muted">{active.request.description}</p>
                </div>
                <AlertTriangle size={15} className="shrink-0 text-amber-400" />
              </div>

              <button onClick={() => setExpanded((value) => !value)} className="mt-3 inline-flex items-center gap-1 text-xs text-text-muted hover:text-text">
                {expanded ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
                Review arguments
              </button>
              {expanded && (
                <textarea
                  value={editedArguments[active.id] || ''}
                  onChange={(event) => setEditedArguments((current) => ({ ...current, [active.id]: event.target.value }))}
                  rows={7}
                  spellCheck={false}
                  className="mt-2 w-full resize-y rounded-xl border border-border bg-surface-alt p-3 font-mono text-xs text-text-secondary outline-none focus:border-primary/50"
                />
              )}

              <div className="mt-3 grid grid-cols-2 gap-2">
                <button disabled={busyId === active.id} onClick={() => void resolve(active, false)} className="rounded-xl border border-border px-3 py-2 text-sm text-text-secondary hover:border-red-400/40 hover:text-red-300 disabled:opacity-50">
                  Reject
                </button>
                <button disabled={busyId === active.id} onClick={() => void resolve(active, true)} className="rounded-xl bg-amber-500 px-3 py-2 text-sm font-semibold text-black hover:bg-amber-400 disabled:opacity-50">
                  {busyId === active.id ? 'Running…' : 'Approve and run'}
                </button>
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
