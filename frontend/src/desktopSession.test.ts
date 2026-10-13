import { afterEach, describe, expect, it, vi } from 'vitest';

const desktopBase = 'http://127.0.0.1:54321/__desktop/fixture-launch-secret/v1';

async function desktopAPI() {
  vi.resetModules();
  vi.stubGlobal('window', {
    go: {
      main: { App: { GetAPIBase: vi.fn(async () => desktopBase) } },
    },
  });
  const client = await import('./api');
  await client.initAPIBase();
  return client;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.resetModules();
});

describe('Wails desktop session isolation', () => {
  it('keeps desktop bearer tokens volatile, includes them for central and streaming requests, and invalidates on logout', async () => {
    const setItem = vi.fn();
    const removeItem = vi.fn();
    vi.stubGlobal('localStorage', { setItem, removeItem });

    const fetchStub = vi.fn(async () => new Response(
      JSON.stringify({ username: 'matrix_user', id: 'test' }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    ));
    vi.stubGlobal('fetch', fetchStub);

    const client = await desktopAPI();
    client.setAuthToken('volatile-desktop-bearer');
    expect(client.getAuthToken()).toBe('volatile-desktop-bearer');
    expect(setItem).not.toHaveBeenCalled();
    expect(removeItem).toHaveBeenCalledWith('omnillm_auth_token');

    await client.authApi.me();
    expect(fetchStub).toHaveBeenLastCalledWith(
      `${desktopBase}/users/me`,
      expect.objectContaining({
        credentials: 'include',
        headers: expect.objectContaining({ Authorization: 'Bearer volatile-desktop-bearer' }),
      }),
    );

    await client.authApi.logout();
    expect(client.getAuthToken()).toBeNull();
    await client.authApi.me();
    expect(fetchStub).toHaveBeenLastCalledWith(
      `${desktopBase}/users/me`,
      expect.objectContaining({
        headers: expect.not.objectContaining({ Authorization: expect.any(String) }),
      }),
    );
    expect(setItem).not.toHaveBeenCalled();
  });

  it('never resurrects a desktop bearer after module restart or from legacy localStorage', async () => {
    const setItem = vi.fn();
    const removeItem = vi.fn();
    vi.stubGlobal('localStorage', {
      getItem: vi.fn(() => 'stale-browser-readable-token'), removeItem, setItem,
    });
    vi.stubGlobal('fetch', vi.fn(async () => new Response(
      JSON.stringify({ username: 'matrix_user' }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    )));

    const firstLaunch = await desktopAPI();
    firstLaunch.setAuthToken('launch-one-secret');
    expect(firstLaunch.getAuthToken()).toBe('launch-one-secret');

    const nextLaunch = await desktopAPI();
    expect(nextLaunch.getAuthToken()).toBeNull();
    await nextLaunch.authApi.me();
    expect(fetch).toHaveBeenCalledWith(
      `${desktopBase}/users/me`,
      expect.objectContaining({
        headers: expect.not.objectContaining({ Authorization: expect.any(String) }),
      }),
    );
    expect(removeItem).toHaveBeenCalledWith('omnillm_auth_token');
    expect(setItem).not.toHaveBeenCalled();
  });

  it('invalidates desktop bearer locally even when logout request fails', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => {
      throw new Error('offline');
    }));
    const client = await desktopAPI();
    client.setAuthToken('temporary-desktop-token');
    await expect(client.authApi.logout()).rejects.toThrow('offline');
    expect(client.getAuthToken()).toBeNull();
  });
});
