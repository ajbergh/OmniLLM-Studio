import { afterEach, describe, expect, it, vi } from 'vitest';
import { authApi, getAuthToken, setAuthToken } from './api';

afterEach(() => {
  setAuthToken(null);
  vi.unstubAllGlobals();
});

describe('cookie session migration', () => {
  it('does not retain browser bearer tokens or re-persist legacy tokens', () => {
    const removeItem = vi.fn();
    const setItem = vi.fn();
    vi.stubGlobal('localStorage', { removeItem, setItem });
    setAuthToken('browser-readable-token');
    expect(getAuthToken()).toBeNull();
    expect(removeItem).toHaveBeenCalledWith('omnillm_auth_token');
    expect(setItem).not.toHaveBeenCalled();
  });

  it('uses browser cookies for API requests without bearer headers', async () => {
    const fetchStub = vi.fn(async () => new Response(
      JSON.stringify({ auth_enabled: true, has_users: true }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    ));
    vi.stubGlobal('fetch', fetchStub);
    setAuthToken('not-retained');
    await expect(authApi.status()).resolves.toEqual({ auth_enabled: true, has_users: true });
    expect(fetchStub).toHaveBeenCalledWith('/v1/auth/status', expect.objectContaining({
      credentials: 'include',
      headers: expect.not.objectContaining({ Authorization: expect.any(String) }),
    }));
  });
});
