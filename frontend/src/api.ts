// Thin fetch wrapper + types mirroring the Go handler DTOs
// (internal/httpapi/*_handlers.go). Handwritten rather than generated --
// there's no OpenAPI spec yet, and keeping the shapes visible here matches
// this project's preference for explicit code over codegen magic.

// The server responded, just not successfully -- status carries the HTTP
// status code, message is the server's `error` field when the body was
// JSON, or the raw response text otherwise.
export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// fetch() itself failed and no response was ever received (offline, DNS
// failure, connection refused). Distinct from ApiError, which means the
// server responded.
export class NetworkError extends Error {}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      ...init,
      headers: { 'Content-Type': 'application/json', ...init?.headers },
    })
  } catch (err) {
    throw new NetworkError(err instanceof Error ? err.message : 'network request failed')
  }

  if (!res.ok) {
    const text = await res.text()
    let message = text || `request failed: ${res.status}`
    try {
      const body: unknown = JSON.parse(text)
      if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') {
        message = body.error
      }
    } catch {
      // not JSON -- fall back to the raw text set above
    }
    throw new ApiError(res.status, message)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

// Turns a caught error into a message worth showing a user: the server's
// own message for ApiError, "can't reach the server" for NetworkError
// (rather than fetch's raw "Failed to fetch"), or fallback for anything
// else (a bug, most likely -- not worth showing verbatim).
export function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof NetworkError) return "Can't reach the server -- check your connection and try again."
  return fallback
}

export interface User {
  id: number
  email: string
  first_name: string
  last_name: string
}

export interface LoginResponse {
  user: User
}

export function login(email: string, password: string): Promise<LoginResponse> {
  return apiFetch<LoginResponse>('/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
}

export function logout(): Promise<void> {
  return apiFetch<void>('/logout', { method: 'POST' })
}

// Restores session state after a page refresh -- 401s (via ApiError) if
// there's no valid session cookie, which the caller treats the same as
// "not logged in."
export function getMe(): Promise<LoginResponse> {
  return apiFetch<LoginResponse>('/me')
}

export interface ChildSearchResult {
  id: number
  family_id: number
  first_name: string
  last_name: string
  sex: string
  birth_date: string | null
  deactivated: boolean
}

export function searchChildren(query: string): Promise<ChildSearchResult[]> {
  const params = new URLSearchParams()
  if (query) params.set('q', query)
  return apiFetch<ChildSearchResult[]>(`/children/search?${params.toString()}`)
}

export interface FamilySearchResult {
  id: number
  address: string
  city: string
  state: string
  zip: string
}

export function searchFamilies(query: string): Promise<FamilySearchResult[]> {
  const params = new URLSearchParams()
  if (query) params.set('q', query)
  return apiFetch<FamilySearchResult[]>(`/families/search?${params.toString()}`)
}
