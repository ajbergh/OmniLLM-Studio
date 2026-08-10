import { useCallback, useEffect, useRef, useState } from 'react';
import { ExternalLink, KeyRound, RefreshCw, ShieldCheck, Unplug } from 'lucide-react';
import { toast } from 'sonner';
import { mcpOAuthApi, type MCPOAuthAuthMethod, type MCPOAuthRegistrationMethod, type MCPOAuthStatus } from '../mcpOAuthApi';
import type { MCPServer } from '../types';

export function MCPAuthorizationPanel({ server, onChanged }: { server: MCPServer; onChanged?: () => void }) {
  const [status, setStatus] = useState<MCPOAuthStatus | null>(null);
  const [clientId, setClientId] = useState('');
  const [clientSecret, setClientSecret] = useState('');
  const [clientIssuer, setClientIssuer] = useState('');
  const [authMethod, setAuthMethod] = useState<MCPOAuthAuthMethod>('none');
  const [registrationMethod, setRegistrationMethod] = useState<MCPOAuthRegistrationMethod>('preregistered');
  const [busy, setBusy] = useState(false);
  const pollTimer = useRef<ReturnType<typeof setInterval> | null>(null);

  const load = useCallback(async () => {
    try {
      const next = await mcpOAuthApi.status(server.id);
      setStatus(next);
      const generatedDCR = next.registration_method === 'dcr';
      setClientId(generatedDCR ? '' : (next.client_id || ''));
      setClientIssuer(generatedDCR ? '' : (next.client_issuer || ''));
      setAuthMethod(next.token_endpoint_auth_method || 'none');
      setRegistrationMethod(next.registration_method === 'cimd' ? 'cimd' : 'preregistered');
      return next;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to load MCP OAuth status');
      return null;
    }
  }, [server.id]);

  useEffect(() => {
    void load();
    return () => {
      if (pollTimer.current) clearInterval(pollTimer.current);
    };
  }, [load]);

  if (server.transport !== 'http') return null;

  const saveConfiguration = async () => {
    const trimmedClientId = clientId.trim();
    if (!trimmedClientId) throw new Error('Client ID is required');
    const payload: Parameters<typeof mcpOAuthApi.configure>[1] = {
      client_id: trimmedClientId,
      token_endpoint_auth_method: registrationMethod === 'cimd' ? 'none' : authMethod,
      registration_method: registrationMethod,
    };
    if (registrationMethod === 'preregistered') {
      const issuer = clientIssuer.trim();
      if (!issuer) throw new Error('Authorization server issuer is required for a preregistered client');
      payload.client_issuer = issuer;
    }
    if (registrationMethod === 'preregistered' && clientSecret !== '') payload.client_secret = clientSecret;
    const next = await mcpOAuthApi.configure(server.id, payload);
    setStatus(next);
    setClientSecret('');
    return next;
  };

  const connect = async () => {
    const popup = window.open('about:blank', '_blank', 'width=720,height=820');
    if (popup) popup.opener = null;
    setBusy(true);
    try {
      const steppingUp = Boolean(status?.required_scope);
      if (clientId.trim()) {
        await saveConfiguration();
      } else if (registrationMethod === 'cimd') {
        throw new Error('A Client ID Metadata Document URL is required');
      }
      const start = await mcpOAuthApi.start(server.id);
      if (popup) popup.location.href = start.authorization_url;
      else window.open(start.authorization_url, '_blank', 'noopener,noreferrer');
      toast.info('Complete authorization in the browser window');

      if (pollTimer.current) clearInterval(pollTimer.current);
      let attempts = 0;
      pollTimer.current = setInterval(async () => {
        attempts += 1;
        const next = await load();
        if ((next?.connected && (!steppingUp || !next.required_scope)) || attempts >= 60) {
          if (pollTimer.current) clearInterval(pollTimer.current);
          pollTimer.current = null;
          if (next?.connected) {
            toast.success(steppingUp ? 'MCP OAuth permissions updated' : 'MCP OAuth connected');
            onChanged?.();
          }
        }
      }, 2000);
    } catch (error) {
      popup?.close();
      toast.error(error instanceof Error ? error.message : 'Failed to start MCP OAuth');
    } finally {
      setBusy(false);
    }
  };

  const disconnect = async () => {
    setBusy(true);
    try {
      await mcpOAuthApi.disconnect(server.id);
      await load();
      onChanged?.();
      toast.success('MCP OAuth disconnected');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to disconnect MCP OAuth');
    } finally {
      setBusy(false);
    }
  };

  const resetDynamicRegistration = async () => {
    setBusy(true);
    try {
      await mcpOAuthApi.resetDynamicRegistration(server.id);
      await load();
      onChanged?.();
      toast.success('Dynamic OAuth registration reset');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to reset dynamic OAuth registration');
    } finally {
      setBusy(false);
    }
  };

  const clearSecret = async () => {
    setBusy(true);
    try {
      const next = await mcpOAuthApi.configure(server.id, {
        client_id: clientId.trim(),
        client_secret: '',
        token_endpoint_auth_method: authMethod,
        registration_method: 'preregistered',
        client_issuer: clientIssuer.trim(),
      });
      setStatus(next);
      setClientSecret('');
      toast.success('Stored MCP OAuth client secret cleared');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to clear MCP OAuth client secret');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="rounded-2xl border border-border bg-surface-alt p-5">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-gradient-to-br from-violet-500/20 to-sky-500/20 shadow-md shadow-violet-500/10">
            <KeyRound size={18} className="text-violet-300" />
          </div>
          <div>
            <h4 className="text-sm font-bold">OAuth 2.1 authorization</h4>
            <p className="text-[11px] text-text-muted">Protected-resource discovery, PKCE S256, resource binding, encrypted tokens, and automatic refresh</p>
          </div>
        </div>
        <button type="button" onClick={() => void load()} className="rounded-lg p-2 text-text-muted hover:bg-surface-hover hover:text-text" aria-label="Refresh OAuth status">
          <RefreshCw size={13} />
        </button>
      </div>

      <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
        <label>
          <span className="mb-1 block text-[10px] font-medium text-text-muted">Client registration</span>
          <select value={registrationMethod} onChange={(event) => { const next = event.target.value as MCPOAuthRegistrationMethod; setRegistrationMethod(next); if (next === 'cimd') { setAuthMethod('none'); setClientSecret(''); } }} className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text">
            <option value="preregistered">Preregistered client</option>
            <option value="cimd">Client ID Metadata Document (CIMD)</option>
          </select>
        </label>
        <label>
          <span className="mb-1 block text-[10px] font-medium text-text-muted">{registrationMethod === 'cimd' ? 'Client metadata document URL' : 'Preregistered client ID'}</span>
          <input value={clientId} onChange={(event) => setClientId(event.target.value)} placeholder={registrationMethod === 'cimd' ? 'https://client.example/oauth/metadata.json' : 'oauth-client-id'} className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text" />
        </label>
      </div>

      {registrationMethod === 'preregistered' && (
        <label className="mt-3 block">
          <span className="mb-1 block text-[10px] font-medium text-text-muted">Authorization server issuer</span>
          <input value={clientIssuer} onChange={(event) => setClientIssuer(event.target.value)} placeholder="https://auth.example.com" className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text" />
          <span className="mt-1 block text-[10px] leading-relaxed text-text-muted">Use the exact HTTPS issuer that owns this preregistered client. Omni verifies discovery against this value before sending credentials.</span>
        </label>
      )}

      {registrationMethod === 'cimd' ? (
        <div className="mt-3 rounded-xl border border-sky-500/20 bg-sky-500/5 p-3 text-[10px] leading-relaxed text-text-muted">
          CIMD is the preferred MCP registration method when client and authorization server have no prior relationship. The URL must be a stable public HTTPS document with a path; its <span className="font-mono">client_id</span> must exactly match the URL and include this redirect URI in <span className="font-mono">redirect_uris</span>.
        </div>
      ) : (
        <>
          <label className="mt-3 block">
            <span className="mb-1 block text-[10px] font-medium text-text-muted">Token endpoint authentication</span>
            <select value={authMethod} onChange={(event) => setAuthMethod(event.target.value as MCPOAuthAuthMethod)} className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text">
              <option value="none">Public client (none)</option>
              <option value="client_secret_basic">client_secret_basic</option>
              <option value="client_secret_post">client_secret_post</option>
            </select>
          </label>
          <label className="mt-3 block">
            <span className="mb-1 block text-[10px] font-medium text-text-muted">Client secret {status?.has_client_secret ? '(stored encrypted; leave blank to keep)' : '(optional for public clients)'}</span>
            <input type="password" value={clientSecret} onChange={(event) => setClientSecret(event.target.value)} autoComplete="new-password" className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text" />
          </label>
        </>
      )}

      {registrationMethod === 'preregistered' && !clientId.trim() && status?.registration_method !== 'dcr' && (
        <div className="mt-3 rounded-xl border border-amber-500/20 bg-amber-500/5 p-3 text-[10px] leading-relaxed text-text-muted">
          No client ID entered. Connect can fall back to legacy Dynamic Client Registration only when the authorization server advertises a registration endpoint. DCR is retained for backward compatibility; preregistration or CIMD is preferred.
        </div>
      )}

      <div className="mt-3 rounded-xl border border-border bg-surface/50 p-3 text-[10px] leading-relaxed text-text-muted">
        <div><span className="font-medium text-text-secondary">Redirect URI:</span> <span className="break-all font-mono">{status?.redirect_uri || 'Loading…'}</span></div>
        <div className="mt-1">Register this redirect URI with the authorization server. For desktop, the loopback port changes per launch; native OAuth registrations should allow loopback redirect ports.</div>
      </div>

      {status?.authorization_server && (
        <div className="mt-3 rounded-xl border border-border bg-surface/50 p-3 text-[10px] text-text-muted">
          <div className="break-all"><span className="font-medium text-text-secondary">Authorization server:</span> {status.authorization_server}</div>
          <div className="mt-1"><span className="font-medium text-text-secondary">Registration:</span> {status.registration_method === 'cimd' ? 'Client ID Metadata Document' : status.registration_method === 'dcr' ? 'Dynamic registration (legacy DCR)' : 'Preregistered client'}</div>
          {status.client_issuer && <div className="mt-1 break-all"><span className="font-medium text-text-secondary">Client issuer binding:</span> {status.client_issuer}</div>}
          {status.scope && <div className="mt-1 break-all"><span className="font-medium text-text-secondary">Granted scope:</span> {status.scope}</div>}
          {status.expires_at && <div className="mt-1"><span className="font-medium text-text-secondary">Access token expiry:</span> {new Date(status.expires_at).toLocaleString()}</div>}
        </div>
      )}

      {status?.required_scope && (
        <div className="mt-3 rounded-xl border border-amber-500/30 bg-amber-500/10 p-3 text-[11px] leading-relaxed text-amber-200">
          <div className="font-semibold">Additional OAuth permission required</div>
          <div className="mt-1 break-all font-mono text-[10px]">{status.required_scope}</div>
          <div className="mt-1 text-[10px] text-text-muted">Granting this step-up starts a new PKCE authorization flow and unions the challenged scopes with the scopes already granted/requested.</div>
        </div>
      )}

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <button type="button" disabled={busy || (registrationMethod === 'cimd' && !clientId.trim())} onClick={() => void connect()} className="btn-primary inline-flex min-h-10 items-center gap-1.5 rounded-xl px-3 text-xs disabled:opacity-50">
          {status?.connected ? <ShieldCheck size={13} /> : <ExternalLink size={13} />}
          {status?.required_scope ? 'Grant additional scopes' : status?.connected ? 'Reconnect OAuth' : status?.registration_method === 'dcr' ? 'Reconnect OAuth' : 'Connect OAuth'}
        </button>
        {status?.connected && (
          <button type="button" disabled={busy} onClick={() => void disconnect()} className="inline-flex min-h-10 items-center gap-1.5 rounded-xl border border-border px-3 text-xs text-text hover:bg-surface-hover disabled:opacity-50">
            <Unplug size={13} /> Disconnect
          </button>
        )}
        {status?.registration_method === 'dcr' && (
          <button type="button" disabled={busy} onClick={() => void resetDynamicRegistration()} className="min-h-10 rounded-xl border border-border px-3 text-xs text-text-muted hover:bg-surface-hover hover:text-text disabled:opacity-50">
            Reset dynamic registration
          </button>
        )}
        {status?.has_client_secret && (
          <button type="button" disabled={busy || !clientId.trim()} onClick={() => void clearSecret()} className="min-h-10 rounded-xl border border-border px-3 text-xs text-text-muted hover:bg-surface-hover hover:text-text disabled:opacity-50">
            Clear stored secret
          </button>
        )}
        <span className={`ml-auto rounded-full px-2 py-1 text-[10px] font-medium ${status?.connected ? 'bg-emerald-500/15 text-emerald-300' : status?.configured ? 'bg-amber-500/15 text-amber-300' : 'bg-surface text-text-muted'}`}>
          {status?.connected ? 'Connected' : status?.configured ? 'Configured' : 'Not configured'}
        </span>
      </div>
    </div>
  );
}
