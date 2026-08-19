import { describe, it, expect } from 'vitest';
import {
  tokenizeShellCommand,
  parseKeyValuePair,
  parseCLICommand,
  parseJSONConfig,
  parseMCPInput,
  parsedConfigToCreateRequest,
  formatEnvOrHeaders,
  deriveServerName,
} from './mcpImportParser';

describe('mcpImportParser', () => {
  describe('Prompt Requirements Verification', () => {
    it('supports npx add-mcp command with line breaks, name, and Bearer header', () => {
      const input = `npx add-mcp@latest "https://mcp.neon.tech/mcp?projectId=xxxx" \\\n  --name Neon \\\n  --header "Authorization: Bearer napi_xxxxxxxxxxxx"`;
      const servers = parseMCPInput(input);
      expect(servers).toHaveLength(1);
      expect(servers[0]).toEqual({
        name: 'Neon',
        transport: 'http',
        url: 'https://mcp.neon.tech/mcp?projectId=xxxx',
        headers: {
          Authorization: 'Bearer napi_xxxxxxxxxxxx',
        },
      });

      const createReq = parsedConfigToCreateRequest(servers[0], false);
      expect(createReq).toEqual({
        name: 'Neon',
        transport: 'http',
        url: 'https://mcp.neon.tech/mcp?projectId=xxxx',
        headers: {
          Authorization: 'Bearer napi_xxxxxxxxxxxx',
        },
        command: undefined,
        args: undefined,
        env: undefined,
        allow_private_network: undefined,
        enabled: false,
      });
    });

    it('supports JSON configs with mcpServers key', () => {
      const input = `{
  "mcpServers": {
    "Neon": {
      "type": "http",
      "url": "https://mcp.neon.tech/mcp?projectId=xxxxx",
      "headers": {
        "Authorization": "Bearer napi_xxxxxxxxxxx803x5j24bcuk"
      }
    }
  }
}`;
      const servers = parseMCPInput(input);
      expect(servers).toHaveLength(1);
      expect(servers[0]).toEqual({
        name: 'Neon',
        transport: 'http',
        url: 'https://mcp.neon.tech/mcp?projectId=xxxxx',
        headers: {
          Authorization: 'Bearer napi_xxxxxxxxxxx803x5j24bcuk',
        },
      });
    });
  });

  describe('deriveServerName', () => {
    it('derives name from hostname for common domains', () => {
      expect(deriveServerName('https://mcp.neon.tech/mcp?foo=bar')).toBe('Neon');
      expect(deriveServerName('https://api.github.com/v1')).toBe('Github');
    });

    it('derives name from command or package path', () => {
      expect(deriveServerName('@modelcontextprotocol/server-filesystem')).toBe('server-filesystem');
      expect(deriveServerName('C:\\tools\\my-mcp-server.exe')).toBe('my-mcp-server_exe');
    });
  });

  describe('tokenizeShellCommand', () => {
    it('splits basic command with arguments', () => {
      const tokens = tokenizeShellCommand('npx -y @modelcontextprotocol/server-filesystem /path/to/dir');
      expect(tokens).toEqual(['npx', '-y', '@modelcontextprotocol/server-filesystem', '/path/to/dir']);
    });

    it('handles double quotes and line continuations', () => {
      const cmd = `npx add-mcp@latest "https://mcp.neon.tech/mcp?projectId=xxxx" \\\n  --name Neon \\\n  --header "Authorization: Bearer napi_xxxxxxxxxxxx"`;
      const tokens = tokenizeShellCommand(cmd);
      expect(tokens).toEqual([
        'npx',
        'add-mcp@latest',
        'https://mcp.neon.tech/mcp?projectId=xxxx',
        '--name',
        'Neon',
        '--header',
        'Authorization: Bearer napi_xxxxxxxxxxxx',
      ]);
    });

    it('handles single quotes with special characters', () => {
      const cmd = `npx add-mcp@latest 'https://api.example.com/mcp' --name 'My Server' -H 'X-Key: 123'`;
      const tokens = tokenizeShellCommand(cmd);
      expect(tokens).toEqual([
        'npx',
        'add-mcp@latest',
        'https://api.example.com/mcp',
        '--name',
        'My Server',
        '-H',
        'X-Key: 123',
      ]);
    });
  });

  describe('parseKeyValuePair', () => {
    it('parses header style "Authorization: Bearer xyz"', () => {
      const pair = parseKeyValuePair('Authorization: Bearer napi_xxxxxxxxxxxx');
      expect(pair).toEqual(['Authorization', 'Bearer napi_xxxxxxxxxxxx']);
    });

    it('parses env style "KEY=value"', () => {
      const pair = parseKeyValuePair('GITHUB_TOKEN=ghp_123456');
      expect(pair).toEqual(['GITHUB_TOKEN', 'ghp_123456']);
    });

    it('returns null on invalid pair', () => {
      expect(parseKeyValuePair('')).toBeNull();
      expect(parseKeyValuePair('invalid_string')).toBeNull();
    });
  });

  describe('parseCLICommand', () => {
    it('parses npx add-mcp command with header and name', () => {
      const cmd = `npx add-mcp@latest "https://mcp.neon.tech/mcp?projectId=xxxx" \\\n  --name Neon \\\n  --header "Authorization: Bearer napi_xxxxxxxxxxxx"`;
      const result = parseCLICommand(cmd);
      expect(result).not.toBeNull();
      expect(result).toEqual({
        name: 'Neon',
        transport: 'http',
        url: 'https://mcp.neon.tech/mcp?projectId=xxxx',
        headers: {
          Authorization: 'Bearer napi_xxxxxxxxxxxx',
        },
      });
    });

    it('parses npx add-mcp command with multiple headers and short flags', () => {
      const cmd = `npx add-mcp "https://mcp.example.com" -n Example -H "Authorization: Bearer tok" -H "X-Custom: val"`;
      const result = parseCLICommand(cmd);
      expect(result).toEqual({
        name: 'Example',
        transport: 'http',
        url: 'https://mcp.example.com',
        headers: {
          Authorization: 'Bearer tok',
          'X-Custom': 'val',
        },
      });
    });

    it('parses raw stdio command like npx filesystem server', () => {
      const cmd = `npx -y @modelcontextprotocol/server-filesystem /Users/test/Documents`;
      const result = parseCLICommand(cmd);
      expect(result).toEqual({
        name: 'server-filesystem',
        transport: 'stdio',
        command: 'npx',
        args: ['-y', '@modelcontextprotocol/server-filesystem', '/Users/test/Documents'],
      });
    });

    it('parses stdio command with env flags', () => {
      const cmd = `uvx mcp-server-git --repository /repo -e GITHUB_TOKEN=abc`;
      const result = parseCLICommand(cmd);
      expect(result).toEqual({
        name: 'mcp-server-git',
        transport: 'stdio',
        command: 'uvx',
        args: ['mcp-server-git', '--repository', '/repo'],
        env: {
          GITHUB_TOKEN: 'abc',
        },
      });
    });

    it('parses claude mcp add command', () => {
      const cmd = `claude mcp add --transport sse Neon https://mcp.neon.tech/mcp --header "Authorization: Bearer token123"`;
      const result = parseCLICommand(cmd);
      expect(result).toEqual({
        name: 'Neon',
        transport: 'http',
        url: 'https://mcp.neon.tech/mcp',
        headers: {
          Authorization: 'Bearer token123',
        },
      });
    });
  });

  describe('parseJSONConfig', () => {
    it('parses standard mcpServers object config', () => {
      const json = JSON.stringify({
        mcpServers: {
          Neon: {
            type: 'http',
            url: 'https://mcp.neon.tech/mcp?projectId=xxxxx',
            headers: {
              Authorization: 'Bearer napi_xxxxxxxxxxx803x5j24bcuk',
            },
          },
        },
      });

      const result = parseJSONConfig(json);
      expect(result).toHaveLength(1);
      expect(result[0]).toEqual({
        name: 'Neon',
        transport: 'http',
        url: 'https://mcp.neon.tech/mcp?projectId=xxxxx',
        headers: {
          Authorization: 'Bearer napi_xxxxxxxxxxx803x5j24bcuk',
        },
      });
    });

    it('parses multiple servers from mcpServers block', () => {
      const json = JSON.stringify({
        mcpServers: {
          Neon: {
            url: 'https://mcp.neon.tech/mcp',
            headers: { Authorization: 'Bearer xxx' },
          },
          filesystem: {
            command: 'npx',
            args: ['-y', '@modelcontextprotocol/server-filesystem', '/tmp'],
            env: { DEBUG: '1' },
          },
        },
      });

      const result = parseJSONConfig(json);
      expect(result).toHaveLength(2);
      expect(result[0].name).toBe('Neon');
      expect(result[0].transport).toBe('http');
      expect(result[1].name).toBe('filesystem');
      expect(result[1].transport).toBe('stdio');
      expect(result[1].command).toBe('npx');
      expect(result[1].args).toEqual(['-y', '@modelcontextprotocol/server-filesystem', '/tmp']);
      expect(result[1].env).toEqual({ DEBUG: '1' });
    });

    it('parses single server JSON object directly', () => {
      const json = JSON.stringify({
        name: 'Custom',
        type: 'stdio',
        command: 'python',
        args: ['-m', 'server'],
      });

      const result = parseJSONConfig(json);
      expect(result).toHaveLength(1);
      expect(result[0]).toEqual({
        name: 'Custom',
        transport: 'stdio',
        command: 'python',
        args: ['-m', 'server'],
      });
    });
  });

  describe('parseMCPInput', () => {
    it('detects user CLI example from prompt', () => {
      const input = `npx add-mcp@latest "https://mcp.neon.tech/mcp?projectId=xxxx" \\\n  --name Neon \\\n  --header "Authorization: Bearer napi_xxxxxxxxxxxx"`;
      const servers = parseMCPInput(input);
      expect(servers).toHaveLength(1);
      expect(servers[0].name).toBe('Neon');
      expect(servers[0].transport).toBe('http');
      expect(servers[0].url).toBe('https://mcp.neon.tech/mcp?projectId=xxxx');
      expect(servers[0].headers).toEqual({ Authorization: 'Bearer napi_xxxxxxxxxxxx' });
    });

    it('detects user JSON example from prompt', () => {
      const input = `{
  "mcpServers": {
    "Neon": {
      "type": "http",
      "url": "https://mcp.neon.tech/mcp?projectId=xxxxx",
      "headers": {
        "Authorization": "Bearer napi_xxxxxxxxxxx803x5j24bcuk"
      }
    }
  }
}`;
      const servers = parseMCPInput(input);
      expect(servers).toHaveLength(1);
      expect(servers[0].name).toBe('Neon');
      expect(servers[0].transport).toBe('http');
      expect(servers[0].url).toBe('https://mcp.neon.tech/mcp?projectId=xxxxx');
      expect(servers[0].headers).toEqual({ Authorization: 'Bearer napi_xxxxxxxxxxx803x5j24bcuk' });
    });
  });

  describe('formatEnvOrHeaders', () => {
    it('formats map as lines', () => {
      expect(formatEnvOrHeaders({ A: '1', B: '2' })).toBe('A=1\nB=2');
      expect(formatEnvOrHeaders(undefined)).toBe('');
    });
  });
});
