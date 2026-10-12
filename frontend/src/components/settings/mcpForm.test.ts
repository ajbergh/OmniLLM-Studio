import { describe, expect, it } from 'vitest';
import {
  createMCPRequest,
  emptyMCPForm,
  formFromMCPServer,
  parseMCPArgs,
  parseMCPEnv,
  updateMCPRequest,
} from './mcpForm';

describe('MCP form parsing and request boundaries', () => {
  it('normalizes stdio command and retains literal values after first equals', () => {
    const request = createMCPRequest({
      ...emptyMCPForm,
      name: ' filesystem ',
      command: ' npx ',
      argsText: '\n  -y \n @modelcontextprotocol/server-filesystem \n\n',
      envText: 'KEY=first=second\n EMPTY=\n',
    });
    expect(request).toMatchObject({
      name: 'filesystem',
      transport: 'stdio',
      command: 'npx',
      args: ['-y', '@modelcontextprotocol/server-filesystem'],
      env: { KEY: 'first=second', EMPTY: '' },
    });
    expect(parseMCPArgs(' a \n\n b ')).toEqual(['a', 'b']);
    expect(parseMCPEnv('BEARER=abc=def', 'Headers')).toEqual({ BEARER: 'abc=def' });
  });

  it('validates required fields and rejects malformed key-value lines', () => {
    expect(() => createMCPRequest(emptyMCPForm)).toThrow('Server name is required');
    expect(() => createMCPRequest({ ...emptyMCPForm, name: 'local' })).toThrow('Command is required');
    expect(() => parseMCPEnv('MISSING_VALUE', 'Headers')).toThrow('Headers must use KEY=value');
    expect(() => parseMCPEnv('=VALUE', 'Headers')).toThrow('Headers must use KEY=value');
    expect(() => createMCPRequest({ ...emptyMCPForm, name: 'remote', transport: 'http' })).toThrow('URL is required');
  });

  it('supports private-network HTTP opt-in, never persisting secrets in edit forms', () => {
    const req = updateMCPRequest({
      ...emptyMCPForm,
      name: 'remote',
      transport: 'http',
      url: ' https://example.org/mcp ',
      headersText: 'Authorization=Bearer test',
      allowPrivateNetwork: true,
    });
    expect(req).toMatchObject({
      name: 'remote',
      transport: 'http',
      url: 'https://example.org/mcp',
      allow_private_network: true,
      headers: { Authorization: 'Bearer test' },
    });
    const editing = formFromMCPServer({
      id: 'id', name: 'remote', transport: 'http', enabled: true,
      url: 'https://example.org/mcp', allow_private_network: false,
    } as Parameters<typeof formFromMCPServer>[0]);
    expect(editing.headersText).toBe('');
    expect(editing.envText).toBe('');
    expect(editing.allowPrivateNetwork).toBe(false);
  });
});
