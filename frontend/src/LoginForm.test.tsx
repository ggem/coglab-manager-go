import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import LoginForm from './LoginForm'
import { ApiError, type LoginResponse } from './api'

const { login, getSSOConfig } = vi.hoisted(() => ({
  login: vi.fn(),
  getSSOConfig: vi.fn(),
}))

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  login,
  getSSOConfig,
}))

const user = {
  id: 1,
  email: 'ada@example.edu',
  first_name: 'Ada',
  last_name: 'Lovelace',
  is_platform_admin: false,
}

afterEach(() => {
  vi.clearAllMocks()
  window.history.pushState({}, '', '/')
})

describe('LoginForm', () => {
  it('calls onLogin with the returned user on a successful submit', async () => {
    getSSOConfig.mockResolvedValue({ enabled: false })
    login.mockResolvedValue({ user } satisfies LoginResponse)
    const onLogin = vi.fn()
    const typeUser = userEvent.setup()

    render(<LoginForm onLogin={onLogin} />)
    await typeUser.type(screen.getByLabelText('Email'), 'ada@example.edu')
    await typeUser.type(screen.getByLabelText('Password'), 's3cret')
    await typeUser.click(screen.getByRole('button', { name: 'Sign in' }))

    await waitFor(() => expect(onLogin).toHaveBeenCalledWith(user))
    expect(login).toHaveBeenCalledWith('ada@example.edu', 's3cret')
  })

  it('shows an error and does not call onLogin when login rejects', async () => {
    getSSOConfig.mockResolvedValue({ enabled: false })
    login.mockRejectedValue(new ApiError(401, 'invalid credentials'))
    const onLogin = vi.fn()
    const typeUser = userEvent.setup()

    render(<LoginForm onLogin={onLogin} />)
    await typeUser.type(screen.getByLabelText('Email'), 'ada@example.edu')
    await typeUser.type(screen.getByLabelText('Password'), 'wrong')
    await typeUser.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('invalid credentials')
    expect(onLogin).not.toHaveBeenCalled()
  })

  it('shows the SSO link only when the backend reports SSO enabled', async () => {
    getSSOConfig.mockResolvedValue({ enabled: true })

    render(<LoginForm onLogin={vi.fn()} />)

    expect(await screen.findByRole('link', { name: 'Sign in with SSO' })).toBeInTheDocument()
  })

  it('does not show the SSO link when SSO is disabled', async () => {
    getSSOConfig.mockResolvedValue({ enabled: false })

    render(<LoginForm onLogin={vi.fn()} />)

    await waitFor(() => expect(getSSOConfig).toHaveBeenCalled())
    expect(screen.queryByRole('link', { name: 'Sign in with SSO' })).not.toBeInTheDocument()
  })

  it('shows a login-failed message on mount when the URL has ?sso_error=1', () => {
    window.history.pushState({}, '', '/?sso_error=1')
    getSSOConfig.mockResolvedValue({ enabled: false })

    render(<LoginForm onLogin={vi.fn()} />)

    expect(screen.getByRole('alert')).toHaveTextContent('login failed')
  })
})
