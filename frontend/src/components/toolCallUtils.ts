import type { ToolResult, ToolCall } from '../types';

export function readStringField(source: unknown, key: string): string | undefined {
  if (!source || typeof source !== 'object') return undefined;
  const value = (source as Record<string, unknown>)[key];
  return typeof value === 'string' && value ? value : undefined;
}

export function parseResultJSON(content: string): Record<string, unknown> | null {
  try {
    const parsed = JSON.parse(content);
    return parsed && typeof parsed === 'object' ? parsed as Record<string, unknown> : null;
  } catch {
    return null;
  }
}

export function getToolCallName(call: ToolCall): string {
  return call.name || call.function?.name || 'tool';
}

export function getToolCallArgs(call: ToolCall): Record<string, unknown> | undefined {
  if (call.arguments) return call.arguments;
  const rawArgs = call.function?.arguments;
  if (!rawArgs) return undefined;
  if (typeof rawArgs === 'object') return rawArgs as Record<string, unknown>;
  try {
    const parsed = JSON.parse(rawArgs);
    return parsed && typeof parsed === 'object' ? parsed as Record<string, unknown> : undefined;
  } catch {
    return { arguments: rawArgs };
  }
}

export function hasToolCallDetails(
  args?: Record<string, unknown>,
  result?: ToolResult,
  status?: 'running' | 'success' | 'error'
): boolean {
  return Boolean(
    (args && Object.keys(args).length > 0) ||
    result ||
    (!result && status === 'error')
  );
}
