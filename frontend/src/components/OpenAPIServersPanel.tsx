import { useCallback, useEffect, useState } from 'react';
import { Braces, Plus, RefreshCw, Save, ShieldAlert, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { openApiServersApi, type OpenAPIServer } from '../openApiServersApi';

const exampleSpec = `{
  "openapi": "3.0.3",
  "paths": {
    "/items/{id}": {
      "get": {
        "operationId": "getItem",
        "description": "Get an item",
        "parameters": [
          {"name":"id","in":"path","required":true,"schema":{"type":"string"}}
        ]
      }
    }
  }
}`;

const emptyServer: OpenAPIServer = {
  name: '', base_url: '', spec_json: exampleSpec, enabled: true,
  allow_private_network: false, auth_header: 'Authorization', auth_prefix: 'Bearer', api_key: '',
};

export function OpenAPIServersPanel() {
  const [servers,setServers]=useState<OpenAPIServer[]>([]);
  const [form,setForm]=useState<OpenAPIServer>(emptyServer);
  const [saving,setSaving]=useState(false);
  const refresh=useCallback(async()=>{try{setServers(await openApiServersApi.list())}catch(error){toast.error(error instanceof Error?error.message:'Failed to load OpenAPI servers')}},[]);
  useEffect(()=>{void refresh()},[refresh]);

  const save=async()=>{setSaving(true);try{const payload={...form};if(!payload.api_key)delete payload.api_key;const saved=await openApiServersApi.save(payload);setForm({...saved,api_key:''});await refresh();toast.success(`${saved.tools?.length||0} OpenAPI tool${saved.tools?.length===1?'':'s'} registered`)}catch(error){toast.error(error instanceof Error?error.message:'Failed to save OpenAPI server')}finally{setSaving(false)}};
  const remove=async()=>{if(!form.id)return;try{await openApiServersApi.delete(form.id);setForm(emptyServer);await refresh();toast.success('OpenAPI server removed')}catch(error){toast.error(error instanceof Error?error.message:'Failed to delete OpenAPI server')}};

  return <div className="space-y-6">
    <section className="rounded-2xl border border-border bg-surface-alt p-5">
      <div className="flex items-center justify-between gap-3"><div className="flex items-center gap-3"><Braces size={18} className="text-blue-300"/><div><h3 className="text-sm font-bold">OpenAPI Tool Servers</h3><p className="text-[11px] text-text-muted">Turn OpenAPI 3 JSON operations into governed Chat/Agent tools.</p></div></div><button type="button" onClick={()=>setForm(emptyServer)} className="btn-secondary inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-xs"><Plus size={13}/>New</button></div>
      <div className="mt-4 rounded-xl border border-amber-500/20 bg-amber-500/5 p-3 text-[11px] text-text-muted"><div className="flex gap-2"><ShieldAlert size={14} className="mt-0.5 shrink-0 text-amber-300"/><p>Generated operations default to <strong className="text-amber-200">Ask</strong>. Private/loopback destinations are blocked unless explicitly enabled. Redirects are disabled and API keys are encrypted at rest.</p></div></div>
      {servers.length>0&&<div className="mt-4 flex flex-wrap gap-2">{servers.map((server)=><button key={server.id} type="button" onClick={()=>setForm({...server,api_key:''})} className={`rounded-lg px-2.5 py-1.5 text-[11px] ${form.id===server.id?'bg-primary/20 text-primary':'bg-surface text-text-muted hover:text-text'}`}>{server.name} · {server.tools?.length||0}</button>)}</div>}
    </section>

    <section className="rounded-2xl border border-border bg-surface-alt p-5 space-y-3">
      <div className="grid gap-3 sm:grid-cols-2"><input value={form.name} onChange={(e)=>setForm({...form,name:e.target.value})} placeholder="Server name" className="rounded-xl border border-border bg-surface px-3 py-2 text-sm"/><input value={form.base_url} onChange={(e)=>setForm({...form,base_url:e.target.value})} placeholder="https://api.example.com" className="rounded-xl border border-border bg-surface px-3 py-2 text-sm"/></div>
      <textarea value={form.spec_json} onChange={(e)=>setForm({...form,spec_json:e.target.value})} rows={14} spellCheck={false} className="w-full rounded-xl border border-border bg-surface px-3 py-2 font-mono text-[11px]" aria-label="OpenAPI JSON specification" />
      <div className="grid gap-3 sm:grid-cols-3"><input value={form.auth_header||''} onChange={(e)=>setForm({...form,auth_header:e.target.value})} placeholder="Authorization" className="rounded-xl border border-border bg-surface px-3 py-2 text-sm"/><input value={form.auth_prefix||''} onChange={(e)=>setForm({...form,auth_prefix:e.target.value})} placeholder="Bearer" className="rounded-xl border border-border bg-surface px-3 py-2 text-sm"/><input type="password" value={form.api_key||''} onChange={(e)=>setForm({...form,api_key:e.target.value})} placeholder={form.has_api_key?'API key configured — enter to replace':'API key (optional)'} className="rounded-xl border border-border bg-surface px-3 py-2 text-sm"/></div>
      <div className="flex flex-wrap gap-4 text-[11px] text-text-muted"><label className="inline-flex items-center gap-2"><input type="checkbox" checked={form.enabled} onChange={(e)=>setForm({...form,enabled:e.target.checked})}/>Enabled</label><label className="inline-flex items-center gap-2"><input type="checkbox" checked={form.allow_private_network} onChange={(e)=>setForm({...form,allow_private_network:e.target.checked})}/>Allow private/loopback network</label></div>
      {form.tools&&form.tools.length>0&&<div className="rounded-xl border border-border bg-surface/50"><div className="border-b border-border px-3 py-2 text-[10px] font-semibold uppercase tracking-wide text-text-muted">Generated tools</div><div className="divide-y divide-border/70">{form.tools.map((tool)=><div key={tool.name} className="flex items-center gap-2 px-3 py-2 text-[10px]"><span className="w-12 font-semibold text-blue-300">{tool.method}</span><span className="min-w-0 flex-1 truncate font-mono text-text">{tool.name}</span><span className="text-text-muted">{tool.policy||'ask'}</span></div>)}</div></div>}
      <div className="flex gap-2"><button type="button" disabled={saving} onClick={save} className="btn-primary inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-xs">{saving?<RefreshCw size={13} className="animate-spin"/>:<Save size={13}/>}Save & register</button>{form.id&&<button type="button" onClick={async()=>{try{const updated=await openApiServersApi.refresh(form.id!);setForm({...updated,api_key:''});await refresh();toast.success('OpenAPI tools refreshed')}catch(error){toast.error(error instanceof Error?error.message:'Refresh failed')}}} className="inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-xs text-text-muted hover:bg-surface-hover hover:text-text"><RefreshCw size={13}/>Refresh</button>}{form.id&&<button type="button" onClick={remove} className="inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-xs text-red-300 hover:bg-red-500/10"><Trash2 size={13}/>Delete</button>}</div>
    </section>
  </div>;
}
