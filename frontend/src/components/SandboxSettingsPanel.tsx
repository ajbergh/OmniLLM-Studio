import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  FolderPlus,
  HardDrive,
  LockKeyhole,
  RefreshCw,
  ShieldCheck,
  Trash2,
} from 'lucide-react';
import { toast } from 'sonner';
import {
  sandboxApi,
  type SandboxMountMode,
  type SandboxRuntimeCapabilities,
  type SandboxRuntimeStatus,
  type SandboxWorkspace,
  type SandboxWorkspaceChange,
} from '../sandboxApi';

type DesktopBridge = Window & {
  go?: {
    main?: {
      App?: {
        SelectSandboxWorkspace?: () => Promise<string>;
      };
    };
  };
};

const capabilityLabels: Array<[keyof SandboxRuntimeCapabilities, string]> = [
  ['os_isolation', 'OS isolation'],
  ['filesystem_isolation', 'Filesystem isolation'],
  ['network_isolation', 'Network isolation'],
  ['network_allowlist', 'Destination allowlist'],
  ['process_tree_isolation', 'Process tree isolation'],
  ['memory_limit', 'Memory quota'],
  ['cpu_limit', 'CPU quota'],
  ['pid_limit', 'PID quota'],
  ['disk_limit', 'Disk quota'],
];

function displayMode(mode: SandboxMountMode): string {
  if (mode === 'read_only') return 'Read only';
  if (mode === 'read_write_no_delete') return 'Read/write, no delete';
  return 'Read/write';
}

function displayDate(value?: string): string {
  if (!value) return '—';
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
}

function basename(path: string): string {
  const trimmed = path.replace(/[\\/]+$/, '');
  return trimmed.split(/[\\/]/).pop() || 'Selected folder';
}

function defaultWorkspaceID(path: string): string {
  const slug = basename(path)
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return slug || 'workspace';
}

function CapabilityBadge({ enabled, label }: { enabled: boolean; label: string }) {
  return (
    <div className="flex items-center justify-between gap-2 rounded-lg border border-border bg-surface/50 px-2.5 py-2">
      <span className="text-[11px] text-text-secondary">{label}</span>
      <span
        className={`rounded-md px-1.5 py-0.5 text-[10px] font-semibold ${
          enabled ? 'bg-emerald-500/15 text-emerald-300' : 'bg-slate-500/15 text-slate-400'
        }`}
      >
        {enabled ? 'Enforced' : 'Unavailable'}
      </span>
    </div>
  );
}

export function SandboxSettingsPanel() {
  const [status, setStatus] = useState<SandboxRuntimeStatus | null>(null);
  const [workspaces, setWorkspaces] = useState<SandboxWorkspace[]>([]);
  const [changes, setChanges] = useState<SandboxWorkspaceChange[]>([]);
  const [selectedWorkspaceID, setSelectedWorkspaceID] = useState('');
  const [selectedPath, setSelectedPath] = useState('');
  const [workspaceID, setWorkspaceID] = useState('');
  const [mode, setMode] = useState<SandboxMountMode>('read_write_no_delete');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [changesLoading, setChangesLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [runtimeStatus, workspaceList] = await Promise.all([
        sandboxApi.status(),
        sandboxApi.workspaces(),
      ]);
      setStatus(runtimeStatus);
      setWorkspaces(workspaceList);
      setSelectedWorkspaceID((current) => {
        if (current && workspaceList.some((item) => item.id === current)) return current;
        return workspaceList[0]?.id ?? '';
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to load sandbox settings');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!selectedWorkspaceID) {
      setChanges([]);
      return;
    }
    let cancelled = false;
    setChangesLoading(true);
    sandboxApi
      .workspaceChanges(selectedWorkspaceID, 30)
      .then((items) => {
        if (!cancelled) setChanges(items);
      })
      .catch((error) => {
        if (!cancelled) toast.error(error instanceof Error ? error.message : 'Failed to load workspace changes');
      })
      .finally(() => {
        if (!cancelled) setChangesLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [selectedWorkspaceID]);

  const selectedWorkspace = useMemo(
    () => workspaces.find((item) => item.id === selectedWorkspaceID) ?? null,
    [selectedWorkspaceID, workspaces],
  );

  const selectFolder = async () => {
    const picker = (window as DesktopBridge).go?.main?.App?.SelectSandboxWorkspace;
    if (!picker) {
      toast.error('Native folder selection is available only in the desktop app');
      return;
    }
    try {
      const path = await picker();
      if (!path) return;
      setSelectedPath(path);
      setWorkspaceID(defaultWorkspaceID(path));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to select workspace folder');
    }
  };

  const createWorkspace = async () => {
    const id = workspaceID.trim();
    if (!selectedPath || !id) return;
    setSaving(true);
    try {
      const created = await sandboxApi.createWorkspace({
        id,
        root_path: selectedPath,
        mode,
      });
      // The physical path is intentionally ephemeral frontend state. Clear it
      // as soon as the backend has converted it into an opaque owner grant.
      setSelectedPath('');
      setWorkspaceID('');
      setWorkspaces((items) => [created, ...items.filter((item) => item.id !== created.id)]);
      setSelectedWorkspaceID(created.id);
      toast.success('Sandbox workspace grant saved');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to save sandbox workspace');
    } finally {
      setSaving(false);
    }
  };

  const revokeWorkspace = async (id: string) => {
    if (!window.confirm(`Revoke sandbox workspace “${id}”? Files on disk will not be deleted.`)) return;
    try {
      await sandboxApi.removeWorkspace(id);
      setWorkspaces((items) => items.filter((item) => item.id !== id));
      setSelectedWorkspaceID((current) => (current === id ? '' : current));
      toast.success('Sandbox workspace grant revoked');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to revoke sandbox workspace');
    }
  };

  const capabilities = status?.capabilities;

  return (
    <div className="space-y-4">
      <div className="rounded-2xl border border-border bg-surface-alt p-5">
        <div className="flex items-start justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-emerald-500/20 to-sky-500/20 shadow-md shadow-emerald-500/10">
              <ShieldCheck size={18} className="text-emerald-300" />
            </div>
            <div className="min-w-0">
              <h3 className="text-sm font-bold">Agent Sandbox</h3>
              <p className="text-[11px] text-text-muted">
                Runtime enforcement, persistent extension policy, and owner-scoped filesystem grants
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={() => void load()}
            disabled={loading}
            className="shrink-0 rounded-lg p-2 text-text-muted transition-colors hover:bg-surface-hover hover:text-text disabled:opacity-50"
            aria-label="Refresh sandbox status"
            title="Refresh sandbox status"
          >
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          </button>
        </div>

        <div className="mt-4 grid gap-3 sm:grid-cols-3">
          <div className="rounded-xl border border-border bg-surface/50 p-3">
            <div className="text-[10px] font-semibold uppercase tracking-wider text-text-muted">Runtime</div>
            <div className="mt-1 text-sm font-semibold text-text">
              {status?.configured ? capabilities?.name || 'Configured' : 'Not configured'}
            </div>
            <div className="mt-1 text-[10px] text-text-muted">
              {capabilities?.version ? `Runtime version ${capabilities.version}` : 'Capability report from the active Broker'}
            </div>
          </div>
          <div className="rounded-xl border border-border bg-surface/50 p-3">
            <div className="text-[10px] font-semibold uppercase tracking-wider text-text-muted">Extensions</div>
            <div className="mt-1 text-sm font-semibold capitalize text-text">
              {status?.extension_sandbox_mode || 'unknown'}
            </div>
            <div className="mt-1 text-[10px] text-text-muted">Persistent plugin / stdio MCP sandbox policy</div>
          </div>
          <div className="rounded-xl border border-border bg-surface/50 p-3">
            <div className="text-[10px] font-semibold uppercase tracking-wider text-text-muted">Folder grants</div>
            <div className="mt-1 text-sm font-semibold text-text">{workspaces.length}</div>
            <div className="mt-1 text-[10px] text-text-muted">
              {status?.path_grant_available_here ? 'Native desktop grant flow available' : 'Read-only management from this client'}
            </div>
          </div>
        </div>

        {capabilities && (
          <div className="mt-4 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {capabilityLabels.map(([key, label]) => (
              <CapabilityBadge key={key} label={label} enabled={Boolean(capabilities[key])} />
            ))}
          </div>
        )}
      </div>

      <div className="rounded-2xl border border-border bg-surface-alt p-5">
        <div className="flex items-center gap-3">
          <HardDrive size={17} className="text-sky-300" />
          <div>
            <h3 className="text-sm font-bold">Filesystem Workspaces</h3>
            <p className="text-[11px] text-text-muted">Only opaque workspace IDs and relative paths are exposed to model tools.</p>
          </div>
        </div>

        {status?.path_grant_available_here && (
          <div className="mt-4 rounded-xl border border-border bg-surface/50 p-3">
            <div className="grid gap-3 sm:grid-cols-[auto_1fr_180px_auto] sm:items-end">
              <button
                type="button"
                onClick={() => void selectFolder()}
                className="flex min-h-10 items-center justify-center gap-2 rounded-xl border border-border px-3 text-xs font-medium text-text-secondary transition-colors hover:bg-surface-hover hover:text-text"
              >
                <FolderPlus size={14} /> Select folder
              </button>
              <label>
                <span className="mb-1 block text-[10px] font-medium text-text-muted">Workspace ID</span>
                <input
                  value={workspaceID}
                  onChange={(event) => setWorkspaceID(event.target.value)}
                  placeholder="project-name"
                  className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-xs text-text outline-none focus:border-primary/50"
                />
              </label>
              <label>
                <span className="mb-1 block text-[10px] font-medium text-text-muted">Access</span>
                <select
                  value={mode}
                  onChange={(event) => setMode(event.target.value as SandboxMountMode)}
                  className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-xs text-text"
                >
                  <option value="read_only">Read only</option>
                  <option value="read_write_no_delete">Read/write, no delete</option>
                  <option value="read_write">Read/write</option>
                </select>
              </label>
              <button
                type="button"
                onClick={() => void createWorkspace()}
                disabled={!selectedPath || !workspaceID.trim() || saving}
                className="min-h-10 rounded-xl bg-primary px-4 text-xs font-semibold text-white disabled:cursor-not-allowed disabled:opacity-40"
              >
                {saving ? 'Saving…' : 'Grant'}
              </button>
            </div>
            {selectedPath && (
              <div className="mt-2 flex items-center gap-1.5 text-[10px] text-text-muted">
                <LockKeyhole size={11} /> Selected locally: <span className="font-medium text-text-secondary">{basename(selectedPath)}</span>
              </div>
            )}
          </div>
        )}

        <div className="mt-4 grid gap-3 lg:grid-cols-[280px_1fr]">
          <div className="space-y-2">
            {workspaces.length === 0 ? (
              <div className="rounded-xl border border-dashed border-border px-3 py-5 text-center text-xs text-text-muted">
                No sandbox workspaces granted
              </div>
            ) : (
              workspaces.map((workspace) => (
                <button
                  key={workspace.id}
                  type="button"
                  onClick={() => setSelectedWorkspaceID(workspace.id)}
                  className={`w-full rounded-xl border p-3 text-left transition-colors ${
                    workspace.id === selectedWorkspaceID
                      ? 'border-primary/50 bg-primary/5'
                      : 'border-border bg-surface/40 hover:bg-surface-hover'
                  }`}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="truncate font-mono text-xs font-semibold text-text">{workspace.id}</span>
                    <span className="text-[9px] text-text-muted">{displayMode(workspace.mode)}</span>
                  </div>
                  <div className="mt-1 text-[9px] text-text-muted">Updated {displayDate(workspace.updated_at)}</div>
                </button>
              ))
            )}
          </div>

          <div className="min-w-0 rounded-xl border border-border bg-surface/40">
            <div className="flex items-center justify-between gap-3 border-b border-border px-3 py-2">
              <div>
                <div className="text-[10px] font-semibold uppercase tracking-wider text-text-muted">Recent changes</div>
                <div className="text-[10px] text-text-muted/70">
                  {selectedWorkspace ? `${selectedWorkspace.id} · ${displayMode(selectedWorkspace.mode)}` : 'Select a workspace'}
                </div>
              </div>
              {selectedWorkspace && (
                <button
                  type="button"
                  onClick={() => void revokeWorkspace(selectedWorkspace.id)}
                  className="rounded-lg p-2 text-text-muted transition-colors hover:bg-danger-soft hover:text-danger"
                  title="Revoke workspace grant"
                  aria-label={`Revoke ${selectedWorkspace.id} workspace grant`}
                >
                  <Trash2 size={13} />
                </button>
              )}
            </div>

            {changesLoading ? (
              <div className="flex items-center justify-center gap-2 px-4 py-8 text-xs text-text-muted">
                <RefreshCw size={13} className="animate-spin" /> Loading changes…
              </div>
            ) : changes.length === 0 ? (
              <div className="px-4 py-8 text-center text-xs text-text-muted">No recorded changes for this workspace</div>
            ) : (
              <div className="max-h-72 overflow-y-auto divide-y divide-border/70">
                {changes.map((change) => (
                  <div key={change.id} className="px-3 py-2.5">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="truncate font-mono text-[11px] text-text-secondary">{change.relative_path}</div>
                        <div className="mt-0.5 text-[10px] text-text-muted">
                          {change.operation} · {displayDate(change.created_at)}
                        </div>
                      </div>
                      <span
                        className={`shrink-0 rounded-md px-1.5 py-0.5 text-[9px] font-medium ${
                          change.revertable ? 'bg-emerald-500/15 text-emerald-300' : 'bg-slate-500/15 text-slate-400'
                        }`}
                      >
                        {change.revertable ? 'Revertable' : 'Audit only'}
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
