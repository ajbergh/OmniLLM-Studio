import { useEffect, useMemo, useState } from 'react';
import { Check, ChevronDown, ShieldQuestion, Wrench, X } from 'lucide-react';
import { api } from '../api';
import type { ChatTurnToolSelection, ToolDefinition } from '../types';

interface ToolPickerProps {
  value: ChatTurnToolSelection;
  onChange: (value: ChatTurnToolSelection) => void;
  disabled?: boolean;
}

export function ToolPicker({ value, onChange, disabled = false }: ToolPickerProps) {
  const [open, setOpen] = useState(false);
  const [tools, setTools] = useState<ToolDefinition[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!open || tools.length > 0) return;
    setLoading(true);
    api.listTools()
      .then((items) => setTools((items || []).filter((tool) => tool.policy !== 'deny')))
      .catch(() => setTools([]))
      .finally(() => setLoading(false));
  }, [open, tools.length]);

  const selected = useMemo(() => new Set(value.allowed_tools || []), [value.allowed_tools]);
  const activeCount = value.mode === 'none'
    ? 0
    : value.mode === 'specific'
      ? 1
      : selected.size;

  const label = value.mode === 'none'
    ? 'Tools off'
    : value.mode === 'specific' && value.required_tool
      ? `Require ${value.required_tool}`
      : selected.size > 0
        ? `${selected.size} tool${selected.size === 1 ? '' : 's'}`
        : value.mode === 'required'
          ? 'Require a tool'
          : 'Tools auto';

  const toggleTool = (name: string) => {
    const next = new Set(selected);
    if (next.has(name)) next.delete(name);
    else next.add(name);
    onChange({ mode: 'auto', allowed_tools: [...next], required_tool: undefined });
  };

  return (
    <div className="relative order-1">
      <button
        type="button"
        onClick={() => !disabled && setOpen((current) => !current)}
        disabled={disabled}
        className={`min-h-10 px-2.5 inline-flex items-center justify-center gap-1.5 rounded-xl transition-colors text-[11px] ${
          disabled
            ? 'opacity-30 cursor-not-allowed text-text-muted'
            : value.mode === 'none'
              ? 'text-text-muted hover:text-text'
              : activeCount > 0 || value.mode === 'required'
                ? 'bg-violet-500/20 text-violet-300'
                : 'text-text-muted hover:text-text'
        }`}
        aria-label="Select tools for this turn"
        title={disabled ? 'Selected model does not support tool calling' : 'Choose which tools the model may use this turn'}
      >
        <Wrench size={15} />
        <span className="hidden sm:inline max-w-32 truncate">{label}</span>
        <ChevronDown size={11} />
      </button>

      {open && !disabled && (
        <>
          <button className="fixed inset-0 z-40 cursor-default" aria-label="Close tool picker" onClick={() => setOpen(false)} />
          <div className="absolute left-0 bottom-full mb-2 z-50 w-[min(92vw,360px)] rounded-2xl border border-border bg-surface-alt shadow-2xl overflow-hidden">
            <div className="flex items-center justify-between px-3 py-2.5 border-b border-border">
              <div>
                <p className="text-xs font-semibold text-text">Tools for this turn</p>
                <p className="text-[10px] text-text-muted mt-0.5">Turn controls can narrow Settings permissions, never override an Off tool.</p>
              </div>
              <button type="button" onClick={() => setOpen(false)} className="p-1 text-text-muted hover:text-text" aria-label="Close">
                <X size={14} />
              </button>
            </div>

            <div className="grid grid-cols-3 gap-1.5 p-2 border-b border-border">
              <button
                type="button"
                onClick={() => onChange({ mode: 'auto', allowed_tools: [], required_tool: undefined })}
                className={`rounded-lg px-2 py-1.5 text-[10px] ${value.mode === 'auto' && selected.size === 0 ? 'bg-primary/20 text-primary' : 'bg-surface-hover text-text-muted hover:text-text'}`}
              >
                Auto
              </button>
              <button
                type="button"
                onClick={() => onChange({ mode: 'required', allowed_tools: [], required_tool: undefined })}
                className={`rounded-lg px-2 py-1.5 text-[10px] ${value.mode === 'required' && !value.required_tool ? 'bg-amber-500/20 text-amber-300' : 'bg-surface-hover text-text-muted hover:text-text'}`}
              >
                Require one
              </button>
              <button
                type="button"
                onClick={() => onChange({ mode: 'none', allowed_tools: [], required_tool: undefined })}
                className={`rounded-lg px-2 py-1.5 text-[10px] ${value.mode === 'none' ? 'bg-red-500/15 text-red-300' : 'bg-surface-hover text-text-muted hover:text-text'}`}
              >
                No tools
              </button>
            </div>

            <div className="max-h-64 overflow-y-auto p-2 space-y-1">
              {loading ? (
                <p className="py-5 text-center text-xs text-text-muted">Loading tools…</p>
              ) : tools.length === 0 ? (
                <p className="py-5 text-center text-xs text-text-muted">No available tools</p>
              ) : tools.map((tool) => {
                const checked = selected.has(tool.name);
                const required = value.mode === 'specific' && value.required_tool === tool.name;
                return (
                  <div key={tool.name} className="flex items-center gap-2 rounded-xl px-2 py-2 hover:bg-surface-hover">
                    <button
                      type="button"
                      onClick={() => toggleTool(tool.name)}
                      className={`h-5 w-5 shrink-0 rounded-md border inline-flex items-center justify-center ${checked ? 'border-primary/60 bg-primary/20 text-primary' : 'border-border text-transparent'}`}
                      aria-label={`${checked ? 'Deselect' : 'Select'} ${tool.name}`}
                    >
                      <Check size={12} />
                    </button>
                    <button type="button" onClick={() => toggleTool(tool.name)} className="min-w-0 flex-1 text-left">
                      <span className="block truncate text-[11px] font-medium text-text">{tool.name}</span>
                      <span className="block truncate text-[10px] text-text-muted">{tool.description}</span>
                    </button>
                    {tool.policy === 'ask' && <ShieldQuestion size={13} className="shrink-0 text-amber-300" aria-label="Approval required" />}
                    <button
                      type="button"
                      onClick={() => onChange({ mode: 'specific', allowed_tools: [tool.name], required_tool: tool.name })}
                      className={`shrink-0 rounded-md px-1.5 py-1 text-[9px] ${required ? 'bg-amber-500/20 text-amber-300' : 'text-text-muted hover:bg-surface hover:text-text'}`}
                      title={`Require ${tool.name} for this turn`}
                    >
                      Require
                    </button>
                  </div>
                );
              })}
            </div>
          </div>
        </>
      )}
    </div>
  );
}
