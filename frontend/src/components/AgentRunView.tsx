import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { agentApi } from '../api';
import { agentRuntimeApi, type AgentProfile, type AgentStreamEvent } from '../agentRuntimeApi';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import {
  Bot, Play, Square, CheckCircle2, Clock, AlertTriangle,
  ChevronDown, ChevronUp, Loader2, Brain, Wrench, MessageSquare,
  ShieldCheck, XCircle, Pause, RotateCcw, Search, GitBranch,
} from 'lucide-react';
import type { AgentRun, AgentRunWithSteps, AgentStep } from '../types';
import { AgentRunStatus, AgentStepStatus, AgentStepType, AgentEventType } from '../types';

interface AgentRunViewProps {
  conversationId: string;
}

interface ParsedAgentEvent {
  type?: string;
  run_id?: string;
  step_id?: string;
  data?: Record<string, unknown>;
  id?: string;
  status?: string;
}

const eventLabels: Record<string, string> = {
  agent_plan: 'Plan created',
  agent_step_start: 'Running a step',
  agent_step_complete: 'Step completed',
  agent_approval_required: 'Waiting for approval',
  agent_retry: 'Retrying a failed step',
  agent_replan: 'Repairing the plan',
  agent_checkpoint: 'Checkpoint saved',
  agent_tool: 'Using a tool',
  agent_complete: 'Run completed',
  agent_error: 'Run encountered an error',
};

export function AgentRunView({ conversationId }: AgentRunViewProps) {
  const [runs, setRuns] = useState<AgentRun[]>([]);
  const [activeRun, setActiveRun] = useState<AgentRunWithSteps | null>(null);
  const [starting, setStarting] = useState(false);
  const [goal, setGoal] = useState('');
  const [expanded, setExpanded] = useState(true);
  const [profile, setProfile] = useState<AgentProfile>('agent');
  const [activity, setActivity] = useState<string | null>(null);
  const [extendedBudget, setExtendedBudget] = useState(false);
  const abortRef = useRef<(() => void) | null>(null);
  const activeRunIdRef = useRef<string | null>(null);
  const reloadTimerRef = useRef<number | null>(null);

  const fetchRuns = useCallback(async () => {
    try {
      setRuns((await agentApi.listRuns(conversationId)) || []);
    } catch {
      // The panel remains useful even if the first history fetch races startup.
    }
  }, [conversationId]);

  const loadRunDetails = useCallback(async (runId: string) => {
    try {
      const details = await agentApi.getRun(runId);
      setActiveRun(details);
      activeRunIdRef.current = runId;
    } catch (error) {
      toast.error((error as Error).message);
    }
  }, []);

  const scheduleReload = useCallback((runId?: string) => {
    const target = runId || activeRunIdRef.current;
    if (!target) return;
    if (reloadTimerRef.current !== null) window.clearTimeout(reloadTimerRef.current);
    reloadTimerRef.current = window.setTimeout(() => {
      void loadRunDetails(target);
      void fetchRuns();
      reloadTimerRef.current = null;
    }, 150);
  }, [fetchRuns, loadRunDetails]);

  useEffect(() => {
    if (expanded) void fetchRuns();
  }, [expanded, fetchRuns]);

  useEffect(() => () => {
    abortRef.current?.();
    if (reloadTimerRef.current !== null) window.clearTimeout(reloadTimerRef.current);
  }, []);

  const handleStreamEvent = useCallback((event: AgentStreamEvent) => {
    let payload: ParsedAgentEvent = {};
    try { payload = event.data ? JSON.parse(event.data) as ParsedAgentEvent : {}; } catch { /* ignore malformed individual event */ }
    const runId = payload.run_id || payload.id || activeRunIdRef.current || undefined;
    if (runId) activeRunIdRef.current = runId;
    setActivity(eventLabels[event.type] || event.type.replaceAll('_', ' '));

    switch (event.type) {
      case AgentEventType.ApprovalRequired:
        toast.info('Agent is waiting for approval');
        scheduleReload(runId);
        break;
      case 'agent_retry':
        toast.info('Retrying a transient failure');
        scheduleReload(runId);
        break;
      case 'agent_replan':
        toast.info('The agent repaired its remaining plan');
        scheduleReload(runId);
        break;
      case AgentEventType.Complete:
        toast.success('Agent run completed');
        scheduleReload(runId);
        break;
      case AgentEventType.Error:
        toast.error(String(payload.data?.error || 'Agent run failed'));
        scheduleReload(runId);
        break;
      case 'done':
        setActivity(null);
        scheduleReload(runId);
        break;
      default:
        scheduleReload(runId);
        break;
    }
  }, [scheduleReload]);

  const runStream = async (factory: () => { promise: Promise<void>; abort: () => void }) => {
    setStarting(true);
    try {
      const stream = factory();
      abortRef.current = stream.abort;
      await stream.promise;
      await fetchRuns();
      if (activeRunIdRef.current) await loadRunDetails(activeRunIdRef.current);
    } catch (error) {
      toast.error(`Agent run failed: ${(error as Error).message}`);
    } finally {
      setStarting(false);
      setActivity(null);
      abortRef.current = null;
    }
  };

  const startRun = async () => {
    if (!goal.trim()) return;
    const submittedGoal = goal.trim();
    setGoal('');
    await runStream(() => agentRuntimeApi.startRun(
      conversationId,
      {
        goal: submittedGoal,
        profile,
        budgets: extendedBudget
          ? { max_steps: 20, max_duration_ms: 600000, max_model_calls: 30, max_tool_calls: 40 }
          : { max_steps: 10, max_duration_ms: 300000, max_model_calls: 16, max_tool_calls: 20 },
      },
      handleStreamEvent,
    ));
  };

  const resumeRun = async (runId: string) => {
    activeRunIdRef.current = runId;
    await runStream(() => agentRuntimeApi.resumeRun(runId, handleStreamEvent));
  };

  const approveStep = async (runId: string, stepId: string, approved: boolean) => {
    try {
      await agentApi.approveStep(runId, stepId, approved);
      toast.success(approved ? 'Step approved' : 'Step rejected');
      scheduleReload(runId);
    } catch (error) {
      toast.error((error as Error).message);
    }
  };

  const pauseRun = async (runId: string) => {
    try {
      await agentRuntimeApi.pauseRun(runId);
      abortRef.current?.();
      abortRef.current = null;
      toast.success('Run paused at its latest checkpoint');
      scheduleReload(runId);
    } catch (error) {
      toast.error((error as Error).message);
    }
  };

  const cancelRun = async (runId: string) => {
    try {
      abortRef.current?.();
      abortRef.current = null;
      await agentApi.cancelRun(runId);
      toast.success('Run cancelled');
      await fetchRuns();
      await loadRunDetails(runId);
    } catch (error) {
      toast.error((error as Error).message);
    }
  };

  const stepTypeIcon = (type: string) => {
    switch (type) {
      case AgentStepType.Think: return <Brain size={11} />;
      case AgentStepType.ToolCall: return <Wrench size={11} />;
      case AgentStepType.Message: return <MessageSquare size={11} />;
      case AgentStepType.Approval: return <ShieldCheck size={11} />;
      default: return <Clock size={11} />;
    }
  };

  const statusBadge = (status: string) => {
    const config: Record<string, { icon: typeof Clock; className: string }> = {
      [AgentRunStatus.Planning]: { icon: Clock, className: 'text-purple-400 bg-purple-400/10' },
      [AgentStepStatus.Pending]: { icon: Clock, className: 'text-amber-400 bg-amber-400/10' },
      [AgentRunStatus.Running]: { icon: Loader2, className: 'text-blue-400 bg-blue-400/10' },
      [AgentRunStatus.AwaitingApproval]: { icon: ShieldCheck, className: 'text-amber-400 bg-amber-400/10' },
      [AgentRunStatus.Paused]: { icon: Pause, className: 'text-yellow-400 bg-yellow-400/10' },
      [AgentRunStatus.Completed]: { icon: CheckCircle2, className: 'text-emerald-400 bg-emerald-400/10' },
      [AgentRunStatus.Failed]: { icon: XCircle, className: 'text-red-400 bg-red-400/10' },
      [AgentRunStatus.Cancelled]: { icon: Square, className: 'text-gray-400 bg-gray-400/10' },
      [AgentStepStatus.Skipped]: { icon: AlertTriangle, className: 'text-gray-400 bg-gray-400/10' },
    };
    const entry = config[status] || { icon: Clock, className: 'text-gray-400 bg-gray-400/10' };
    const Icon = entry.icon;
    return <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] ${entry.className}`}><Icon size={10} className={status === AgentRunStatus.Running ? 'animate-spin' : ''} />{status}</span>;
  };

  const canControl = activeRun && [AgentRunStatus.Planning, AgentRunStatus.Running, AgentRunStatus.AwaitingApproval].includes(activeRun.status);

  return (
    <div className="glass overflow-hidden rounded-xl">
      <button onClick={() => setExpanded((value) => !value)} className="flex w-full items-center justify-between px-4 py-3 text-sm hover:bg-surface-light/50">
        <div className="flex items-center gap-2 text-text-muted">
          <Bot size={14} /><span className="font-medium">Agent Runtime</span>
          {runs.length > 0 && <span className="rounded-full bg-primary/20 px-1.5 py-0.5 text-xs text-primary">{runs.length}</span>}
          {activity && <span className="inline-flex items-center gap-1 text-[10px] text-blue-300"><Loader2 size={9} className="animate-spin" />{activity}</span>}
        </div>
        {expanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
      </button>

      <AnimatePresence>
        {expanded && (
          <motion.div initial={{ height: 0, opacity: 0 }} animate={{ height: 'auto', opacity: 1 }} exit={{ height: 0, opacity: 0 }} className="overflow-hidden">
            <div className="space-y-3 px-4 pb-4">
              <div className="flex flex-wrap items-center gap-2">
                <div className="inline-flex rounded-lg border border-border bg-surface-alt p-0.5">
                  <button onClick={() => setProfile('agent')} className={`inline-flex items-center gap-1 rounded-md px-2 py-1 text-[10px] ${profile === 'agent' ? 'bg-amber-500/20 text-amber-300' : 'text-text-muted'}`}><Bot size={10} />Agent</button>
                  <button onClick={() => setProfile('research')} className={`inline-flex items-center gap-1 rounded-md px-2 py-1 text-[10px] ${profile === 'research' ? 'bg-blue-500/20 text-blue-300' : 'text-text-muted'}`}><Search size={10} />Research</button>
                </div>
                <label className="inline-flex items-center gap-1.5 text-[10px] text-text-muted">
                  <input type="checkbox" checked={extendedBudget} onChange={(event) => setExtendedBudget(event.target.checked)} />Extended budget
                </label>
              </div>

              <div className="flex gap-2">
                <input value={goal} onChange={(event) => setGoal(event.target.value)} placeholder={profile === 'research' ? 'Describe the research goal…' : 'Describe the goal…'} className="flex-1 rounded-lg border border-border bg-surface-light px-3 py-2 text-sm text-text outline-none focus:border-primary/50" onKeyDown={(event) => event.key === 'Enter' && void startRun()} />
                <button onClick={() => void startRun()} disabled={starting || !goal.trim()} className="btn-primary inline-flex items-center gap-1.5 rounded-lg px-3 py-2 text-xs font-medium disabled:opacity-50">{starting ? <Loader2 size={12} className="animate-spin" /> : <Play size={12} />}Run</button>
              </div>

              {activeRun ? (
                <div className="space-y-2">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0"><p className="truncate text-sm font-medium text-text">{activeRun.goal}</p><p className="text-[10px] text-text-muted">Updated {new Date(activeRun.updated_at).toLocaleString()}</p></div>
                    <div className="flex items-center gap-1.5">
                      {statusBadge(activeRun.status)}
                      {canControl && <button onClick={() => void pauseRun(activeRun.id)} className="rounded p-1 text-yellow-400 hover:bg-yellow-400/10" title="Pause at checkpoint"><Pause size={12} /></button>}
                      {canControl && <button onClick={() => void cancelRun(activeRun.id)} className="rounded p-1 text-red-400 hover:bg-red-400/10" title="Cancel run"><Square size={12} /></button>}
                      {activeRun.status === AgentRunStatus.Paused && <button onClick={() => void resumeRun(activeRun.id)} disabled={starting} className="rounded p-1 text-emerald-400 hover:bg-emerald-400/10" title="Resume run"><RotateCcw size={12} /></button>}
                    </div>
                  </div>

                  {activeRun.result_summary && <div className="rounded-lg border border-emerald-400/20 bg-emerald-400/5 p-2.5 text-xs text-emerald-300">{activeRun.result_summary}</div>}

                  <div className="space-y-1.5">
                    {activeRun.steps?.map((step, index) => (
                      <StepCard key={step.id} step={step} index={index} statusBadge={statusBadge} stepTypeIcon={stepTypeIcon} onApprove={(approved) => void approveStep(activeRun.id, step.id, approved)} />
                    ))}
                  </div>
                  <button onClick={() => setActiveRun(null)} className="text-xs text-text-muted hover:text-text">← Run history</button>
                </div>
              ) : (
                <div className="max-h-52 space-y-1.5 overflow-y-auto">
                  {runs.length === 0 ? <div className="py-4 text-center text-xs text-text-muted">No agent runs yet</div> : runs.map((run) => (
                    <button key={run.id} onClick={() => void loadRunDetails(run.id)} className="w-full rounded-lg bg-surface-light/30 p-2.5 text-left text-xs hover:bg-surface-light/50">
                      <div className="flex items-center justify-between gap-2"><span className="truncate font-medium text-text">{run.goal}</span>{statusBadge(run.status)}</div>
                      <div className="mt-1 flex items-center gap-1 text-[10px] text-text-muted"><GitBranch size={9} />{new Date(run.created_at).toLocaleString()}</div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

function parseStepOutput(raw?: string): string | null {
  if (!raw || raw === '{}') return null;
  try {
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === 'object') {
      if (parsed.output) return String(parsed.output);
      if (parsed.error) return `Error: ${parsed.error}`;
      return JSON.stringify(parsed, null, 2);
    }
    return String(parsed);
  } catch { return raw; }
}

function StepCard({ step, index, statusBadge, stepTypeIcon, onApprove }: {
  step: AgentStep;
  index: number;
  statusBadge: (status: string) => ReactNode;
  stepTypeIcon: (type: string) => ReactNode;
  onApprove: (approved: boolean) => void;
}) {
  const [open, setOpen] = useState(step.status === AgentStepStatus.AwaitingApproval || step.status === AgentStepStatus.Running);
  const output = parseStepOutput(step.output_json);
  return (
    <div className="overflow-hidden rounded-lg bg-surface-light/50 text-xs">
      <button onClick={() => setOpen((value) => !value)} className="flex w-full items-center gap-2 p-2.5 text-left">
        <span className="text-text-muted">{stepTypeIcon(step.type)}</span>
        <span className="w-5 shrink-0 text-[10px] text-text-muted">{index + 1}</span>
        <span className="min-w-0 flex-1 truncate text-text-secondary">{step.description}</span>
        {statusBadge(step.status)}
        {open ? <ChevronUp size={11} /> : <ChevronDown size={11} />}
      </button>
      {open && (
        <div className="space-y-2 border-t border-border/40 px-3 py-2.5">
          {step.tool_name && <div className="inline-flex items-center gap-1 rounded bg-primary/10 px-2 py-1 font-mono text-[10px] text-primary"><Wrench size={9} />{step.tool_name}</div>}
          {step.input_json && step.input_json !== '{}' && <pre className="max-h-32 overflow-auto whitespace-pre-wrap rounded bg-surface-alt p-2 text-[10px] text-text-muted">{step.input_json}</pre>}
          {output && <pre className="max-h-44 overflow-auto whitespace-pre-wrap rounded bg-surface-alt p-2 text-[10px] text-text-secondary">{output}</pre>}
          {step.duration_ms !== undefined && <p className="text-[10px] text-text-muted">{(step.duration_ms / 1000).toFixed(2)}s</p>}
          {step.status === AgentStepStatus.AwaitingApproval && (
            <div className="grid grid-cols-2 gap-2">
              <button onClick={() => onApprove(false)} className="rounded-lg border border-border px-2 py-1.5 text-red-300 hover:bg-red-500/10">Reject</button>
              <button onClick={() => onApprove(true)} className="rounded-lg bg-amber-500 px-2 py-1.5 font-medium text-black hover:bg-amber-400">Approve</button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
