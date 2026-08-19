import type { CreateMCPServerRequest, MCPTransport } from './types';

export interface ParsedMCPServerConfig {
  name: string;
  transport: MCPTransport;
  command?: string;
  args?: string[];
  url?: string;
  env?: Record<string, string>;
  headers?: Record<string, string>;
  allow_private_network?: boolean;
}

/**
 * Tokenize shell-like command strings handling single and double quotes, escapes, and backslash line continuations.
 */
export function tokenizeShellCommand(input: string): string[] {
  const tokens: string[] = [];
  let current = '';
  let inDouble = false;
  let inSingle = false;
  let escaping = false;

  // Normalize multi-line continuations (backslash at end of line)
  const normalized = input.replace(/\\\r?\n/g, ' ');

  for (let i = 0; i < normalized.length; i++) {
    const char = normalized[i];

    if (escaping) {
      current += char;
      escaping = false;
      continue;
    }

    if (char === '\\' && !inSingle) {
      escaping = true;
      continue;
    }

    if (char === '"' && !inSingle) {
      inDouble = !inDouble;
      continue;
    }

    if (char === "'" && !inDouble) {
      inSingle = !inSingle;
      continue;
    }

    if (!inDouble && !inSingle && /\s/.test(char)) {
      if (current.length > 0) {
        tokens.push(current);
        current = '';
      }
      continue;
    }

    current += char;
  }

  if (current.length > 0) {
    tokens.push(current);
  }

  return tokens;
}

/**
 * Parse a Header or Env string in "Key: Value" or "Key=Value" format into [key, value].
 */
export function parseKeyValuePair(entry: string, preferredSeparator?: ':' | '='): [string, string] | null {
  const trimmed = entry.trim();
  if (!trimmed) return null;

  let sepIndex = -1;
  if (preferredSeparator) {
    sepIndex = trimmed.indexOf(preferredSeparator);
  }

  if (sepIndex === -1) {
    const colonIdx = trimmed.indexOf(':');
    const equalIdx = trimmed.indexOf('=');

    if (colonIdx > 0 && equalIdx > 0) {
      sepIndex = Math.min(colonIdx, equalIdx);
    } else if (colonIdx > 0) {
      sepIndex = colonIdx;
    } else if (equalIdx > 0) {
      sepIndex = equalIdx;
    }
  }

  if (sepIndex <= 0) return null;

  const key = trimmed.slice(0, sepIndex).trim();
  const val = trimmed.slice(sepIndex + 1).trim();
  if (!key) return null;

  return [key, val];
}

/**
 * Derive a default server name from a URL or command.
 */
export function deriveServerName(urlOrCmd: string): string {
  try {
    if (urlOrCmd.startsWith('http://') || urlOrCmd.startsWith('https://')) {
      const u = new URL(urlOrCmd);
      const hostParts = u.hostname.split('.');
      if (hostParts.length > 1) {
        // e.g. mcp.neon.tech -> neon, or api.github.com -> github
        const candidate = hostParts[hostParts.length - 2];
        if (candidate && candidate.length > 2 && !['com', 'org', 'net', 'tech', 'io', 'co', 'app'].includes(candidate)) {
          return candidate.charAt(0).toUpperCase() + candidate.slice(1);
        }
      }
      return u.hostname.replace(/[^a-zA-Z0-9_-]/g, '_');
    }
  } catch {
    // fallback
  }

  const base = urlOrCmd.split(/[/\\]/).pop() || 'mcp-server';
  return base.replace(/[^a-zA-Z0-9_-]/g, '_') || 'mcp-server';
}

/**
 * Parses CLI commands like:
 * npx add-mcp@latest "https://mcp.neon.tech/mcp?projectId=xxxx" --name Neon --header "Authorization: Bearer napi_xxx"
 * or
 * npx -y @modelcontextprotocol/server-filesystem /path/to/dir
 * or
 * claude mcp add --transport sse Neon https://mcp.neon.tech/mcp --header "Authorization: Bearer napi_xxx"
 * or
 * mcp add Neon https://... -H "Authorization: Bearer ..."
 */
export function parseCLICommand(input: string): ParsedMCPServerConfig | null {
  const tokens = tokenizeShellCommand(input.trim());
  if (tokens.length === 0) return null;

  let name: string | undefined;
  let transport: MCPTransport | undefined;
  let url: string | undefined;
  let command: string | undefined;
  const args: string[] = [];
  const env: Record<string, string> = {};
  const headers: Record<string, string> = {};

  // Check if it's an add-mcp command, e.g. `npx add-mcp@latest <url> [flags]`
  const isAddMcp = tokens.some((t) => /add-mcp/i.test(t));
  const isClaudeMcpAdd = tokens.some((t) => t === 'mcp') && tokens.some((t) => t === 'add');

  let i = 0;

  // Handle leading CLI runners
  if (tokens[0] === 'npx' || tokens[0] === 'npx.cmd' || tokens[0] === 'bunx' || tokens[0] === 'uvx' || tokens[0] === 'pnpx') {
    if (isAddMcp) {
      // Skip npx / bunx / uvx and the add-mcp package specifier
      i = 1;
      while (i < tokens.length && tokens[i].startsWith('-')) {
        i++; // skip options like -y
      }
      if (i < tokens.length && /add-mcp/i.test(tokens[i])) {
        i++; // skip add-mcp@latest
      }
    }
  } else if (isClaudeMcpAdd) {
    // skip up to 'add'
    while (i < tokens.length && tokens[i] !== 'add') {
      i++;
    }
    if (i < tokens.length && tokens[i] === 'add') {
      i++;
    }
  }

  // Parse remaining arguments
  const positionalArgs: string[] = [];

  while (i < tokens.length) {
    const token = tokens[i];

    if (token === '--name' || token === '-n') {
      if (i + 1 < tokens.length) {
        name = tokens[i + 1];
        i += 2;
        continue;
      }
    } else if (token.startsWith('--name=')) {
      name = token.slice('--name='.length);
      i++;
      continue;
    } else if (token === '--header' || token === '-H' || token === '--headers') {
      if (i + 1 < tokens.length) {
        const pair = parseKeyValuePair(tokens[i + 1]);
        if (pair) headers[pair[0]] = pair[1];
        i += 2;
        continue;
      }
    } else if (token.startsWith('--header=') || token.startsWith('-H=')) {
      const val = token.slice(token.indexOf('=') + 1);
      const pair = parseKeyValuePair(val);
      if (pair) headers[pair[0]] = pair[1];
      i++;
      continue;
    } else if (token === '--env' || token === '-e') {
      if (i + 1 < tokens.length) {
        const pair = parseKeyValuePair(tokens[i + 1]);
        if (pair) env[pair[0]] = pair[1];
        i += 2;
        continue;
      }
    } else if (token.startsWith('--env=') || token.startsWith('-e=')) {
      const val = token.slice(token.indexOf('=') + 1);
      const pair = parseKeyValuePair(val);
      if (pair) env[pair[0]] = pair[1];
      i++;
      continue;
    } else if (token === '--transport' || token === '-t') {
      if (i + 1 < tokens.length) {
        const t = tokens[i + 1].toLowerCase();
        if (t === 'http' || t === 'sse' || t === 'streamable') transport = 'http';
        else if (t === 'stdio') transport = 'stdio';
        i += 2;
        continue;
      }
    } else if (token.startsWith('--transport=')) {
      const t = token.slice('--transport='.length).toLowerCase();
      if (t === 'http' || t === 'sse' || t === 'streamable') transport = 'http';
      else if (t === 'stdio') transport = 'stdio';
      i++;
      continue;
    } else if (token === '--url' || token === '-u') {
      if (i + 1 < tokens.length) {
        url = tokens[i + 1];
        transport = 'http';
        i += 2;
        continue;
      }
    } else if (token.startsWith('--url=')) {
      url = token.slice('--url='.length);
      transport = 'http';
      i++;
      continue;
    } else if (token === '--command' || token === '-c') {
      if (i + 1 < tokens.length) {
        command = tokens[i + 1];
        transport = 'stdio';
        i += 2;
        continue;
      }
    } else if (token.startsWith('--command=')) {
      command = token.slice('--command='.length);
      transport = 'stdio';
      i++;
      continue;
    } else if (token === '--') {
      i++;
      while (i < tokens.length) {
        args.push(tokens[i]);
        i++;
      }
      break;
    } else {
      positionalArgs.push(token);
      i++;
    }
  }

  // Handle positional arguments
  if (isAddMcp || isClaudeMcpAdd) {
    for (const arg of positionalArgs) {
      if (arg.startsWith('http://') || arg.startsWith('https://')) {
        url = arg;
        transport = 'http';
      } else if (!name && !arg.startsWith('-')) {
        name = arg;
      }
    }
  } else {
    // Raw command like: `npx -y @modelcontextprotocol/server-filesystem /path`
    // or `node server.js`
    if (positionalArgs.length > 0) {
      // Check if first arg is an http url
      if (positionalArgs[0].startsWith('http://') || positionalArgs[0].startsWith('https://')) {
        url = positionalArgs[0];
        transport = 'http';
      } else {
        command = positionalArgs[0];
        args.push(...positionalArgs.slice(1));
        transport = 'stdio';
      }
    }
  }

  if (url) {
    transport = 'http';
  } else if (command) {
    transport = 'stdio';
  }

  if (!transport) {
    return null;
  }

  if (!name) {
    if (url) {
      name = deriveServerName(url);
    } else if (command) {
      // Check args if command is npx/node/python
      const firstArg = args.find((a) => !a.startsWith('-'));
      if (firstArg) {
        name = deriveServerName(firstArg);
      } else {
        name = deriveServerName(command);
      }
    } else {
      name = 'mcp-server';
    }
  }

  return {
    name,
    transport,
    command,
    args: args.length > 0 ? args : undefined,
    url,
    env: Object.keys(env).length > 0 ? env : undefined,
    headers: Object.keys(headers).length > 0 ? headers : undefined,
  };
}

/**
 * Parses JSON configurations such as Claude Desktop / Cursor / VSCode MCP configs.
 *
 * Examples:
 * {
 *   "mcpServers": {
 *     "Neon": {
 *       "type": "http",
 *       "url": "https://mcp.neon.tech/mcp?projectId=xxxxx",
 *       "headers": {
 *         "Authorization": "Bearer napi_xxx"
 *       }
 *     },
 *     "filesystem": {
 *       "command": "npx",
 *       "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path"],
 *       "env": { "FOO": "bar" }
 *     }
 *   }
 * }
 * or direct single server object:
 * {
 *   "name": "Neon",
 *   "type": "http",
 *   "url": "...",
 *   "headers": { ... }
 * }
 */
export function parseJSONConfig(jsonString: string): ParsedMCPServerConfig[] {
  let parsed: unknown;
  try {
    parsed = JSON.parse(jsonString);
  } catch {
    return [];
  }

  if (!parsed || typeof parsed !== 'object') {
    return [];
  }

  const results: ParsedMCPServerConfig[] = [];
  const record = parsed as Record<string, unknown>;

  // Check if it has "mcpServers" or "servers" root key
  const serverMap = (record.mcpServers || record.servers) as Record<string, unknown> | undefined;

  if (serverMap && typeof serverMap === 'object' && !Array.isArray(serverMap)) {
    for (const [serverKey, rawVal] of Object.entries(serverMap)) {
      if (rawVal && typeof rawVal === 'object') {
        const item = rawVal as Record<string, unknown>;
        const single = parseSingleServerJSON(serverKey, item);
        if (single) results.push(single);
      }
    }
    return results;
  }

  // Check if it's an array of server objects
  if (Array.isArray(parsed)) {
    for (const item of parsed) {
      if (item && typeof item === 'object') {
        const single = parseSingleServerJSON(undefined, item as Record<string, unknown>);
        if (single) results.push(single);
      }
    }
    return results;
  }

  // Check if it's a single server configuration directly
  const single = parseSingleServerJSON(typeof record.name === 'string' ? record.name : undefined, record);
  if (single) {
    results.push(single);
  }

  return results;
}

function parseSingleServerJSON(keyName: string | undefined, obj: Record<string, unknown>): ParsedMCPServerConfig | null {
  const name = (typeof obj.name === 'string' && obj.name.trim()) || keyName || 'mcp-server';
  const typeStr = typeof obj.type === 'string' ? obj.type.toLowerCase() : typeof obj.transport === 'string' ? obj.transport.toLowerCase() : '';
  const url = typeof obj.url === 'string' ? obj.url.trim() : undefined;
  const command = typeof obj.command === 'string' ? obj.command.trim() : undefined;

  let transport: MCPTransport;
  if (typeStr === 'http' || typeStr === 'sse' || typeStr === 'streamable' || url) {
    transport = 'http';
  } else if (typeStr === 'stdio' || command) {
    transport = 'stdio';
  } else {
    return null;
  }

  let args: string[] | undefined;
  if (Array.isArray(obj.args)) {
    args = obj.args.map((a) => String(a).trim()).filter(Boolean);
  }

  let env: Record<string, string> | undefined;
  if (obj.env && typeof obj.env === 'object' && !Array.isArray(obj.env)) {
    const rawEnv = obj.env as Record<string, unknown>;
    const parsedEnv: Record<string, string> = {};
    for (const [k, v] of Object.entries(rawEnv)) {
      if (k && v !== undefined && v !== null) {
        parsedEnv[k] = String(v);
      }
    }
    if (Object.keys(parsedEnv).length > 0) env = parsedEnv;
  }

  let headers: Record<string, string> | undefined;
  if (obj.headers && typeof obj.headers === 'object' && !Array.isArray(obj.headers)) {
    const rawHeaders = obj.headers as Record<string, unknown>;
    const parsedHeaders: Record<string, string> = {};
    for (const [k, v] of Object.entries(rawHeaders)) {
      if (k && v !== undefined && v !== null) {
        parsedHeaders[k] = String(v);
      }
    }
    if (Object.keys(parsedHeaders).length > 0) headers = parsedHeaders;
  }

  const allow_private_network = typeof obj.allow_private_network === 'boolean'
    ? obj.allow_private_network
    : typeof obj.allowPrivateNetwork === 'boolean'
      ? obj.allowPrivateNetwork
      : undefined;

  return {
    name,
    transport,
    command: transport === 'stdio' ? command : undefined,
    args: transport === 'stdio' ? args : undefined,
    url: transport === 'http' ? url : undefined,
    env,
    headers,
    allow_private_network,
  };
}

/**
 * Universal auto-detector: parses raw user input as either JSON config or CLI command.
 */
export function parseMCPInput(raw: string): ParsedMCPServerConfig[] {
  const trimmed = raw.trim();
  if (!trimmed) return [];

  // Try JSON first if it starts with { or [
  if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
    const jsonServers = parseJSONConfig(trimmed);
    if (jsonServers.length > 0) {
      return jsonServers;
    }
  }

  // Try CLI parsing
  const cliServer = parseCLICommand(trimmed);
  if (cliServer) {
    return [cliServer];
  }

  // Try JSON again even if it didn't start with { or [ (e.g. leading comment or whitespace)
  const jsonServersFallback = parseJSONConfig(trimmed);
  if (jsonServersFallback.length > 0) {
    return jsonServersFallback;
  }

  return [];
}

/**
 * Converts a parsed MCP server config to CreateMCPServerRequest
 */
export function parsedConfigToCreateRequest(config: ParsedMCPServerConfig, enabled = false): CreateMCPServerRequest {
  return {
    name: config.name,
    transport: config.transport,
    command: config.command,
    args: config.args,
    url: config.url,
    env: config.env,
    headers: config.headers,
    allow_private_network: config.allow_private_network,
    enabled,
  };
}

/**
 * Format headers/env dictionary into KEY=value string lines
 */
export function formatEnvOrHeaders(record?: Record<string, string>): string {
  if (!record) return '';
  return Object.entries(record)
    .map(([k, v]) => `${k}=${v}`)
    .join('\n');
}
