import { useEffect, useState } from 'react';
import { RefreshCw, ShieldCheck, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '../api';
import { scopedToolPermissionsApi, type ScopedToolPermission, type ToolScopeType } from '../scopedToolPermissionsApi';
import type { ToolDefinition } from '../types';
import { GitHubSettingsSection } from './GitHubSettingsSection';

export function ScopedToolPermissionsPanel() {
  const [scopeType, setScopeType] = useState<ToolScopeType>('workspace');
  const [scopeId, setScopeId] = useState('');
  const [toolName, setToolName] = useState('');
  const [policy, setPolicy] = useState<'allow' | 'ask' | 'deny'>('ask');
  const [tools, setTools] = useState<ToolDefinition[]>([]);
  const [items, setItems] = useState<ScopedToolPermission[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    api.listTools().then((next) => {
      setTools(next || []);
      if (!toolName && next?.length) setToolName(next[0].name);
    }).catch(() => setTools([]));
  }, [toolName]);

  const load = async () => {
    if (!scopeId.trim()) return;
    setLoading(true);
    try { setItems(await scopedToolPermissionsApi.list(scopeType, scopeId.trim())); }
    catch (error) { toast.error(error instanceof Error ? error.message : 'Failed to load scoped policies'); }
    finally { setLoading(false); }
  };

  const save = async () => {
    if (!scopeId.trim() || !toolName) return;
    try {
      setItems(await scopedToolPermissionsApi.upsert({ scope_type: scopeType, scope_id: scopeId.trim(), tool_name: toolName, policy }));
      toast.success('Scoped tool policy saved');
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Failed to save scoped policy'); }
  };

  return (
    <div className="space-y-6">
      <GitHubSettingsSection />

      <div className="rounded-2xl border border-border bg-surface-alt p-5">
        <div className="flex items-start gap-3">
          <ShieldCheck size={18} className="mt-0.5 shrink-0 text-emerald-300" />
          <div>
            <h3 className="text-sm font-bold">Scoped Tool Restrictions</h3>
            <p className="mt-1 text-[11px] leading-relaxed text-text-muted">Admin restrictions compose from global → user → workspace → conversation. Lower scopes can tighten Allow → Ask → Off, but can never widen an inherited restriction. Per-turn composer selection narrows access one step further.</p>
          </div>
        </div>

        <div className="mt-4 grid gap-2 sm:grid-cols-[130px_1fr_auto]">
          <select value={scopeType} onChange={(event) => setScopeType(event.target.value as ToolScopeType)} className="rounded-xl border border-border bg-surface px-3 py-2 text-xs text-text">
            <option value="user">User</option><option value="workspace">Workspace</option><option value="conversation">Conversation</option>
          </select>
          <input value={scopeId} onChange={(event) => setScopeId(event.target.value)} placeholder={`${scopeType} ID`} className="rounded-xl border border-border bg-surface px-3 py-2 text-xs text-text" />
          <button type="button" onClick={load} disabled={loading || !scopeId.trim()} className="btn-secondary inline-flex min-h-9 items-center justify-center gap-1.5 rounded-xl px-3 text-xs disabled:opacity-50"> <RefreshCw size={12} className={loading ? 'animate-spin' : ''}/>Load</button>
        </div>

        <div className="mt-3 grid gap-2 sm:grid-cols-[1fr_110px_auto]">
          <select value={toolName} onChange={(event) => setToolName(event.target.value)} className="rounded-xl border border-border bg-surface px-3 py-2 text-xs text-text">
            {tools.map((tool) => <option key={tool.name} value={tool.name}>{tool.name}</option>)}
          </select>
          <select value={policy} onChange={(event) => setPolicy(event.target.value as 'allow'|'ask'|'deny')} className="rounded-xl border border-border bg-surface px-3 py-2 text-xs text-text">
            <option value="allow">Allow</option><option value="ask">Ask</option><option value="deny">Off</option>
          </select>
          <button type="button" onClick={save} disabled={!scopeId.trim() || !toolName} className="btn-primary rounded-xl px-3 py-2 text-xs disabled:opacity-50">Save restriction</button>
        </div>

        {items.length > 0 && <div className="mt-4 overflow-hidden rounded-xl border border-border bg-surface/50 divide-y divide-border/70">
          {items.map((item) => <div key={`${item.scope_type}:${item.scope_id}:${item.tool_name}`} className="flex items-center gap-3 px-3 py-2 text-[11px]">
            <span className="min-w-0 flex-1 truncate font-mono text-text">{item.tool_name}</span>
            <span className={`rounded-md px-2 py-0.5 text-[10px] ${item.policy==='deny'?'bg-red-500/15 text-red-300':item.policy==='ask'?'bg-amber-500/15 text-amber-300':'bg-emerald-500/15 text-emerald-300'}`}>{item.policy==='deny'?'Off':item.policy==='ask'?'Ask':'Allow'}</span>
            <button type="button" aria-label={`Remove scoped policy for ${item.tool_name}`} onClick={async()=>{try{await scopedToolPermissionsApi.delete(item);await load();}catch(error){toast.error(error instanceof Error?error.message:'Failed to remove scoped policy')}}} className="p-1.5 text-text-muted hover:text-red-300"><Trash2 size={12}/></button>
          </div>)}
        </div>}
      </div>
    </div>
  );
}