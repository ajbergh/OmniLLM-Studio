import { useCallback, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { Activity, CheckCircle2, Clock3, RefreshCw, RotateCcw, XCircle } from 'lucide-react';
import { toast } from 'sonner';
import { toolDiagnosticsApi, type ToolDiagnosticsResponse, type ToolInvocationSummary } from '../toolDiagnosticsApi';
import { SandboxSettingsPanel } from './SandboxSettingsPanel';

function statusClasses(status: string): string {
  if (status === 'tool_completed') return 'bg-emerald-500/15 text-emerald-300';
  if (status === 'tool_failed') return 'bg-red-500/15 text-red-300';
  if (status === 'tool_timed_out') return 'bg-amber-500/15 text-amber-300';
  if (status === 'tool_cancelled') return 'bg-slate-500/15 text-slate-300';
  return 'bg-primary/10 text-primary';
}

function formatStatus(status: string): string {
  return status.replace(/^tool_/, '').replaceAll('_', ' ');
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(value?: string): string {
  if (!value) return '—';
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
}

function StatCard({ label, value, detail, icon }: { label: string; value: string; detail: string; icon: ReactNode }) {
  return (
    <div className="rounded-xl border border-border bg-surface/50 p-3">
      <div className="flex items-center gap-2 text-[10px] font-semibold uppercase tracking-wider text-text-muted">
        {icon}
        {label}
      </div>
      <div className="mt-2 text-lg font-bold text-text">{value}</div>
      <div className="mt-0.5 text-[10px] text-text-muted">{detail}</div>
    </div>
  );
}

function InvocationRow({ invocation }: { invocation: ToolInvocationSummary }) {
  return (
    <tr className="border-t border-border/70 text-[11px]">
      <td className="px-3 py-2 font-mono text-text-secondary">{invocation.tool_name}</td>
      <td className="px-3 py-2">
        <span className={`rounded-md px-1.5 py-0.5 font-medium capitalize ${statusClasses(invocation.status)}`}>
          {formatStatus(invocation.status)}
        </span>
      </td>
      <td className="px-3 py-2 text-text-muted">{invocation.approval_status || '—'}</td>
      <td className="px-3 py-2 text-right tabular-nums text-text-secondary">{invocation.duration_ms} ms</td>
      <td className="px-3 py-2 text-right tabular-nums text-text-muted">{formatBytes(invocation.result_bytes)}</td>
      <td className="px-3 py-2 text-right tabular-nums text-text-muted">{invocation.retry_count}</td>
      <td className="px-3 py-2 whitespace-nowrap text-text-muted">{formatDate(invocation.created_at)}</td>
    </tr>
  );
}

export function ToolDiagnosticsPanel() {
  const [diagnostics, setDiagnostics] = useState<ToolDiagnosticsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [toolName, setToolName] = useState('');
  const [status, setStatus] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setDiagnostics(await toolDiagnosticsApi.get({ limit: 100, toolName: toolName || undefined, status: status || undefined }));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to load tool diagnostics');
    } finally {
      setLoading(false);
    }
  }, [status, toolName]);

  useEffect(() => {
    void load();
  }, [load]);

  const metrics = diagnostics?.metrics ?? [];
  const invocations = diagnostics?.invocations ?? [];
  const toolNames = useMemo(() => {
    const names = new Set<string>();
    metrics.forEach((item) => names.add(item.tool_name));
    invocations.forEach((item) => names.add(item.tool_name));
    return [...names].sort();
  }, [invocations, metrics]);

  const totals = useMemo(() => {
    const calls = metrics.reduce((sum, item) => sum + item.calls, 0);
    const successes = metrics.reduce((sum, item) => sum + item.successes, 0);
    const failures = metrics.reduce((sum, item) => sum + item.failures + item.timeouts + item.cancellations, 0);
    const retries = metrics.reduce((sum, item) => sum + item.retries, 0);
    const duration = metrics.reduce((sum, item) => sum + item.total_duration_ms, 0);
    return {
      calls,
      successes,
      failures,
      retries,
      successRate: calls > 0 ? Math.round((successes / calls) * 100) : 0,
      averageDuration: calls > 0 ? Math.round(duration / calls) : 0,
    };
  }, [metrics]);

  return (
    <div className="space-y-4">
      <SandboxSettingsPanel />

      <div className="rounded-2xl border border-border bg-surface-alt p-5">
        <div className="flex items-start justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-sky-500/20 to-violet-500/20 shadow-md shadow-sky-500/10">
              <Activity size={18} className="text-sky-300" />
            </div>
            <div className="min-w-0">
              <h3 className="text-sm font-bold">Tool Diagnostics</h3>
              <p className="text-[11px] text-text-muted">Privacy-safe runtime health and recent invocation history for your user scope</p>
            </div>
          </div>
          <button
            type="button"
            onClick={() => void load()}
            disabled={loading}
            className="shrink-0 rounded-lg p-2 text-text-muted transition-colors hover:bg-surface-hover hover:text-text disabled:opacity-50"
            aria-label="Refresh tool diagnostics"
            title="Refresh tool diagnostics"
          >
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          </button>
        </div>

        <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-4">
          <StatCard label="Calls" value={String(totals.calls)} detail={`${totals.failures} non-success`} icon={<Activity size={12} />} />
          <StatCard label="Success" value={`${totals.successRate}%`} detail={`${totals.successes} completed`} icon={<CheckCircle2 size={12} />} />
          <StatCard label="Retries" value={String(totals.retries)} detail="Transient replays" icon={<RotateCcw size={12} />} />
          <StatCard label="Avg latency" value={`${totals.averageDuration} ms`} detail="Current process" icon={<Clock3 size={12} />} />
        </div>

        <div className="mt-4 flex flex-wrap items-end gap-2">
          <label className="min-w-[150px] flex-1">
            <span className="mb-1 block text-[10px] font-medium text-text-muted">Tool</span>
            <select value={toolName} onChange={(event) => setToolName(event.target.value)} className="w-full rounded-lg border border-border bg-surface px-2.5 py-2 text-xs text-text">
              <option value="">All tools</option>
              {toolNames.map((name) => <option key={name} value={name}>{name}</option>)}
            </select>
          </label>
          <label className="min-w-[150px] flex-1">
            <span className="mb-1 block text-[10px] font-medium text-text-muted">Status</span>
            <select value={status} onChange={(event) => setStatus(event.target.value)} className="w-full rounded-lg border border-border bg-surface px-2.5 py-2 text-xs text-text">
              <option value="">All statuses</option>
              <option value="tool_completed">Completed</option>
              <option value="tool_failed">Failed</option>
              <option value="tool_timed_out">Timed out</option>
              <option value="tool_cancelled">Cancelled</option>
            </select>
          </label>
        </div>

        <div className="mt-4 overflow-x-auto rounded-xl border border-border bg-surface/40">
          <div className="flex items-center justify-between border-b border-border px-3 py-2">
            <div>
              <div className="text-[10px] font-semibold uppercase tracking-wider text-text-muted">Recent invocations</div>
              <div className="text-[10px] text-text-muted/70">Arguments, result bodies, and error text are never returned by this endpoint.</div>
            </div>
            {loading && <RefreshCw size={12} className="animate-spin text-text-muted" />}
          </div>
          {invocations.length === 0 && !loading ? (
            <div className="flex items-center justify-center gap-2 px-4 py-6 text-xs text-text-muted">
              <XCircle size={14} /> No matching tool invocations
            </div>
          ) : (
            <table className="min-w-[760px] w-full">
              <thead className="text-[10px] uppercase tracking-wider text-text-muted">
                <tr>
                  <th className="px-3 py-2 text-left font-medium">Tool</th>
                  <th className="px-3 py-2 text-left font-medium">Status</th>
                  <th className="px-3 py-2 text-left font-medium">Approval</th>
                  <th className="px-3 py-2 text-right font-medium">Latency</th>
                  <th className="px-3 py-2 text-right font-medium">Result</th>
                  <th className="px-3 py-2 text-right font-medium">Retries</th>
                  <th className="px-3 py-2 text-left font-medium">Created</th>
                </tr>
              </thead>
              <tbody>{invocations.map((item) => <InvocationRow key={item.id} invocation={item} />)}</tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  );
}
