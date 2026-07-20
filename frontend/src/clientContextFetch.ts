let installed = false;

function browserTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || '';
  } catch {
    return '';
  }
}

function isOmniApiUrl(url: URL): boolean {
  return url.pathname === '/v1'
    || url.pathname.startsWith('/v1/')
    || url.pathname.includes('/__desktop/');
}

function addClientContext(rawUrl: string): string {
  try {
    const url = new URL(rawUrl, window.location.origin);
    if (!isOmniApiUrl(url)) return rawUrl;

    const timezone = browserTimezone();
    const locale = typeof navigator !== 'undefined' ? navigator.language : '';
    if (timezone) url.searchParams.set('omnillm_timezone', timezone);
    if (locale) url.searchParams.set('omnillm_locale', locale);
    return url.toString();
  } catch {
    return rawUrl;
  }
}

export function installClientContextFetch(): void {
  if (installed || typeof window === 'undefined' || typeof window.fetch !== 'function') return;
  installed = true;

  const originalFetch = window.fetch.bind(window);
  window.fetch = ((input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    if (input instanceof Request) {
      const contextualRequest = new Request(addClientContext(input.url), input);
      return originalFetch(contextualRequest, init);
    }
    const rawUrl = input instanceof URL ? input.toString() : input;
    return originalFetch(addClientContext(rawUrl), init);
  }) as typeof window.fetch;
}
