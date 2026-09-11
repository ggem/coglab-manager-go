import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'
import { ApiError, type LoginResponse } from './api'

const { getMe, navigate } = vi.hoisted(() => ({
  getMe: vi.fn(),
  navigate: vi.fn(),
}))

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  getMe,
}))

// Same rationale as SetPassword.test.tsx: these tests never reach
// App's own <Routes> (the /set-password branch returns before it), so
// mocking useNavigate directly is enough for SetPassword's own
// post-submit redirect, without needing a real Router.
vi.mock('react-router-dom', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router-dom')>()),
  useNavigate: () => navigate,
}))

function renderApp() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>,
  )
}

afterEach(() => {
  vi.clearAllMocks()
  window.history.pushState({}, '', '/')
})

describe('App /set-password routing', () => {
  // Regression coverage for a real bug: App checked window.location for
  // /set-password only inside its "not logged in" branch, so a browser
  // that already had a session (for another account, or a stale one
  // from an earlier attempt) skipped straight to the authenticated
  // <Routes> tree -- which has no /set-password route at all -- and
  // silently rendered nothing. An invite link must work regardless of
  // whatever session state happens to already be in the browser.
  it('renders SetPassword even when the browser already has a logged-in session', async () => {
    getMe.mockResolvedValue({
      user: { id: 1, email: 'admin@example.edu', first_name: 'Admin', last_name: 'Person', is_platform_admin: true },
    } satisfies LoginResponse)
    window.history.pushState({}, '', '/set-password?token=abc123')

    renderApp()

    expect(await screen.findByRole('heading', { name: 'Set your password' })).toBeInTheDocument()
  })

  it('renders SetPassword when logged out', async () => {
    getMe.mockRejectedValue(new ApiError(401, 'not logged in'))
    window.history.pushState({}, '', '/set-password?token=abc123')

    renderApp()

    expect(await screen.findByRole('heading', { name: 'Set your password' })).toBeInTheDocument()
  })
})
