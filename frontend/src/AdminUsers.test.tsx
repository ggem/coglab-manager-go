import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import AdminUsers from './AdminUsers'
import { renderWithProviders } from './test/renderWithProviders'
import type { AdminUser, CreateUserResult, ResendInviteResult } from './api'

const { listUsers, createUser, resendInvite } = vi.hoisted(() => ({
  listUsers: vi.fn(),
  createUser: vi.fn(),
  resendInvite: vi.fn(),
}))

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  listUsers,
  createUser,
  resendInvite,
}))

const activeUser: AdminUser = {
  id: 1,
  email: 'active@example.edu',
  first_name: 'Active',
  last_name: 'Person',
  is_platform_admin: true,
  has_password: true,
  deactivated: false,
}
const pendingUser: AdminUser = {
  id: 2,
  email: 'pending@example.edu',
  first_name: 'Pending',
  last_name: 'Person',
  is_platform_admin: false,
  has_password: false,
  deactivated: false,
}
const deactivatedUser: AdminUser = {
  id: 3,
  email: 'gone@example.edu',
  first_name: 'Gone',
  last_name: 'Person',
  is_platform_admin: false,
  has_password: false,
  deactivated: true,
}

afterEach(() => {
  vi.clearAllMocks()
})

function rowFor(name: string) {
  return screen.getByText(name).closest('tr')!
}

describe('AdminUsers roster', () => {
  it('renders each account with its status and a Resend invite button only for pending accounts', async () => {
    listUsers.mockResolvedValue([activeUser, pendingUser, deactivatedUser])

    renderWithProviders(<AdminUsers />)

    await screen.findByText('active@example.edu')
    expect(within(rowFor('Active Person')).getByText('Active')).toBeInTheDocument()
    expect(within(rowFor('Active Person')).getByText('Yes')).toBeInTheDocument()
    expect(within(rowFor('Active Person')).queryByRole('button', { name: 'Resend invite' })).not.toBeInTheDocument()

    expect(within(rowFor('Pending Person')).getByText('Invite pending')).toBeInTheDocument()
    expect(within(rowFor('Pending Person')).getByRole('button', { name: 'Resend invite' })).toBeInTheDocument()

    expect(within(rowFor('Gone Person')).getByText('Deactivated')).toBeInTheDocument()
    expect(within(rowFor('Gone Person')).queryByRole('button', { name: 'Resend invite' })).not.toBeInTheDocument()
  })
})

describe('AdminUsers create form', () => {
  it('resets the form and shows no notice when the invite email sends', async () => {
    listUsers.mockResolvedValue([])
    createUser.mockResolvedValue({
      id: 4,
      email: 'new-hire@example.edu',
      first_name: 'New',
      last_name: 'Hire',
      is_platform_admin: false,
      has_password: false,
      deactivated: false,
      invite_email_sent: true,
    } satisfies CreateUserResult)
    const typeUser = userEvent.setup()

    renderWithProviders(<AdminUsers />)
    await screen.findByText('No users yet.')
    await typeUser.type(screen.getByLabelText('Email'), 'new-hire@example.edu')
    await typeUser.type(screen.getByLabelText('First name'), 'New')
    await typeUser.type(screen.getByLabelText('Last name'), 'Hire')
    await typeUser.click(screen.getByRole('button', { name: 'Create user' }))

    await waitFor(() => expect(createUser).toHaveBeenCalled())
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Email')).toHaveValue('')
  })

  it('shows an actionable notice when the invite email fails to send', async () => {
    listUsers.mockResolvedValue([])
    createUser.mockResolvedValue({
      id: 5,
      email: 'no-email@example.edu',
      first_name: 'No',
      last_name: 'Email',
      is_platform_admin: false,
      has_password: false,
      deactivated: false,
      invite_email_sent: false,
    } satisfies CreateUserResult)
    const typeUser = userEvent.setup()

    renderWithProviders(<AdminUsers />)
    await screen.findByText('No users yet.')
    await typeUser.type(screen.getByLabelText('Email'), 'no-email@example.edu')
    await typeUser.type(screen.getByLabelText('First name'), 'No')
    await typeUser.type(screen.getByLabelText('Last name'), 'Email')
    await typeUser.click(screen.getByRole('button', { name: 'Create user' }))

    expect(await screen.findByRole('status')).toHaveTextContent('no-email@example.edu was created, but the invite email failed to send')
  })
})

describe('AdminUsers resend invite', () => {
  it('clears the notice when a resend succeeds', async () => {
    listUsers.mockResolvedValue([pendingUser])
    resendInvite.mockResolvedValue({ invite_email_sent: true } satisfies ResendInviteResult)
    const typeUser = userEvent.setup()

    renderWithProviders(<AdminUsers />)
    await typeUser.click(await screen.findByRole('button', { name: 'Resend invite' }))

    await waitFor(() => expect(resendInvite).toHaveBeenCalledWith(2))
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('shows a notice when a resend fails again', async () => {
    listUsers.mockResolvedValue([pendingUser])
    resendInvite.mockResolvedValue({ invite_email_sent: false } satisfies ResendInviteResult)
    const typeUser = userEvent.setup()

    renderWithProviders(<AdminUsers />)
    await typeUser.click(await screen.findByRole('button', { name: 'Resend invite' }))

    expect(await screen.findByRole('status')).toHaveTextContent('failed again')
  })
})
