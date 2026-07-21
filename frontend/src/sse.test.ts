import { describe, expect, it } from 'vitest';
import { SSEDecoder } from './sse';

const fixture = 'event: tool_result\ndata: {"tool_call_id":"abc","result":{"content":"ok"}}\n\nevent: done\ndata: {"message_id":"m1"}\n\n';

describe('SSEDecoder', () => {
  it('preserves event framing across every byte boundary', () => {
    for (let split = 0; split <= fixture.length; split++) {
      const decoder = new SSEDecoder('token');
      const events = [
        ...decoder.push(fixture.slice(0, split)),
        ...decoder.push(fixture.slice(split)),
      ];
      expect(events).toHaveLength(2);
      expect(events[0].event).toBe('tool_result');
      expect(JSON.parse(events[0].data).tool_call_id).toBe('abc');
      expect(events[1].event).toBe('done');
      expect(decoder.finish().incomplete).toBe(false);
    }
  });

  it('supports CRLF and multiline data fields', () => {
    const decoder = new SSEDecoder();
    const events = decoder.push('event: message\r\ndata: first\r\ndata: second\r\n\r\n');
    expect(events).toEqual([{ event: 'message', data: 'first\nsecond' }]);
  });

  it('does not dispatch an unterminated partial event', () => {
    const decoder = new SSEDecoder('token');
    expect(decoder.push('event: token\ndata: {"content":"partial"}')).toEqual([]);
    expect(decoder.finish()).toEqual({ events: [], incomplete: true });
  });

  it('ignores comments while retaining the event payload', () => {
    const decoder = new SSEDecoder();
    const events = decoder.push(': heartbeat\nevent: heartbeat\ndata: {}\n\n');
    expect(events).toEqual([{ event: 'heartbeat', data: '{}' }]);
  });
});
