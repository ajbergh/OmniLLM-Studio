import { motion } from 'framer-motion';
import { Wrench, CheckCircle2, XCircle, Loader2 } from 'lucide-react';
import type { ToolResult } from '../types';

interface ToolCallCardProps {
  toolName: string;
  args?: Record<string, unknown>;
  result?: ToolResult;
  status?: 'running' | 'success' | 'error';
}

export function ToolCallCard({ toolName, args, result, status = 'running' }: ToolCallCardProps) {
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

  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      className={`rounded-xl border ${statusColor} p-3 my-2 text-sm`}
    >
      {/* Header */}
      <div className="flex items-center gap-2 mb-2">
        <div className="p-1 rounded-lg bg-surface-light">
          <Wrench size={12} className="text-text-muted" />
        </div>
        <span className="font-medium text-text">{toolName}</span>
        {statusIcon}
      </div>

      {/* Arguments */}
      {args && Object.keys(args).length > 0 && (
        <div className="mb-2">
          <div className="text-xs text-text-muted mb-1">Arguments</div>
          <pre className="text-xs p-2 rounded-lg bg-surface-light/50 overflow-x-auto text-text-muted">
            {JSON.stringify(args, null, 2)}
          </pre>
        </div>
      )}

      {/* Result */}
      {result && (
        <div>
          <div className="text-xs text-text-muted mb-1">Result</div>
          {result.is_error ? (
            <div className="text-xs p-2 rounded-lg bg-red-500/10 text-red-400">
              {result.content}
            </div>
          ) : (
            <pre className="text-xs p-2 rounded-lg bg-surface-light/50 overflow-x-auto max-h-32 text-text-muted">
              {result.content}
            </pre>
          )}
        </div>
      )}
    </motion.div>
  );
}
