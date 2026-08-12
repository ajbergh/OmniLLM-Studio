import { afterEach, describe, expect, it, vi } from 'vitest';
import { sandboxApi } from './sandboxApi';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('sandbox settings API routing', () => {
  it('keeps sandbox status on the authenticated v1 API surface', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        configured: false,
        capabilities: {
          name: '',
          os_isolation: false,
          filesystem_isolation: false,
          network_isolation: false,
          network_allowlist: false,
          process_tree_isolation: false,
          memory_limit: false,
          cpu_limit: false,
          pid_limit: false,
          disk_limit: false,
        },
        extension_sandbox_mode: 'auto',
        path_grants_configured: false,
        path_grant_available_here: false,
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    await sandboxApi.status();

    expect(fetchMock).toHaveBeenCalledWith(
      '/v1/sandbox/status',
      expect.objectContaining({ credentials: 'include' }),
    );
  });

  it('keeps workspace change review on the v1 API surface and encodes workspace IDs', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => [],
    });
    vi.stubGlobal('fetch', fetchMock);

    await sandboxApi.workspaceChanges('project one/child', 500);

    expect(fetchMock).toHaveBeenCalledWith(
      '/v1/sandbox/workspaces/project%20one%2Fchild/changes?limit=200',
      expect.objectContaining({ credentials: 'include' }),
    );
  });
});
