import type { ApiResponse } from './types';

export async function clearToken() {
  await fetch('/api/v1/auth/logout', { method: 'POST', cache: 'no-store' }).catch(() => undefined);
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path.startsWith('/api') ? path : `/api/v1${path}`, {
    ...init,
    headers: {
      accept: 'application/json',
      ...(init?.body ? { 'content-type': 'application/json' } : {}),
      ...(init?.headers || {})
    },
    cache: 'no-store'
  });
  const json = (await res.json().catch(() => ({}))) as ApiResponse<T>;
  if (res.status === 401 && typeof window !== 'undefined' && window.location.pathname !== '/login') {
    window.location.href = '/login';
  }
  if (!res.ok || json.code !== 0) {
    throw new Error(json.message || `Request failed (${res.status})`);
  }
  return json.data as T;
}

export function postJSON<T>(path: string, body?: unknown) {
  return apiFetch<T>(path, { method: 'POST', body: body == null ? undefined : JSON.stringify(body) });
}

export function patchJSON<T>(path: string, body: unknown) {
  return apiFetch<T>(path, { method: 'PATCH', body: JSON.stringify(body) });
}

export function putJSON<T>(path: string, body: unknown) {
  return apiFetch<T>(path, { method: 'PUT', body: JSON.stringify(body) });
}

export function deleteJSON<T>(path: string) {
  return apiFetch<T>(path, { method: 'DELETE' });
}
