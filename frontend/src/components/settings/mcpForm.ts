// MCP server form projection and validation. Extracted without changing behavior.
import type { CreateMCPServerRequest, MCPServer, MCPTransport, UpdateMCPServerRequest } from '../../types';

export type MCPFormState = {
  name: string;
  transport: MCPTransport;
  command: string;
  argsText: string;
  url: string;
  envText: string;
  headersText: string;
  allowPrivateNetwork: boolean;
  enabled: boolean;
};

export const emptyMCPForm: MCPFormState = {
  name: '',
  transport: 'stdio',
  command: '',
  argsText: '',
  url: '',
  envText: '',
  headersText: '',
  allowPrivateNetwork: false,
  enabled: false,
};

export const filesystemMCPTemplate: MCPFormState = {
  name: 'filesystem',
  transport: 'stdio',
  command: 'npx.cmd',
  argsText: [
    '-y',
    '@modelcontextprotocol/server-filesystem@2025.8.21',
    'C:\\Users\\you\\Documents',
  ].join('\n'),
  url: '',
  envText: '',
  headersText: '',
  allowPrivateNetwork: false,
  enabled: false,
};

export function parseMCPArgs(value: string): string[] {
  return value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
}

export function parseMCPEnv(value: string, label: string): Record<string, string> {
  const env: Record<string, string> = {};
  for (const rawLine of value.split('\n')) {
    const line = rawLine.trim();
    if (!line) continue;
    const equals = line.indexOf('=');
    if (equals <= 0) {
      throw new Error(`${label} must use KEY=value lines`);
    }
    const key = line.slice(0, equals).trim();
    const envValue = line.slice(equals + 1).trim();
    if (!key) {
      throw new Error(`${label} keys cannot be empty`);
    }
    env[key] = envValue;
  }
  return env;
}

export function createMCPRequest(form: MCPFormState): CreateMCPServerRequest {
  const name = form.name.trim();
  const transport = form.transport;
  if (!name) throw new Error('Server name is required');
  
  const request: CreateMCPServerRequest = {
    name,
    transport,
    enabled: form.enabled,
  };

  if (transport === 'stdio') {
    const command = form.command.trim();
    if (!command) throw new Error('Command is required for stdio transport');
    request.command = command;
    request.args = parseMCPArgs(form.argsText);
    if (form.envText.trim()) {
      request.env = parseMCPEnv(form.envText, 'Environment variables');
    }
  } else if (transport === 'http') {
    const url = form.url.trim();
    if (!url) throw new Error('URL is required for http transport');
    request.url = url;
    request.allow_private_network = form.allowPrivateNetwork;
    if (form.headersText.trim()) {
      request.headers = parseMCPEnv(form.headersText, 'Headers');
    }
  }

  return request;
}

export function updateMCPRequest(form: MCPFormState): UpdateMCPServerRequest {
  const request = createMCPRequest(form) as UpdateMCPServerRequest;
  if (form.transport === 'stdio' && !form.envText.trim()) {
    delete request.env;
  }
  if (form.transport === 'http' && !form.headersText.trim()) {
    delete request.headers;
  }
  return request;
}

export function formFromMCPServer(server: MCPServer): MCPFormState {
  return {
    name: server.name,
    transport: server.transport || 'stdio',
    command: server.command || '',
    argsText: (server.args || []).join('\n'),
    url: server.url || '',
    envText: '',
    headersText: '',
    allowPrivateNetwork: server.allow_private_network ?? false,
    enabled: server.enabled,
  };
}

