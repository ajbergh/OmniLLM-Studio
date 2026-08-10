import { useCallback, useEffect, useState } from 'react';
import { Bot, BookOpen, Plus, Save, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { assistantProfilesApi, type AssistantProfile, type Skill } from '../assistantProfilesApi';
import { api } from '../api';
import type { ToolDefinition } from '../types';

const emptyProfile: AssistantProfile = { name: '', description: '', provider: '', model: '', system_prompt: '', tool_names: [], skill_ids: [] };
const emptySkill: Skill = { name: '', description: '', body_markdown: '', enabled: true };

export function AssistantProfilesPanel() {
  const [profiles, setProfiles] = useState<AssistantProfile[]>([]);
  const [skills, setSkills] = useState<Skill[]>([]);
  const [tools, setTools] = useState<ToolDefinition[]>([]);
  const [profile, setProfile] = useState<AssistantProfile>(emptyProfile);
  const [skill, setSkill] = useState<Skill>(emptySkill);

  const refresh = useCallback(async () => {
    try {
      const [nextProfiles, nextSkills, nextTools] = await Promise.all([
        assistantProfilesApi.listProfiles(), assistantProfilesApi.listSkills(), api.listTools(),
      ]);
      setProfiles(nextProfiles || []);
      setSkills(nextSkills || []);
      setTools((nextTools || []).filter((tool) => tool.policy !== 'deny'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to load assistants');
    }
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const saveProfile = async () => {
    try {
      const saved = await assistantProfilesApi.saveProfile(profile);
      setProfile(saved); await refresh(); toast.success('Assistant profile saved');
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Failed to save profile'); }
  };
  const saveSkill = async () => {
    try {
      const saved = await assistantProfilesApi.saveSkill(skill);
      setSkill(saved); await refresh(); toast.success('Skill saved');
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Failed to save skill'); }
  };

  return (
    <div className="space-y-6">
      <section className="rounded-2xl border border-border bg-surface-alt p-5">
        <div className="mb-4 flex items-center justify-between gap-3">
          <div className="flex items-center gap-3"><Bot size={18} className="text-violet-300" /><div><h3 className="text-sm font-bold">Assistant Profiles</h3><p className="text-[11px] text-text-muted">Package model defaults, instructions, tools, and Skills for Agent Mode.</p></div></div>
          <button type="button" onClick={() => setProfile(emptyProfile)} className="btn-secondary inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-xs"><Plus size={13}/>New</button>
        </div>
        {profiles.length > 0 && <div className="mb-4 flex flex-wrap gap-2">{profiles.map((item) => <button key={item.id} type="button" onClick={() => setProfile(item)} className={`rounded-lg px-2.5 py-1.5 text-[11px] ${profile.id === item.id ? 'bg-primary/20 text-primary' : 'bg-surface text-text-muted hover:text-text'}`}>{item.name}</button>)}</div>}
        <div className="grid gap-3 sm:grid-cols-2">
          <input value={profile.name} onChange={(e)=>setProfile({...profile,name:e.target.value})} placeholder="Profile name" className="rounded-xl border border-border bg-surface px-3 py-2 text-sm" />
          <input value={profile.description} onChange={(e)=>setProfile({...profile,description:e.target.value})} placeholder="Description" className="rounded-xl border border-border bg-surface px-3 py-2 text-sm" />
          <input value={profile.provider || ''} onChange={(e)=>setProfile({...profile,provider:e.target.value})} placeholder="Provider (optional)" className="rounded-xl border border-border bg-surface px-3 py-2 text-sm" />
          <input value={profile.model || ''} onChange={(e)=>setProfile({...profile,model:e.target.value})} placeholder="Model (optional)" className="rounded-xl border border-border bg-surface px-3 py-2 text-sm" />
        </div>
        <textarea value={profile.system_prompt || ''} onChange={(e)=>setProfile({...profile,system_prompt:e.target.value})} placeholder="System instructions" rows={5} className="mt-3 w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm" />
        <div className="mt-4"><p className="mb-2 text-[11px] font-semibold uppercase tracking-wide text-text-muted">Allowed tools <span className="normal-case font-normal">(empty = unrestricted)</span></p><div className="flex max-h-40 flex-wrap gap-1.5 overflow-y-auto">{tools.map((tool)=>{const selected=profile.tool_names.includes(tool.name);return <button key={tool.name} type="button" onClick={()=>setProfile({...profile,tool_names:selected?profile.tool_names.filter((name)=>name!==tool.name):[...profile.tool_names,tool.name]})} className={`rounded-lg px-2 py-1 text-[10px] ${selected?'bg-violet-500/20 text-violet-300':'bg-surface text-text-muted hover:text-text'}`}>{tool.name}</button>})}</div></div>
        <div className="mt-4"><p className="mb-2 text-[11px] font-semibold uppercase tracking-wide text-text-muted">Attached Skills</p><div className="flex flex-wrap gap-1.5">{skills.map((item)=>{const selected=Boolean(item.id && profile.skill_ids.includes(item.id));return <button key={item.id} type="button" onClick={()=>item.id&&setProfile({...profile,skill_ids:selected?profile.skill_ids.filter((id)=>id!==item.id):[...profile.skill_ids,item.id]})} className={`rounded-lg px-2 py-1 text-[10px] ${selected?'bg-cyan-500/20 text-cyan-300':'bg-surface text-text-muted hover:text-text'}`}>{item.name}</button>})}</div></div>
        <div className="mt-5 flex gap-2"><button type="button" onClick={saveProfile} className="btn-primary inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-xs"><Save size={13}/>Save profile</button>{profile.id&&<button type="button" onClick={async()=>{await assistantProfilesApi.deleteProfile(profile.id!);setProfile(emptyProfile);await refresh();}} className="inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-xs text-red-300 hover:bg-red-500/10"><Trash2 size={13}/>Delete</button>}</div>
      </section>

      <section className="rounded-2xl border border-border bg-surface-alt p-5">
        <div className="mb-4 flex items-center justify-between gap-3"><div className="flex items-center gap-3"><BookOpen size={18} className="text-cyan-300"/><div><h3 className="text-sm font-bold">Skills</h3><p className="text-[11px] text-text-muted">Reusable Markdown instructions are discovered by metadata and loaded only when needed.</p></div></div><button type="button" onClick={()=>setSkill(emptySkill)} className="btn-secondary inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-xs"><Plus size={13}/>New</button></div>
        {skills.length > 0 && <div className="mb-4 flex flex-wrap gap-2">{skills.map((item)=><button key={item.id} type="button" onClick={async()=>item.id&&setSkill(await assistantProfilesApi.getSkill(item.id))} className={`rounded-lg px-2.5 py-1.5 text-[11px] ${skill.id===item.id?'bg-primary/20 text-primary':'bg-surface text-text-muted hover:text-text'}`}>{item.name}</button>)}</div>}
        <div className="grid gap-3 sm:grid-cols-2"><input value={skill.name} onChange={(e)=>setSkill({...skill,name:e.target.value})} placeholder="Skill name" className="rounded-xl border border-border bg-surface px-3 py-2 text-sm"/><input value={skill.description} onChange={(e)=>setSkill({...skill,description:e.target.value})} placeholder="Short discovery description" className="rounded-xl border border-border bg-surface px-3 py-2 text-sm"/></div>
        <textarea value={skill.body_markdown || ''} onChange={(e)=>setSkill({...skill,body_markdown:e.target.value})} placeholder="# Skill instructions\nWhen this skill applies…" rows={9} className="mt-3 w-full rounded-xl border border-border bg-surface px-3 py-2 font-mono text-xs"/>
        <div className="mt-4 flex gap-2"><button type="button" onClick={saveSkill} className="btn-primary inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-xs"><Save size={13}/>Save skill</button>{skill.id&&<button type="button" onClick={async()=>{await assistantProfilesApi.deleteSkill(skill.id!);setSkill(emptySkill);await refresh();}} className="inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-xs text-red-300 hover:bg-red-500/10"><Trash2 size={13}/>Delete</button>}</div>
      </section>
    </div>
  );
}
