const API_PREFIX = '/api/backend';
let accessToken: string | null = null;
let refreshPromise: Promise<string | null> | null = null;
export function setAccessToken(token: string | null) { accessToken = token; }
export function getAccessToken() { return accessToken; }

async function send(path: string, init: RequestInit): Promise<Response> {
  const headers = new Headers(init.headers);
  if (accessToken) headers.set('Authorization', `Bearer ${accessToken}`);
  if (init.body && !(init.body instanceof FormData)) headers.set('Content-Type', 'application/json');
  return fetch(`${API_PREFIX}${path}`, { ...init, headers, credentials: 'include' });
}

export function refreshAccessToken() {
  if (!refreshPromise) {
    refreshPromise = send('/auth/refresh', { method: 'POST' })
      .then(async (response) => {
        if (!response.ok) throw new Error('Không làm mới được phiên đăng nhập.');
        const tokens = await response.json() as { access_token?: string };
        setAccessToken(tokens.access_token ?? null);
        return tokens.access_token ?? null;
      })
      .catch((error: unknown) => {
        setAccessToken(null);
        throw error;
      })
      .finally(() => { refreshPromise = null; });
  }
  return refreshPromise;
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  let response = await send(path, init);
  if (response.status === 401 && path !== '/auth/refresh') {
    const token = await refreshAccessToken().catch(() => null);
    if (token) response = await send(path, init);
  }
  if (!response.ok) {
    const raw = await response.text();
    let message = raw || `Request failed (${response.status})`;
    try {
      const body = JSON.parse(raw) as { error?: string };
      if (body.error) message = body.error;
    } catch { /* Keep the plain-text response. */ }
    throw new Error(message);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}
