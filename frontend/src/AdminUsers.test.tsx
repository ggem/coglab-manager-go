import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import AdminUsers from './AdminUsers'
import { renderWithProviders } from './test/renderWithProviders'
import type { AdminUser, CreateUserResult, Lab, LoginResponse, ResendInviteResult, Role } from './api'

const { listUsers, createUser, resendInvite, getMe, deactivateUser, setPlatformAdmin, listLabs, listRoles } = vi.hoisted(() => ({
  listUsers: vi.fn(),
  createUser: vi.fn(),
  resendInvite: vi.fn(),
  getMe: vi.fn(),
  deactivateUser: vi.fn(),
  setPlatformAdmin: vi.fn(),
  listLabs: vi.fn(),
  listRoles: vi.fn(),
}))

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  listUsers,
  createUser,
  resendInvite,
  getMe,
  deactivateUser,
  setPlatformAdmin,
  listLabs,
  listRoles,
}))

// A caller distinct from every test fixture user below, so "this is my
// own row" behavior doesn't interfere with tests that aren't exercising
// it -- the "caller's own row" case is exercised explicitly further down.
const caller = { id: 99, email: 'caller@example.edu', first_name: 'Caller', last_name: 'Admin', is_platform_admin: true }

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

const labFixture: Lab = { id: 9, name: 'Cognitive Development Center', short_name: 'CDC' }
const roleFixture: Role = { id: 1, name: 'staff', description: 'Plain lab member' }

beforeEach(() => {
  getMe.mockResolvedValue({ user: caller } satisfies LoginResponse)
  listLabs.mockResolvedValue([labFixture])
  listRoles.mockResolvedValue([roleFixture])
})

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

  it('rejects submitting a lab without a permission role', async () => {
    listUsers.mockResolvedValue([])
    const typeUser = userEvent.setup()

    renderWithProviders(<AdminUsers />)
    await screen.findByText('No users yet.')
    await typeUser.type(screen.getByLabelText('Email'), 'new-hire@example.edu')
    await typeUser.type(screen.getByLabelText('First name'), 'New')
    await typeUser.type(screen.getByLabelText('Last name'), 'Hire')
    await typeUser.selectOptions(screen.getByLabelText('Lab (optional)'), String(labFixture.id))
    await typeUser.click(screen.getByRole('button', { name: 'Create user' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Select both a lab and a permission role')
    expect(createUser).not.toHaveBeenCalled()
  })

  it('rejects submitting a permission role without a lab', async () => {
    listUsers.mockResolvedValue([])
    const typeUser = userEvent.setup()

    renderWithProviders(<AdminUsers />)
    await screen.findByText('No users yet.')
    await typeUser.type(screen.getByLabelText('Email'), 'new-hire@example.edu')
    await typeUser.type(screen.getByLabelText('First name'), 'New')
    await typeUser.type(screen.getByLabelText('Last name'), 'Hire')
    await typeUser.selectOptions(screen.getByLabelText('Permission role (optional)'), String(roleFixture.id))
    await typeUser.click(screen.getByRole('button', { name: 'Create user' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Select both a lab and a permission role')
    expect(createUser).not.toHaveBeenCalled()
  })

  it('creates a user with a lab and role assigned when both are selected', async () => {
    listUsers.mockResolvedValue([])
    createUser.mockResolvedValue({
      id: 6,
      email: 'lab-hire@example.edu',
      first_name: 'Lab',
      last_name: 'Hire',
      is_platform_admin: false,
      has_password: false,
      deactivated: false,
      invite_email_sent: true,
    } satisfies CreateUserResult)
    const typeUser = userEvent.setup()

    renderWithProviders(<AdminUsers />)
    await screen.findByText('No users yet.')
    await typeUser.type(screen.getByLabelText('Email'), 'lab-hire@example.edu')
    await typeUser.type(screen.getByLabelText('First name'), 'Lab')
    await typeUser.type(screen.getByLabelText('Last name'), 'Hire')
    await typeUser.selectOptions(screen.getByLabelText('Lab (optional)'), String(labFixture.id))
    await typeUser.selectOptions(screen.getByLabelText('Permission role (optional)'), String(roleFixture.id))
    await typeUser.click(screen.getByRole('button', { name: 'Create user' }))

    await waitFor(() =>
      expect(createUser).toHaveBeenCalledWith(
        expect.objectContaining({ lab_id: labFixture.id, role_id: roleFixture.id }),
      ),
    )
  })

  // Code-review finding: (labs ?? []) / (roles ?? []) made a failed
  // lookup indistinguishable from "there are genuinely no labs/roles" --
  // an admin could submit believing the lone "No lab" option was the
  // only choice, silently creating a lab-less account. The selects must
  // now disable and say so instead.
  it('disables the lab/role pickers and shows an error when the lookups fail, instead of silently looking empty', async () => {
    listUsers.mockResolvedValue([])
    listLabs.mockRejectedValue(new Error('network error'))
    listRoles.mockRejectedValue(new Error('network error'))

    renderWithProviders(<AdminUsers />)
    await screen.findByText('No users yet.')

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to load the lab/permission-role picker')
    expect(screen.getByLabelText('Lab (optional)')).toBeDisabled()
    expect(screen.getByLabelText('Lab (optional)')).toHaveTextContent('Failed to load labs')
    expect(screen.getByLabelText('Permission role (optional)')).toBeDisabled()
    expect(screen.getByLabelText('Permission role (optional)')).toHaveTextContent('Failed to load roles')
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

describe('AdminUsers deactivate action', () => {
  it('does not offer Deactivate on the caller\'s own row', async () => {
    listUsers.mockResolvedValue([{ ...activeUser, id: caller.id, email: caller.email }])

    renderWithProviders(<AdminUsers />)

    const row = await screen.findByText(caller.email)
    expect(within(row.closest('tr')!).queryByRole('button', { name: 'Deactivate' })).not.toBeInTheDocument()
  })

  it('does not offer Deactivate on an already-deactivated row', async () => {
    listUsers.mockResolvedValue([deactivatedUser])

    renderWithProviders(<AdminUsers />)

    const row = await screen.findByText(deactivatedUser.email)
    expect(within(row.closest('tr')!).queryByRole('button', { name: 'Deactivate' })).not.toBeInTheDocument()
  })

  it('deactivates a user after confirming, and refreshes the roster', async () => {
    listUsers.mockResolvedValue([activeUser])
    deactivateUser.mockResolvedValue(undefined)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const typeUser = userEvent.setup()

    renderWithProviders(<AdminUsers />)
    await typeUser.click(await screen.findByRole('button', { name: 'Deactivate' }))

    expect(window.confirm).toHaveBeenCalled()
    await waitFor(() => expect(deactivateUser).toHaveBeenCalledWith(activeUser.id))
    await waitFor(() => expect(listUsers).toHaveBeenCalledTimes(2))
  })

  it('does not deactivate when the confirmation is declined', async () => {
    listUsers.mockResolvedValue([activeUser])
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    const typeUser = userEvent.setup()

    renderWithProviders(<AdminUsers />)
    await typeUser.click(await screen.findByRole('button', { name: 'Deactivate' }))

    expect(deactivateUser).not.toHaveBeenCalled()
  })

  it('shows a notice when deactivation fails', async () => {
    listUsers.mockResolvedValue([activeUser])
    deactivateUser.mockRejectedValue(new Error('boom'))
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const typeUser = userEvent.setup()

    renderWithProviders(<AdminUsers />)
    await typeUser.click(await screen.findByRole('button', { name: 'Deactivate' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Failed to deactivate user.')
  })
})

describe('AdminUsers platform-admin action', () => {
  it('does not offer a platform-admin action on the caller\'s own row', async () => {
    listUsers.mockResolvedValue([{ ...activeUser, id: caller.id, email: caller.email }])

    renderWithProviders(<AdminUsers />)

    const row = await screen.findByText(caller.email)
    expect(within(row.closest('tr')!).queryByRole('button', { name: /platform admin/ })).not.toBeInTheDocument()
  })

  it('does not offer a platform-admin action on an already-deactivated row', async () => {
    listUsers.mockResolvedValue([deactivatedUser])

    renderWithProviders(<AdminUsers />)

    const row = await screen.findByText(deactivatedUser.email)
    expect(within(row.closest('tr')!).queryByRole('button', { name: /platform admin/ })).not.toBeInTheDocument()
  })

  it('grants platform admin to a non-admin user', async () => {
    listUsers.mockResolvedValue([pendingUser])
    setPlatformAdmin.mockResolvedValue(undefined)
    const typeUser = userEvent.setup()

    renderWithProviders(<AdminUsers />)
    await typeUser.click(await screen.findByRole('button', { name: 'Grant platform admin' }))

    await waitFor(() => expect(setPlatformAdmin).toHaveBeenCalledWith(pendingUser.id, true))
  })

  it('revokes platform admin from an admin user', async () => {
    listUsers.mockResolvedValue([activeUser])
    setPlatformAdmin.mockResolvedValue(undefined)
    const typeUser = userEvent.setup()

    renderWithProviders(<AdminUsers />)
    await typeUser.click(await screen.findByRole('button', { name: 'Revoke platform admin' }))

    await waitFor(() => expect(setPlatformAdmin).toHaveBeenCalledWith(activeUser.id, false))
  })

  it('shows a notice when changing platform-admin status fails', async () => {
    listUsers.mockResolvedValue([pendingUser])
    setPlatformAdmin.mockRejectedValue(new Error('boom'))
    const typeUser = userEvent.setup()

    renderWithProviders(<AdminUsers />)
    await typeUser.click(await screen.findByRole('button', { name: 'Grant platform admin' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Failed to change platform-admin status.')
  })
})
