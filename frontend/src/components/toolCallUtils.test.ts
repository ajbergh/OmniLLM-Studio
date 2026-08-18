import { describe, expect, it } from 'vitest';
import {
  readStringField,
  parseResultJSON,
  hasToolCallDetails,
} from './toolCallUtils';

describe('toolCallUtils', () => {
  it('reads string fields safely from objects', () => {
    expect(readStringField({ screenshot_base64: 'abc' }, 'screenshot_base64')).toBe('abc');
    expect(readStringField({ screenshot_base64: 123 }, 'screenshot_base64')).toBeUndefined();
    expect(readStringField(null, 'url')).toBeUndefined();
    expect(readStringField(undefined, 'url')).toBeUndefined();
    expect(readStringField('not-an-object', 'url')).toBeUndefined();
  });

  it('parses result JSON safely', () => {
    expect(parseResultJSON('{"url":"https://example.com"}')).toEqual({ url: 'https://example.com' });
    expect(parseResultJSON('invalid-json')).toBeNull();
    expect(parseResultJSON('"just a string"')).toBeNull();
  });

  it('determines if tool call has details to expand', () => {
    // When args exist
    expect(hasToolCallDetails({ query: 'test' })).toBe(true);

    // When result exists
    expect(hasToolCallDetails(undefined, { tool_call_id: '1', content: 'output', is_error: false })).toBe(true);

    // When error without result
    expect(hasToolCallDetails(undefined, undefined, 'error')).toBe(true);

    // When running with no args and no result
    expect(hasToolCallDetails(undefined, undefined, 'running')).toBe(false);
    expect(hasToolCallDetails({}, undefined, 'running')).toBe(false);

    // When success with no args and no result
    expect(hasToolCallDetails(undefined, undefined, 'success')).toBe(false);
  });
});
