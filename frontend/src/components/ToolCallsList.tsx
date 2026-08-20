import { useState } from 'react';
import { ChevronDown, Wrench, CheckCircle2, XCircle, Loader2 } from 'lucide-react';
import type { ToolResult, ToolCall } from '../types';
import { ToolCallCard } from './ToolCallCard';
import { getToolCallName, getToolCallArgs } from './toolCallUtils';

interface ToolCallsListProps {
  toolCalls: ToolCall[];
  toolResults?: ToolResult[];
}

function findToolResult(call: ToolCall, index: number, toolResults: ToolResult[]): ToolResult | undefined {
  if (call.id) {
    const match = toolResults.find((item) => item.tool_call_id === call.id);
    if (match) return match;
    const positional = toolResults[index];
    if (positional && !positional.tool_call_id) return positional;
    return undefined;
  }
  return toolResults[index];
}

export function ToolCallsList({ toolCalls, toolResults = [] }: ToolCallsListProps) {
  const [expanded, setExpanded] = useState(false);

  if (!toolCalls || toolCalls.length === 0) return null;

  const statuses = toolCalls.map((call, index) => {
    const result = findToolResult(call, index, toolResults);
    if (!result) return 'running' as const;
    return result.is_error ? ('error' as const) : ('success' as const);
  });

  const runningCount = statuses.filter((s) => s === 'running').length;
  const errorCount = statuses.filter((s) => s === 'error').length;
  const successCount = statuses.filter((s) => s === 'success').length;

  // Single tool call: render single compact card directly
  if (toolCalls.length === 1) {
    const call = toolCalls[0];
    const result = findToolResult(call, 0, toolResults);
    const status = statuses[0];
    return (
      <div className="mt-1.5 pt-1.5 border-t border-border/20">
        <ToolCallCard
          toolName={getToolCallName(call)}
          args={getToolCallArgs(call)}
          result={result}
          status={status}
        />
      </div>
    );
  }

  // Multiple tool calls: compact collapsible summary group
  return (
    <div className="mt-1.5 pt-1.5 border-t border-border/20 text-xs">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-surface border border-border/60 hover:bg-surface-hover transition-colors text-text cursor-pointer select-none"
      >
        <Wrench size={11} className="text-text-muted" />
        <span className="font-medium text-[11px]">
          {toolCalls.length} tool {toolCalls.length === 1 ? 'call' : 'calls'}
        </span>
        <div className="flex items-center gap-1 ml-1">
          {runningCount > 0 && (
            <span className="flex items-center gap-0.5 text-[10px] text-primary font-mono">
              <Loader2 size={10} className="animate-spin" /> {runningCount}
            </span>
          )}
          {successCount > 0 && (
            <span className="flex items-center gap-0.5 text-[10px] text-emerald-400 font-mono">
              <CheckCircle2 size={10} /> {successCount}
            </span>
          )}
          {errorCount > 0 && (
            <span className="flex items-center gap-0.5 text-[10px] text-red-400 font-mono">
              <XCircle size={10} /> {errorCount}
            </span>
          )}
        </div>
        <ChevronDown
          size={12}
          className={`text-text-muted ml-1 transition-transform duration-150 ${expanded ? 'rotate-180' : ''}`}
        />
      </button>

      {expanded && (
        <div className="mt-1.5 space-y-1 pl-1 border-l-2 border-border/40">
          {toolCalls.map((call, index) => {
            const result = findToolResult(call, index, toolResults);
            const status = statuses[index];
            return (
              <ToolCallCard
                key={call.id || `${getToolCallName(call)}-${index}`}
                toolName={getToolCallName(call)}
                args={getToolCallArgs(call)}
                result={result}
                status={status}
              />
            );
          })}
        </div>
      )}
    </div>
  );
}
