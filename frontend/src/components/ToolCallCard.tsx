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
    running: <Loader2 size={14} className="animate-spin text-primary" />,
    success: <CheckCircle2 size={14} className="text-emerald-400" />,
    error: <XCircle size={14} className="text-red-400" />,
  }[status];

  const statusColor = {
    running: 'border-primary/30 bg-primary/5',
    success: 'border-emerald-500/30 bg-emerald-500/5',
    error: 'border-red-500/30 bg-red-500/5',
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
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      className={`rounded-xl border ${statusColor} p-3 my-2 text-sm transition-colors`}
    >
      {/* Header / Toggle Button */}
      <button
        type="button"
        onClick={() => hasDetails && setExpanded(!expanded)}
        disabled={!hasDetails}
        className={`w-full flex items-center justify-between gap-2 text-left ${
          hasDetails ? 'cursor-pointer select-none' : 'cursor-default'
        }`}
        aria-expanded={hasDetails ? expanded : undefined}
      >
        <div className="flex items-center gap-2 min-w-0">
          <div className="p-1 rounded-lg bg-surface-light shrink-0">
            <Wrench size={12} className="text-text-muted" />
          </div>
          <span className="font-medium text-text truncate">{toolName}</span>
          {statusIcon}
        </div>
        {hasDetails && (
          <div className="text-text-muted hover:text-text shrink-0 p-0.5">
            {expanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
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
            transition={{ duration: 0.15 }}
            className="overflow-hidden mt-2 pt-2 border-t border-border/30"
          >
            {/* Arguments */}
            {args && Object.keys(args).length > 0 && (
              <div className="mb-2">
                <div className="text-xs text-text-muted mb-1 font-medium">Arguments</div>
                <pre className="text-xs p-2 rounded-lg bg-surface-light/50 overflow-x-auto text-text-muted">
                  {JSON.stringify(args, null, 2)}
                </pre>
              </div>
            )}

            {/* Result */}
            {!result && status === 'error' && (
              <div className="text-xs p-2 rounded-lg bg-red-500/10 text-red-400">
                No tool result was recorded. Retry the request or inspect the tool policy and provider logs.
              </div>
            )}
            {result && (
              <div>
                <div className="text-xs text-text-muted mb-1 font-medium">Result</div>
                {result.is_error ? (
                  <div className="text-xs p-2 rounded-lg bg-red-500/10 text-red-400">
                    {result.content}
                  </div>
                ) : screenshotBase64 ? (
                  <div className="space-y-2">
                    <img
                      src={`data:image/png;base64,${screenshotBase64}`}
                      alt={screenshotURL ? `Screenshot of ${screenshotURL}` : 'Browser screenshot'}
                      className="max-w-full rounded-lg border border-border"
                    />
                    {screenshotURL && (
                      <div className="text-[10px] text-text-muted truncate">{screenshotURL}</div>
                    )}
                  </div>
                ) : (
                  <pre className="text-xs p-2 rounded-lg bg-surface-light/50 overflow-x-auto max-h-32 text-text-muted">
                    {result.content}
                  </pre>
                )}
              </div>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}
