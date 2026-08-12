import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  AlertTriangle,
  ExternalLink,
  Github,
  Link2,
  Loader2,
  RefreshCw,
  Trash2,
  Unplug,
} from 'lucide-react';
import {
  bindGitHubRepository,
  deleteGitHubRepositoryBinding,
  disconnectGitHub,
  getGitHubAuthStatus,
  getGitHubRepositoryBindings,
  listGitHubRepositories,
  nextGitHubPollDelayMs,
  pollGitHubDeviceAuthorization,
  shouldContinueGitHubPolling,
  startGitHubDeviceAuthorization,
  type GitHubAuthStatus,
  type GitHubDeviceAuthorization,
  type GitHubRepository,
  type GitHubRepositoryBinding,
  type GitHubRepositoryBindingsResponse,
} from '../githubSettingsApi';

const REPOSITORIES_PER_PAGE = 30;

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'GitHub request failed';
}

function repositoryLabel(repository: GitHubRepository): string {
  const markers = [
    repository.private ? 'private' : 'public',
    repository.fork ? 'fork' : '',
    repository.archived ? 'archived' : '',
    repository.disabled ? 'disabled' : '',
  ].filter(Boolean);
  return `${repository.full_name} · ${markers.join(' · ')}`;
}

function formatExpiry(value?: string): string | null {
  if (!value) return null;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date.toLocaleString();
}

export function GitHubSettingsSection() {
  const [status, setStatus] = useState<GitHubAuthStatus | null>(null);
  const [device, setDevice] = useState<GitHubDeviceAuthorization | null>(null);
  const [bindings, setBindings] = useState<GitHubRepositoryBindingsResponse | null>(null);
  const [repositories, setRepositories] = useState<GitHubRepository[]>([]);
  const [repositoryPage, setRepositoryPage] = useState(0);
  const [hasMoreRepositories, setHasMoreRepositories] = useState(false);
  const [selectedRepositories, setSelectedRepositories] = useState<Record<string, string>>({});
  const [search, setSearch] = useState('');
  const [busy, setBusy] = useState<string | null>('loading');
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const timeoutRef = useRef<number | null>(null);
  const requestRef = useRef<AbortController | null>(null);
  const mountedRef = useRef(true);

  const clearPolling = useCallback(() => {
    if (timeoutRef.current !== null) {
      window.clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }
    requestRef.current?.abort();
    requestRef.current = null;
  }, []);

  const loadBindings = useCallback(async (signal?: AbortSignal) => {
    const next = await getGitHubRepositoryBindings(signal);
    if (!mountedRef.current) return;
    setBindings(next);
  }, []);

  const loadRepositoryPage = useCallback(async (page: number, append: boolean, signal?: AbortSignal) => {
    const next = await listGitHubRepositories(page, REPOSITORIES_PER_PAGE, signal);
    if (!mountedRef.current) return;
    setRepositories((current) => {
      const combined = append ? [...current, ...next.repositories] : next.repositories;
      const unique = new Map(combined.map((repository) => [repository.id, repository]));
      return [...unique.values()];
    });
    setRepositoryPage(next.page);
    setHasMoreRepositories(next.has_more);
  }, []);

  const refresh = useCallback(async (signal?: AbortSignal) => {
    setError(null);
    const nextStatus = await getGitHubAuthStatus(signal);
    if (!mountedRef.current) return;
    setStatus(nextStatus);

    if (!nextStatus.configured) {
      setBindings(null);
      setRepositories([]);
      setRepositoryPage(0);
      setHasMoreRepositories(false);
      return;
    }

    if (nextStatus.github_user_id) {
      await loadBindings(signal);
    } else {
      setBindings(null);
    }

    if (nextStatus.connected) {
      await loadRepositoryPage(1, false, signal);
    } else {
      setRepositories([]);
      setRepositoryPage(0);
      setHasMoreRepositories(false);
    }
  }, [loadBindings, loadRepositoryPage]);

  useEffect(() => {
    mountedRef.current = true;
    const controller = new AbortController();
    requestRef.current = controller;
    setBusy('loading');
    void refresh(controller.signal)
      .catch((nextError) => {
        if (!controller.signal.aborted) setError(errorMessage(nextError));
      })
      .finally(() => {
        if (mountedRef.current && requestRef.current === controller) {
          requestRef.current = null;
          setBusy(null);
        }
      });

    return () => {
      mountedRef.current = false;
      clearPolling();
      controller.abort();
    };
  }, [clearPolling, refresh]);

  const schedulePoll = useCallback((authorization: GitHubDeviceAuthorization, delayMs: number) => {
    if (!mountedRef.current) return;
    timeoutRef.current = window.setTimeout(() => {
      timeoutRef.current = null;
      const controller = new AbortController();
      requestRef.current = controller;
      void pollGitHubDeviceAuthorization(controller.signal)
        .then(async (result) => {
          if (!mountedRef.current || controller.signal.aborted) return;
          if (shouldContinueGitHubPolling(result.status)) {
            schedulePoll(authorization, nextGitHubPollDelayMs(result, authorization.interval_seconds));
            return;
          }
          setDevice(null);
          if (result.status === 'connected') {
            setNotice(`Connected${result.github_login ? ` as ${result.github_login}` : ''}.`);
            await refresh(controller.signal);
            return;
          }
          if (result.status === 'denied') {
            setError('GitHub authorization was denied. Start a new connection when ready.');
          } else if (result.status === 'expired' || result.status === 'not_started') {
            setError('GitHub authorization expired. Start a new connection to try again.');
          } else {
            setError(`GitHub authorization stopped with status: ${result.status}`);
          }
        })
        .catch((nextError) => {
          if (!controller.signal.aborted) setError(errorMessage(nextError));
        })
        .finally(() => {
          if (requestRef.current === controller) requestRef.current = null;
        });
    }, delayMs);
  }, [refresh]);

  const startConnection = useCallback(async () => {
    clearPolling();
    setBusy('connect');
    setError(null);
    setNotice(null);
    const controller = new AbortController();
    requestRef.current = controller;
    try {
      const authorization = await startGitHubDeviceAuthorization(controller.signal);
      if (!mountedRef.current) return;
      setDevice(authorization);
      setStatus((current) => current ? { ...current, pending: true } : current);
      schedulePoll(authorization, Math.max(1000, authorization.interval_seconds * 1000));
    } catch (nextError) {
      if (!controller.signal.aborted) setError(errorMessage(nextError));
    } finally {
      if (mountedRef.current && requestRef.current === controller) {
        requestRef.current = null;
        setBusy(null);
      }
    }
  }, [clearPolling, schedulePoll]);

  const disconnect = useCallback(async () => {
    clearPolling();
    setBusy('disconnect');
    setError(null);
    setNotice(null);
    try {
      await disconnectGitHub();
      if (!mountedRef.current) return;
      setDevice(null);
      setBindings(null);
      setRepositories([]);
      setSelectedRepositories({});
      setNotice('GitHub was disconnected from OmniLLM-Studio. This does not revoke the authorization at GitHub.');
      await refresh();
    } catch (nextError) {
      setError(errorMessage(nextError));
    } finally {
      if (mountedRef.current) setBusy(null);
    }
  }, [clearPolling, refresh]);

  const bindingByLocalRepository = useMemo(() => {
    const result = new Map<string, GitHubRepositoryBinding>();
    for (const binding of bindings?.bindings ?? []) result.set(binding.local_repository_id, binding);
    return result;
  }, [bindings]);

  const filteredRepositories = useMemo(() => {
    const needle = search.trim().toLowerCase();
    if (!needle) return repositories;
    return repositories.filter((repository) => repository.full_name.toLowerCase().includes(needle));
  }, [repositories, search]);

  const bindRepository = useCallback(async (localRepositoryId: string) => {
    const selected = Number(selectedRepositories[localRepositoryId]);
    if (!Number.isFinite(selected) || selected <= 0) return;
    setBusy(`bind:${localRepositoryId}`);
    setError(null);
    try {
      await bindGitHubRepository(localRepositoryId, selected);
      await loadBindings();
      setNotice(`Updated GitHub binding for ${localRepositoryId}.`);
    } catch (nextError) {
      setError(errorMessage(nextError));
    } finally {
      if (mountedRef.current) setBusy(null);
    }
  }, [loadBindings, selectedRepositories]);

  const removeBinding = useCallback(async (localRepositoryId: string) => {
    setBusy(`delete:${localRepositoryId}`);
    setError(null);
    try {
      await deleteGitHubRepositoryBinding(localRepositoryId);
      await loadBindings();
      setNotice(`Removed GitHub binding for ${localRepositoryId}.`);
    } catch (nextError) {
      setError(errorMessage(nextError));
    } finally {
      if (mountedRef.current) setBusy(null);
    }
  }, [loadBindings]);

  const loadMoreRepositories = useCallback(async () => {
    if (!hasMoreRepositories || busy) return;
    setBusy('load-more');
    setError(null);
    try {
      await loadRepositoryPage(Math.max(1, repositoryPage + 1), true);
    } catch (nextError) {
      setError(errorMessage(nextError));
    } finally {
      if (mountedRef.current) setBusy(null);
    }
  }, [busy, hasMoreRepositories, loadRepositoryPage, repositoryPage]);

  if (busy === 'loading' && !status) {
    return (
      <div className="flex items-center gap-2 text-sm text-[var(--color-text-muted)]">
        <Loader2 size={16} className="animate-spin" /> Loading GitHub connection…
      </div>
    );
  }

  if (!status?.configured) {
    return (
      <div className="space-y-3">
        <div className="flex items-start gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-tertiary)] p-4">
          <Github size={20} className="mt-0.5 shrink-0" />
          <div>
            <div className="font-medium">GitHub App connection is not configured</div>
            <div className="mt-1 text-sm text-[var(--color-text-muted)]">
              An administrator must configure the OmniLLM-Studio GitHub App client ID before users can connect accounts.
            </div>
          </div>
        </div>
        {error && <div className="text-sm text-[var(--color-error)]">{error}</div>}
      </div>
    );
  }

  const connectedIdentity = status.github_login || (status.github_user_id ? `GitHub user ${status.github_user_id}` : null);
  const expires = formatExpiry(status.expires_at);
  const staleBindings = (bindings?.bindings ?? []).filter((binding) => !binding.account_matches || !binding.local_configured);

  return (
    <div className="space-y-6">
      <section className="space-y-3">
        <div className="flex items-start justify-between gap-4">
          <div>
            <div className="flex items-center gap-2 font-medium"><Github size={18} /> GitHub connection</div>
            <div className="mt-1 text-sm text-[var(--color-text-muted)]">
              Connect a personal GitHub identity for repository discovery and request-scoped Git credentials.
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <button
              type="button"
              onClick={() => void startConnection()}
              disabled={busy !== null}
              className="flex items-center gap-1.5 rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm hover:bg-[var(--color-bg-hover)] disabled:opacity-50"
            >
              {busy === 'connect' ? <Loader2 size={14} className="animate-spin" /> : <RefreshCw size={14} />}
              {status.connected ? 'Reconnect' : 'Connect GitHub'}
            </button>
            {(status.github_user_id || status.connected) && (
              <button
                type="button"
                onClick={() => void disconnect()}
                disabled={busy !== null}
                className="flex items-center gap-1.5 rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm hover:bg-[var(--color-bg-hover)] disabled:opacity-50"
              >
                {busy === 'disconnect' ? <Loader2 size={14} className="animate-spin" /> : <Unplug size={14} />}
                Disconnect
              </button>
            )}
          </div>
        </div>

        {connectedIdentity && (
          <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-tertiary)] p-3 text-sm">
            <span className="font-medium">{status.connected ? 'Connected' : 'Reauthorization required'}:</span>{' '}
            {connectedIdentity}{expires ? ` · token expires ${expires}` : ''}
          </div>
        )}

        {device && (
          <div className="rounded-lg border border-[var(--color-accent)] bg-[var(--color-bg-tertiary)] p-4">
            <div className="text-sm font-medium">Complete authorization at GitHub</div>
            <div className="mt-3 flex flex-wrap items-center gap-3">
              <code className="rounded bg-[var(--color-bg-primary)] px-3 py-2 text-lg font-semibold tracking-widest">{device.user_code}</code>
              <a
                href={device.verification_uri}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1.5 text-sm text-[var(--color-accent)] hover:underline"
              >
                Open GitHub <ExternalLink size={14} />
              </a>
            </div>
            <div className="mt-2 text-xs text-[var(--color-text-muted)]">
              This code expires {formatExpiry(device.expires_at) ?? 'soon'}. OmniLLM-Studio checks GitHub only at the provider-required interval.
            </div>
          </div>
        )}

        {error && <div className="rounded-md bg-[var(--color-error)]/10 p-3 text-sm text-[var(--color-error)]">{error}</div>}
        {notice && <div className="rounded-md bg-[var(--color-success)]/10 p-3 text-sm text-[var(--color-success)]">{notice}</div>}
      </section>

      {(status.connected || status.github_user_id) && (
        <section className="space-y-4 border-t border-[var(--color-border)] pt-5">
          <div>
            <div className="flex items-center gap-2 font-medium"><Link2 size={18} /> Repository bindings</div>
            <div className="mt-1 text-sm text-[var(--color-text-muted)]">
              Map a GitHub repository to an administrator-configured local repository ID. Local filesystem paths are never shown here.
            </div>
          </div>

          {status.connected && (
            <input
              type="search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Filter loaded GitHub repositories…"
              className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-primary)] px-3 py-2 text-sm outline-none focus:border-[var(--color-accent)]"
            />
          )}

          {(bindings?.local_repositories ?? []).length === 0 ? (
            <div className="rounded-lg border border-[var(--color-border)] p-4 text-sm text-[var(--color-text-muted)]">
              No local Git repositories are configured for binding. An administrator can add stable repository IDs through the existing Git repository configuration.
            </div>
          ) : (
            <div className="space-y-3">
              {(bindings?.local_repositories ?? []).map((localRepositoryId) => {
                const binding = bindingByLocalRepository.get(localRepositoryId);
                const bindingBusy = busy === `bind:${localRepositoryId}` || busy === `delete:${localRepositoryId}`;
                return (
                  <div key={localRepositoryId} className="rounded-lg border border-[var(--color-border)] p-4">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div>
                        <div className="font-medium">{localRepositoryId}</div>
                        {binding ? (
                          <div className="mt-1 text-sm text-[var(--color-text-muted)]">
                            {binding.github_full_name}
                            {binding.private ? ' · private' : ' · public'}
                            {binding.fork ? ' · fork' : ''}
                            {binding.archived ? ' · archived' : ''}
                            {binding.disabled ? ' · disabled' : ''}
                          </div>
                        ) : (
                          <div className="mt-1 text-sm text-[var(--color-text-muted)]">Not bound</div>
                        )}
                      </div>
                      {binding && (
                        <button
                          type="button"
                          onClick={() => void removeBinding(localRepositoryId)}
                          disabled={bindingBusy}
                          className="flex items-center gap-1.5 text-sm text-[var(--color-text-muted)] hover:text-[var(--color-error)] disabled:opacity-50"
                        >
                          {busy === `delete:${localRepositoryId}` ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
                          Unbind
                        </button>
                      )}
                    </div>

                    {status.connected && (
                      <div className="mt-3 flex flex-col gap-2 sm:flex-row">
                        <select
                          value={selectedRepositories[localRepositoryId] ?? ''}
                          onChange={(event) => setSelectedRepositories((current) => ({ ...current, [localRepositoryId]: event.target.value }))}
                          className="min-w-0 flex-1 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-primary)] px-3 py-2 text-sm outline-none focus:border-[var(--color-accent)]"
                        >
                          <option value="">Choose a GitHub repository…</option>
                          {filteredRepositories.map((repository) => (
                            <option key={repository.id} value={repository.id} disabled={repository.disabled}>
                              {repositoryLabel(repository)}
                            </option>
                          ))}
                        </select>
                        <button
                          type="button"
                          disabled={bindingBusy || !selectedRepositories[localRepositoryId]}
                          onClick={() => void bindRepository(localRepositoryId)}
                          className="rounded-md bg-[var(--color-accent)] px-3 py-2 text-sm text-white hover:opacity-90 disabled:opacity-50"
                        >
                          {busy === `bind:${localRepositoryId}` ? 'Binding…' : binding ? 'Replace binding' : 'Bind'}
                        </button>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}

          {status.connected && hasMoreRepositories && (
            <button
              type="button"
              onClick={() => void loadMoreRepositories()}
              disabled={busy !== null}
              className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm hover:bg-[var(--color-bg-hover)] disabled:opacity-50"
            >
              {busy === 'load-more' ? 'Loading…' : 'Load more GitHub repositories'}
            </button>
          )}

          {staleBindings.length > 0 && (
            <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-4">
              <div className="flex items-center gap-2 text-sm font-medium"><AlertTriangle size={16} /> Inactive bindings</div>
              <div className="mt-2 space-y-1 text-sm text-[var(--color-text-muted)]">
                {staleBindings.map((binding) => (
                  <div key={`${binding.local_repository_id}:${binding.github_repository_id}`}>
                    {binding.local_repository_id} → {binding.github_full_name}: {!binding.account_matches ? 'belongs to a different GitHub identity' : 'local repository is no longer configured'}
                  </div>
                ))}
              </div>
            </div>
          )}
        </section>
      )}

      <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-tertiary)] p-3 text-xs text-[var(--color-text-muted)]">
        Connecting or binding GitHub supplies identity and repository selection only. It does not enable Git pushes, branch publication, pull-request mutations, cloning, or other write capabilities; existing operator gates and tool approvals still apply.
      </div>
    </div>
  );
}
