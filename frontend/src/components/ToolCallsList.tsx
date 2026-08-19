import { useState } from 'react';
import { ChevronDown, Wrench, CheckCircle2, XCircle } from 'lucide-react';
import type { ToolResult, ToolCall } from '../types';
import { ToolCallCard } from './ToolCallCard';
import { getToolCallName, getToolCallArgs } from './toolCallUtils';

interface ToolCallsListProps {
  toolCalls: ToolCall[];
  toolResults?: ToolResult[];
}

export function ToolCallsList({ toolCalls, toolResults = [] }: ToolCallsListProps) {
  const [expanded, setExpanded] = useState(false);

  if (!toolCalls || toolCalls.length === 0) return null;

  const errorCount = toolCalls.filter((call, index) => {
    const result = toolResults.find((item) => item.tool_call_id && call.id && item.tool_call_id === call.id)
      || toolResults[index];
    return result?.is_error;
  }).length;

  const successCount = toolCalls.length - errorCount;

  // Single tool call: render single compact card directly
  if (toolCalls.length === 1) {
    const call = toolCalls[0];
    const result = toolResults.find((item) => item.tool_call_id && call.id && item.tool_call_id === call.id)
      || toolResults[0];
    return (
      <div className="mt-1.5 pt-1.5 border-t border-border/20">
        <ToolCallCard
          toolName={getToolCallName(call)}
          args={getToolCallArgs(call)}
          result={result}
          status={result ? (result.is_error ? 'error' : 'success') : 'error'}
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
            const result = toolResults.find((item) => item.tool_call_id && call.id && item.tool_call_id === call.id)
              || toolResults[index];
            return (
              <ToolCallCard
                key={call.id || `${getToolCallName(call)}-${index}`}
                toolName={getToolCallName(call)}
                args={getToolCallArgs(call)}
                result={result}
                status={result ? (result.is_error ? 'error' : 'success') : 'error'}
              />
            );
          })}
        </div>
      )}
    </div>
  );
}
