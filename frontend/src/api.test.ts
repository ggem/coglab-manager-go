import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, NetworkError, errorMessage, getMe, logout } from './api'

// apiFetch itself isn't exported (deliberately -- callers go through the
// typed wrapper functions), so its shared behavior is exercised here
// through its two thinnest real callers: getMe() (a plain GET expecting a
// JSON body) and logout() (a POST expecting 204/no body).

afterEach(() => {
  vi.unstubAllGlobals()
})

function stubFetch(response: Response) {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response))
}

describe('apiFetch (via getMe/logout)', () => {
  it('decodes a 200 JSON body', async () => {
    stubFetch(new Response(JSON.stringify({ user: { id: 1, email: 'a@example.edu' } }), { status: 200 }))

    const result = await getMe()

    expect(result.user.id).toBe(1)
  })

  it('resolves to undefined on 204', async () => {
    stubFetch(new Response(null, { status: 204 }))

    await expect(logout()).resolves.toBeUndefined()
  })

  it('throws ApiError with the server message for a non-OK JSON error body', async () => {
    stubFetch(new Response(JSON.stringify({ error: 'invalid credentials' }), { status: 401 }))

    await expect(getMe()).rejects.toMatchObject({ status: 401, message: 'invalid credentials' })
  })

  it('throws ApiError with the raw text for a non-OK, non-JSON body', async () => {
    stubFetch(new Response('upstream timeout', { status: 502 }))

    await expect(getMe()).rejects.toMatchObject({ status: 502, message: 'upstream timeout' })
  })

  it('falls back to a generic message for a non-OK, empty body', async () => {
    stubFetch(new Response('', { status: 500 }))

    await expect(getMe()).rejects.toMatchObject({ status: 500, message: 'request failed: 500' })
  })

  it('wraps a rejected fetch (network failure) in NetworkError', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))

    await expect(getMe()).rejects.toBeInstanceOf(NetworkError)
  })
})

describe('errorMessage', () => {
  it("returns the server's message for an ApiError", () => {
    expect(errorMessage(new ApiError(400, 'bad request'), 'fallback')).toBe('bad request')
  })

  it('returns a fixed message for a NetworkError, not its raw text', () => {
    expect(errorMessage(new NetworkError('Failed to fetch'), 'fallback')).toBe(
      "Can't reach the server -- check your connection and try again.",
    )
  })

  it('returns the fallback for anything else', () => {
    expect(errorMessage(new Error('some bug'), 'fallback')).toBe('fallback')
    expect(errorMessage('not even an error', 'fallback')).toBe('fallback')
  })
})
