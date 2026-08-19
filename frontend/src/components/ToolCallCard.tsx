import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Wrench, CheckCircle2, XCircle, Loader2, ChevronDown, ChevronUp } from 'lucide-react';
import type { ToolResult } from '../types';
import { readStringField, parseResultJSON, hasToolCallDetails } from './toolCallUtils';

interface ToolCallCardProps {
  toolName: string;
  args?: Record<string, unknown>;
  result?: ToolResult;
  status?: 'running' | 'success' | 'error';
  defaultExpanded?: boolean;
}

export function ToolCallCard({
  toolName,
  args,
  result,
  status = 'running',
  defaultExpanded = false,
}: ToolCallCardProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  const statusIcon = {
    running: <Loader2 size={12} className="animate-spin text-primary" />,
    success: <CheckCircle2 size={12} className="text-emerald-400" />,
    error: <XCircle size={12} className="text-red-400" />,
  }[status];

  const statusColor = {
    running: 'border-primary/20 bg-primary/5 hover:border-primary/40',
    success: 'border-emerald-500/20 bg-emerald-500/5 hover:border-emerald-500/30',
    error: 'border-red-500/20 bg-red-500/5 hover:border-red-500/30',
  }[status];

  const parsedResult = result ? parseResultJSON(result.content) : null;
  const screenshotBase64 = result
    ? readStringField(result.metadata, 'screenshot_base64') || readStringField(parsedResult, 'screenshot_base64')
    : undefined;
  const screenshotURL = result
    ? readStringField(result.metadata, 'url') || readStringField(parsedResult, 'url')
    : undefined;

  const hasDetails = hasToolCallDetails(args, result, status);

  return (
    <div
      className={`rounded-lg border ${statusColor} px-2.5 py-1.5 my-1 text-xs transition-colors`}
    >
      {/* Compact Header / Toggle Button */}
      <button
        type="button"
        onClick={() => hasDetails && setExpanded(!expanded)}
        disabled={!hasDetails}
        className={`w-full flex items-center justify-between gap-1.5 text-left ${
          hasDetails ? 'cursor-pointer select-none' : 'cursor-default'
        }`}
        aria-expanded={hasDetails ? expanded : undefined}
      >
        <div className="flex items-center gap-1.5 min-w-0">
          <div className="p-0.5 rounded bg-surface-light shrink-0 text-text-muted">
            <Wrench size={10} />
          </div>
          <span className="font-mono text-[11px] font-medium text-text truncate">{toolName}</span>
          {statusIcon}
        </div>
        {hasDetails && (
          <div className="text-text-muted hover:text-text shrink-0 p-0.5">
            {expanded ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
          </div>
        )}
      </button>

      {/* Expandable Details (Arguments & Result) */}
      <AnimatePresence>
        {expanded && hasDetails && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.12 }}
            className="overflow-hidden mt-1.5 pt-1.5 border-t border-border/30"
          >
            {/* Arguments */}
            {args && Object.keys(args).length > 0 && (
              <div className="mb-1.5">
                <div className="text-[10px] text-text-muted mb-0.5 font-medium">Arguments</div>
                <pre className="text-[10px] p-1.5 rounded bg-surface-light/50 overflow-x-auto text-text-muted font-mono leading-tight">
                  {JSON.stringify(args, null, 2)}
                </pre>
              </div>
            )}

            {/* Result */}
            {!result && status === 'error' && (
              <div className="text-[11px] p-1.5 rounded bg-red-500/10 text-red-400">
                No tool result was recorded. Retry the request or inspect the tool policy and provider logs.
              </div>
            )}
            {result && (
              <div>
                <div className="text-[10px] text-text-muted mb-0.5 font-medium">Result</div>
                {result.is_error ? (
                  <div className="text-[11px] p-1.5 rounded bg-red-500/10 text-red-400 font-mono break-words">
                    {result.content}
                  </div>
                ) : screenshotBase64 ? (
                  <div className="space-y-1.5">
                    <img
                      src={`data:image/png;base64,${screenshotBase64}`}
                      alt={screenshotURL ? `Screenshot of ${screenshotURL}` : 'Browser screenshot'}
                      className="max-w-full rounded border border-border"
                    />
                    {screenshotURL && (
                      <div className="text-[10px] text-text-muted truncate font-mono">{screenshotURL}</div>
                    )}
                  </div>
                ) : (
                  <pre className="text-[10px] p-1.5 rounded bg-surface-light/50 overflow-x-auto max-h-32 text-text-muted font-mono leading-tight whitespace-pre-wrap break-words">
                    {result.content}
                  </pre>
                )}
              </div>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
