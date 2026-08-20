import { describe, expect, it } from 'vitest';
import { renderToString } from 'react-dom/server';
import { ToolCallsList } from './ToolCallsList';
import type { ToolCall, ToolResult } from '../types';

describe('ToolCallsList', () => {
  it('renders nothing when toolCalls is empty', () => {
    const html = renderToString(<ToolCallsList toolCalls={[]} />);
    expect(html).toBe('');
  });

  it('renders single tool call compact card', () => {
    const toolCalls: ToolCall[] = [
      { id: 'call_1', name: 'tool_search', arguments: { query: 'test' } },
    ];
    const toolResults: ToolResult[] = [
      { tool_call_id: 'call_1', content: 'results', is_error: false },
    ];
    const html = renderToString(<ToolCallsList toolCalls={toolCalls} toolResults={toolResults} />);
    expect(html).toContain('tool_search');
  });

  it('renders collapsed summary count and badge for multiple completed tools', () => {
    const toolCalls: ToolCall[] = [
      { id: 'call_1', name: 'tool_search', arguments: { query: 'test' } },
      { id: 'call_2', name: 'tool_invoke', arguments: { name: 'fetch' } },
    ];
    const toolResults: ToolResult[] = [
      { tool_call_id: 'call_1', content: 'ok', is_error: false },
      { tool_call_id: 'call_2', content: 'ok', is_error: false },
    ];
    const html = renderToString(<ToolCallsList toolCalls={toolCalls} toolResults={toolResults} />);
    expect(html).toContain('tool');
    expect(html).toContain('calls');
    expect(html).toContain('2');
  });

  it('shows running spinner status for running tool calls', () => {
    const toolCalls: ToolCall[] = [
      { id: 'call_1', name: 'tool_search', arguments: { query: 'test' } },
      { id: 'call_2', name: 'tool_invoke', arguments: { name: 'fetch' } },
    ];
    const toolResults: ToolResult[] = [
      { tool_call_id: 'call_1', content: 'ok', is_error: false },
    ];
    const html = renderToString(<ToolCallsList toolCalls={toolCalls} toolResults={toolResults} />);
    expect(html).toContain('tool');
    expect(html).toContain('calls');
    // Running count 1, Success count 1
    expect(html).toContain('animate-spin');
  });
});
