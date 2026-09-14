import i18n from '../i18n/index'

const BASE = ''

export class InvalidServerResponseError extends Error {
  readonly status: number
  constructor(status: number) {
    super(i18n.t('errors.invalidServerResponse'))
    this.name = 'InvalidServerResponseError'
    this.status = status
  }
}

function isJson(res: Response): boolean {
  return (res.headers.get('content-type') ?? '').toLowerCase().includes('json')
}

async function handleResponse<T>(res: Response): Promise<T> {
  if (res.status === 401) {
    const body = await res.json().catch(() => ({}))
    if (body?.error?.code === 'reauth_required') {
      // Still logged in, just needs a step-up confirmation — let the caller
      // handle it (see useReauth), don't bounce to the full login page.
      throw Object.assign(new Error(body?.error?.message ?? 'reauth required'), { status: 401, body })
    }
    window.location.href = `/auth/login?return_to=${encodeURIComponent(window.location.pathname)}`
    throw Object.assign(new Error('unauthorized'), { status: 401 })
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw Object.assign(new Error(body?.error?.message ?? res.statusText), { status: res.status, body })
  }
  if (res.status === 204) return undefined as T
  // The SPA fallback (and any ingress error page) can answer an unmatched
  // /api path with 200 text/html; parsing that surfaces a raw
  // "Unexpected token '<'" SyntaxError in a red toast.
  if (!isJson(res)) throw new InvalidServerResponseError(res.status)
  try {
    return (await res.json()) as T
  } catch {
    throw new InvalidServerResponseError(res.status)
  }
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

export interface RequestOptions {
  signal?: AbortSignal
}

export const api = {
  // Pass TanStack Query's `signal` through here (queryFn: ({ signal }) =>
  // api.get('/api/x', { signal })) so navigating away or switching filters
  // actually aborts the in-flight request instead of leaving it racing.
  get: <T>(path: string, opts?: RequestOptions) => request<T>(path, opts),
  post: <T>(path: string, body?: unknown, opts?: RequestOptions) =>
    request<T>(path, { ...opts, method: 'POST', body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: unknown, opts?: RequestOptions) =>
    request<T>(path, { ...opts, method: 'PUT', body: body ? JSON.stringify(body) : undefined }),
  patch: <T>(path: string, body?: unknown, opts?: RequestOptions) =>
    request<T>(path, { ...opts, method: 'PATCH', body: body ? JSON.stringify(body) : undefined }),
  delete: <T>(path: string, opts?: RequestOptions) => request<T>(path, { ...opts, method: 'DELETE' }),
  // No Content-Type header here — fetch sets the multipart boundary itself
  // when given a FormData body, and an explicit 'application/json' would break parsing.
  postForm: <T>(path: string, formData: FormData, opts?: RequestOptions) =>
    fetch(BASE + path, {
      ...opts,
      method: 'POST',
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
      body: formData,
    }).then(res => handleResponse<T>(res)),
}

export function isUnauthorizedError(err: unknown): boolean {
  return typeof err === 'object' && err !== null && (err as { status?: unknown }).status === 401
}

// request() sets Error.message from the server's error body already, so
// callers never need to reach into err.body.error.message themselves.
export function getErrorMessage(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback
}
