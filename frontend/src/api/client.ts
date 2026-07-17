const BASE = ''

async function handleResponse<T>(res: Response): Promise<T> {
  if (res.status === 401) {
    window.location.href = `/auth/login?return_to=${encodeURIComponent(window.location.pathname)}`
    throw new Error('unauthorized')
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw Object.assign(new Error(body?.error?.message ?? res.statusText), { status: res.status, body })
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      'X-Requested-With': 'XMLHttpRequest',
      ...options?.headers,
    },
  })
  return handleResponse<T>(res)
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined }),
  patch: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PATCH', body: body ? JSON.stringify(body) : undefined }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
  delete_body: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'DELETE', body: JSON.stringify(body) }),
  // No Content-Type header here — fetch sets the multipart boundary itself
  // when given a FormData body, and an explicit 'application/json' would break parsing.
  postForm: <T>(path: string, formData: FormData) =>
    fetch(BASE + path, {
      method: 'POST',
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
      body: formData,
    }).then(res => handleResponse<T>(res)),
}

// request() sets Error.message from the server's error body already, so
// callers never need to reach into err.body.error.message themselves.
export function getErrorMessage(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback
}
