import { useState, useEffect, useCallback, useRef, type ReactNode } from 'react';
import { agentApi } from '../api';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import {
  Bot, Play, Square, CheckCircle2, Clock, AlertTriangle,
  ChevronDown, ChevronUp, ThumbsUp, ThumbsDown, Loader2,
  Brain, Wrench, MessageSquare, ShieldCheck, XCircle
} from 'lucide-react';
import type { AgentRun, AgentRunWithSteps, AgentStep } from '../types';
import {
  AgentRunStatus, AgentStepStatus, AgentStepType, AgentEventType,
} from '../types';

interface AgentRunViewProps {
  conversationId: string;
}

export function AgentRunView({ conversationId }: AgentRunViewProps) {
  const [runs, setRuns] = useState<AgentRun[]>([]);
  const [activeRun, setActiveRun] = useState<AgentRunWithSteps | null>(null);
  const [starting, setStarting] = useState(false);
  const [goal, setGoal] = useState('');
  const [expanded, setExpanded] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  const fetchRuns = useCallback(async () => {
    try {
      const data = await agentApi.listRuns(conversationId);
      setRuns(data || []);
    } catch {
      // silent — list may not be available yet
    }
  }, [conversationId]);

  useEffect(() => {
    if (expanded) fetchRuns();
  }, [expanded, fetchRuns]);

  // Cleanup abort controller on unmount.
  useEffect(() => {
    return () => { abortRef.current?.abort(); };
  }, []);

  const startRun = async () => {
    if (!goal.trim()) return;
    setStarting(true);
    try {
      const { promise, abort } = agentApi.startRun(
        conversationId,
        { goal, provider: '', model: '' },
        (event) => {
          try {
            const payload = event.data ? JSON.parse(event.data) : {};

            switch (event.type) {
              case AgentEventType.Plan:
                // Plan received — could update UI with planned steps
                break;
              case AgentEventType.StepStart:
                // Step execution started
                break;
              case AgentEventType.StepComplete:
                // Step execution completed
                break;
              case AgentEventType.ApprovalRequired:
                toast.info('Agent is waiting for your approval');
                // Reload run details to show approval UI
                if (payload.run_id || activeRun?.id) {
                  loadRunDetails(payload.run_id || activeRun!.id);
                }
                break;
              case AgentEventType.Complete:
                toast.success('Agent run completed');
                break;
              case AgentEventType.Error:
                toast.error(`Agent error: ${payload?.data?.error || payload?.error || 'Unknown error'}`);
                break;
              case 'done':
                // Final SSE event — stream is closing
                break;
              default:
                break;
            }
          } catch {
            // Ignore parse errors on individual events
          }
        },
      );
      abortRef.current = abort;
      await promise;

      setGoal('');
      fetchRuns();
    } catch (err) {
      toast.error(`Agent run failed: ${(err as Error).message}`);
    } finally {
      setStarting(false);
      abortRef.current = null;
    }
  };

  const loadRunDetails = async (runId: string) => {
    try {
      const details = await agentApi.getRun(runId);
      setActiveRun(details);
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const approveStep = async (runId: string, stepId: string, approved: boolean) => {
    try {
      await agentApi.approveStep(runId, stepId, approved);
      toast.success(approved ? 'Step approved' : 'Step rejected');
      loadRunDetails(runId);
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const cancelRun = async (runId: string) => {
    try {
      // Abort the SSE stream if it's the active one.
      abortRef.current?.abort();
      abortRef.current = null;

      await agentApi.cancelRun(runId);
      toast.success('Run cancelled');
      fetchRuns();
      setActiveRun(null);
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const stepTypeIcon = (type: string) => {
    switch (type) {
      case AgentStepType.Think:    return <Brain size={10} />;
      case AgentStepType.ToolCall: return <Wrench size={10} />;
      case AgentStepType.Message:  return <MessageSquare size={10} />;
      case AgentStepType.Approval: return <ShieldCheck size={10} />;
      default: return <Clock size={10} />;
    }
  };

  const statusBadge = (status: string) => {
    const config: Record<string, { icon: typeof Clock; color: string }> = {
      [AgentRunStatus.Planning]:         { icon: Clock, color: 'text-purple-400 bg-purple-400/10' },
      [AgentStepStatus.Pending]:         { icon: Clock, color: 'text-amber-400 bg-amber-400/10' },
      [AgentRunStatus.Running]:          { icon: Loader2, color: 'text-blue-400 bg-blue-400/10' },
      [AgentRunStatus.AwaitingApproval]: { icon: ShieldCheck, color: 'text-amber-400 bg-amber-400/10' },
      [AgentRunStatus.Paused]:           { icon: Clock, color: 'text-yellow-400 bg-yellow-400/10' },
      [AgentRunStatus.Completed]:        { icon: CheckCircle2, color: 'text-emerald-400 bg-emerald-400/10' },
      [AgentRunStatus.Failed]:           { icon: XCircle, color: 'text-red-400 bg-red-400/10' },
      [AgentRunStatus.Cancelled]:        { icon: Square, color: 'text-gray-400 bg-gray-400/10' },
      [AgentStepStatus.Skipped]:         { icon: AlertTriangle, color: 'text-gray-400 bg-gray-400/10' },
    };
    const entry = config[status] || { icon: Clock, color: 'text-gray-400 bg-gray-400/10' };
    const Icon = entry.icon;
    return (
      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs ${entry.color}`}>
        <Icon size={10} className={status === AgentRunStatus.Running ? 'animate-spin' : ''} /> {status}
      </span>
    );
  };

  const canCancel = (status: string) =>
    status === AgentRunStatus.Running ||
    status === AgentRunStatus.AwaitingApproval ||
    status === AgentRunStatus.Planning;

  return (
    <div className="glass rounded-xl overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-4 py-3 text-sm hover:bg-surface-light/50 transition-colors"
      >
        <div className="flex items-center gap-2 text-text-muted">
          <Bot size={14} />
          <span className="font-medium">Agent Mode</span>
          {runs.length > 0 && (
            <span className="text-xs px-1.5 py-0.5 rounded-full bg-primary/20 text-primary">
              {runs.length}
            </span>
          )}
        </div>
        {expanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
      </button>

      <AnimatePresence>
        {expanded && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            className="overflow-hidden"
          >
            <div className="px-4 pb-4 space-y-3">
              {/* Start new run */}
              <div className="flex gap-2">
                <input
                  type="text"
                  value={goal}
                  onChange={(e) => setGoal(e.target.value)}
                  placeholder="Describe the goal..."
                  className="flex-1 px-3 py-1.5 rounded-lg bg-surface-light border border-border text-text text-sm
                             focus:outline-none focus:border-primary/50"
                  onKeyDown={(e) => e.key === 'Enter' && startRun()}
                />
                <motion.button
                  whileHover={{ scale: 1.05 }}
                  whileTap={{ scale: 0.95 }}
                  onClick={startRun}
                  disabled={starting || !goal.trim()}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg btn-primary text-xs font-medium disabled:opacity-50"
                >
                  {starting ? <div className="w-3 h-3 border-2 border-white/30 border-t-white rounded-full animate-spin" /> : <Play size={12} />}
                  Run
                </motion.button>
              </div>

              {/* Run details */}
              {activeRun ? (
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium text-text">{activeRun.goal}</span>
                    <div className="flex items-center gap-2">
                      {statusBadge(activeRun.status)}
                      {canCancel(activeRun.status) && (
                        <motion.button
                          whileTap={{ scale: 0.95 }}
                          onClick={() => cancelRun(activeRun.id)}
                          className="p-1 rounded text-red-400 hover:bg-red-400/10"
                          title="Cancel run"
                        >
                          <Square size={12} />
                        </motion.button>
                      )}
                    </div>
                  </div>

                  {/* Result summary */}
                  {activeRun.result_summary && (
                    <div className="p-2.5 rounded-lg bg-emerald-400/5 border border-emerald-400/20 text-xs text-emerald-300">
                      {activeRun.result_summary}
                    </div>
                  )}

                  {/* Steps */}
                  <div className="space-y-1.5">
                    {activeRun.steps?.map((step: AgentStep, i: number) => (
                      <StepCard
                        key={step.id}
                        step={step}
                        index={i}
                        runId={activeRun.id}
                        statusBadge={statusBadge}
                        stepTypeIcon={stepTypeIcon}
                        onApprove={(approved) => approveStep(activeRun.id, step.id, approved)}
                      />
                    ))}
                  </div>

                  <button
                    onClick={() => setActiveRun(null)}
                    className="text-xs text-text-muted hover:text-text transition-colors"
                  >
                    ← Back to runs
                  </button>
                </div>
              ) : (
                /* Run list */
                <div className="space-y-1.5 max-h-48 overflow-y-auto">
                  {runs.length === 0 ? (
                    <div className="py-4 text-center text-text-muted text-xs">No agent runs yet</div>
                  ) : (
                    runs.map((run) => (
                      <button
                        key={run.id}
                        onClick={() => loadRunDetails(run.id)}
                        className="w-full text-left p-2.5 rounded-lg bg-surface-light/30 hover:bg-surface-light/50 transition-colors text-xs"
                      >
                        <div className="flex items-center justify-between">
                          <span className="text-text font-medium truncate">{run.goal}</span>
                          {statusBadge(run.status)}
                        </div>
                        <div className="text-text-muted mt-0.5">
                          {new Date(run.created_at).toLocaleString()}
                        </div>
                      </button>
                    ))
                  )}
                </div>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

// --------------- Step Card (collapsible with parsed output) ---------------

function parseStepOutput(raw: string | undefined): string | null {
  if (!raw || raw === '{}') return null;
  try {
    const parsed = JSON.parse(raw);
    // Common shape: { "output": "..." } or { "error": "..." }
    if (typeof parsed === 'object' && parsed !== null) {
      if (parsed.output) return String(parsed.output);
      if (parsed.error)  return `Error: ${parsed.error}`;
      // Fallback: pretty-print the whole object
      return JSON.stringify(parsed, null, 2);
    }
    return String(parsed);
  } catch {
    return raw; // Not valid JSON — render as-is
  }
}

interface StepCardProps {
  step: AgentStep;
  index: number;
  runId: string;
  statusBadge: (status: string) => ReactNode;
  stepTypeIcon: (type: string) => ReactNode;
  onApprove: (approved: boolean) => void;
}

function StepCard({ step, index, statusBadge, stepTypeIcon, onApprove }: StepCardProps) {
  const [open, setOpen] = useState(
    step.status === AgentStepStatus.AwaitingApproval ||
    step.status === AgentStepStatus.Running
  );
  const output = parseStepOutput(step.output_json);
  const hasContent = !!(step.description || output);

  return (
    <div className="rounded-lg bg-surface-light/50 text-xs overflow-hidden">
      <button
        onClick={() => hasContent && setOpen(!open)}
        className="w-full flex items-center justify-between p-2.5 hover:bg-surface-light/70 transition-colors"
      >
        <span className="font-medium text-text flex items-center gap-1.5">
          {stepTypeIcon(step.type)}
          Step {index + 1}: {step.type}
        </span>
        <div className="flex items-center gap-1">
          {step.duration_ms != null && step.duration_ms > 0 && (
            <span className="text-text-muted text-[10px]">{step.duration_ms}ms</span>
          )}
          {statusBadge(step.status)}
          {hasContent && (open ? <ChevronUp size={10} className="text-text-muted" /> : <ChevronDown size={10} className="text-text-muted" />)}
        </div>
      </button>

      <AnimatePresence>
        {open && hasContent && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            className="overflow-hidden"
          >
            <div className="px-2.5 pb-2.5 space-y-1.5">
              {step.description && <p className="text-text-muted">{step.description}</p>}

              {/* Approval buttons */}
              {step.status === AgentStepStatus.AwaitingApproval && (
                <div className="flex items-center gap-2 py-1">
                  <motion.button
                    whileTap={{ scale: 0.95 }}
                    onClick={() => onApprove(true)}
                    className="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-emerald-400/10 text-emerald-400 hover:bg-emerald-400/20 transition-colors"
                  >
                    <ThumbsUp size={12} /> Approve
                  </motion.button>
                  <motion.button
                    whileTap={{ scale: 0.95 }}
                    onClick={() => onApprove(false)}
                    className="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-red-400/10 text-red-400 hover:bg-red-400/20 transition-colors"
                  >
                    <ThumbsDown size={12} /> Reject
                  </motion.button>
                </div>
              )}

              {output && (
                <pre className="text-text-muted/70 whitespace-pre-wrap break-words bg-surface/50 rounded p-1.5 max-h-40 overflow-y-auto">
                  {output}
                </pre>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
