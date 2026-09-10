import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import SetPassword from './SetPassword'
import { ApiError, type LoginResponse } from './api'

const { setPassword, navigate } = vi.hoisted(() => ({
  setPassword: vi.fn(),
  navigate: vi.fn(),
}))

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  setPassword,
}))

// SetPassword only calls useNavigate to redirect after a successful
// submit -- mocking the hook directly (rather than mounting a real
// MemoryRouter + <Routes>) is enough to verify that call, and the
// component doesn't otherwise depend on any router state.
vi.mock('react-router-dom', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router-dom')>()),
  useNavigate: () => navigate,
}))

const user = {
  id: 42,
  email: 'new-hire@example.edu',
  first_name: 'Barbara',
  last_name: 'Liskov',
  is_platform_admin: false,
}

afterEach(() => {
  vi.clearAllMocks()
  window.history.pushState({}, '', '/')
})

describe('SetPassword', () => {
  it('shows a missing-token message and never calls setPassword when the URL has no token', () => {
    render(<SetPassword onLogin={vi.fn()} />)

    expect(screen.getByRole('alert')).toHaveTextContent('missing its token')
    expect(setPassword).not.toHaveBeenCalled()
  })

  it('shows a client-side error and never calls setPassword when the passwords do not match', async () => {
    window.history.pushState({}, '', '/set-password?token=abc123')
    const typeUser = userEvent.setup()

    render(<SetPassword onLogin={vi.fn()} />)
    await typeUser.type(screen.getByLabelText('Password'), 'first-password')
    await typeUser.type(screen.getByLabelText('Confirm password'), 'second-password')
    await typeUser.click(screen.getByRole('button', { name: 'Set password' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Passwords do not match.')
    expect(setPassword).not.toHaveBeenCalled()
  })

  it('logs in and navigates to / on a successful submit', async () => {
    window.history.pushState({}, '', '/set-password?token=abc123')
    setPassword.mockResolvedValue({ user } satisfies LoginResponse)
    const onLogin = vi.fn()
    const typeUser = userEvent.setup()

    render(<SetPassword onLogin={onLogin} />)
    await typeUser.type(screen.getByLabelText('Password'), 'a-long-password')
    await typeUser.type(screen.getByLabelText('Confirm password'), 'a-long-password')
    await typeUser.click(screen.getByRole('button', { name: 'Set password' }))

    expect(setPassword).toHaveBeenCalledWith('abc123', 'a-long-password')
    await vi.waitFor(() => expect(onLogin).toHaveBeenCalledWith(user))
    expect(navigate).toHaveBeenCalledWith('/', { replace: true })
  })

  it('shows the server error and does not navigate when setPassword rejects', async () => {
    window.history.pushState({}, '', '/set-password?token=expired-token')
    setPassword.mockRejectedValue(new ApiError(400, 'this link is invalid or has expired'))
    const typeUser = userEvent.setup()

    render(<SetPassword onLogin={vi.fn()} />)
    await typeUser.type(screen.getByLabelText('Password'), 'a-long-password')
    await typeUser.type(screen.getByLabelText('Confirm password'), 'a-long-password')
    await typeUser.click(screen.getByRole('button', { name: 'Set password' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('this link is invalid or has expired')
    expect(navigate).not.toHaveBeenCalled()
  })
})
