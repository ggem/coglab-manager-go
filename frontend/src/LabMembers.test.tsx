import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import LabMembers from './LabMembers'
import { renderWithProviders } from './test/renderWithProviders'
import type { CreateLabMembershipForNewUserResult, LabMembership, Role } from './api'

// Scoped to the "create a new user and add them to this lab" flow --
// the newest, least-covered part of this component (see the frontend
// test suite's own follow-up note). The rest of LabMembers (per-row
// edit/remove, trainings via AttachList) is pre-existing surface not
// covered by this pass.

const { listLabMemberships, listRoles, getMe, searchUsersNotInLab, createLabMembershipForNewUser } = vi.hoisted(
  () => ({
    listLabMemberships: vi.fn(),
    listRoles: vi.fn(),
    getMe: vi.fn(),
    searchUsersNotInLab: vi.fn(),
    createLabMembershipForNewUser: vi.fn(),
  }),
)

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  listLabMemberships,
  listRoles,
  getMe,
  searchUsersNotInLab,
  createLabMembershipForNewUser,
}))

const roles: Role[] = [
  { id: 1, name: 'staff', description: '' },
  { id: 2, name: 'coordinator', description: '' },
  { id: 3, name: 'admin', description: '' },
]

const adminMembership: LabMembership = {
  user_id: 1,
  first_name: 'Admin',
  last_name: 'Person',
  email: 'admin@example.edu',
  role_id: 3,
  role_name: 'admin',
  priority: 'lab_director',
}

function mockAsAdmin() {
  getMe.mockResolvedValue({ user: { id: 1, email: 'admin@example.edu', first_name: 'Admin', last_name: 'Person', is_platform_admin: false } })
  listLabMemberships.mockResolvedValue([adminMembership])
  listRoles.mockResolvedValue(roles)
}

afterEach(() => {
  vi.clearAllMocks()
})

// Drives the UI up to the create-user mini-form being visible: search
// for a name, get zero results (the role picker only appears once a
// search has actually run), pick a permission role, then open the form.
async function openCreateUserForm(typeUser: ReturnType<typeof userEvent.setup>) {
  searchUsersNotInLab.mockResolvedValue([])
  await typeUser.type(await screen.findByPlaceholderText('Search by name…'), 'Nonexistent Person')
  await typeUser.click(screen.getByRole('button', { name: 'Search' }))
  await typeUser.selectOptions(await screen.findByLabelText('Permission role'), 'staff')
  await typeUser.click(await screen.findByRole('button', { name: 'Create new user' }))
}

describe('LabMembers create-user-and-add flow', () => {
  it('is not offered to a non-admin', async () => {
    getMe.mockResolvedValue({ user: { id: 99, email: 'staff@example.edu', first_name: 'S', last_name: 'T', is_platform_admin: false } })
    listLabMemberships.mockResolvedValue([adminMembership])
    listRoles.mockResolvedValue(roles)

    renderWithProviders(<LabMembers labId={9} />)

    await screen.findByText("Only this lab's admins can add, edit, or remove members, or change their trained roles.")
    expect(screen.queryByPlaceholderText('Search by name…')).not.toBeInTheDocument()
  })

  it('shows "Create new user" only after a search with zero results', async () => {
    mockAsAdmin()
    searchUsersNotInLab.mockResolvedValue([])
    const typeUser = userEvent.setup()

    renderWithProviders(<LabMembers labId={9} />)
    expect(screen.queryByRole('button', { name: 'Create new user' })).not.toBeInTheDocument()

    await typeUser.type(await screen.findByPlaceholderText('Search by name…'), 'Nonexistent Person')
    await typeUser.click(screen.getByRole('button', { name: 'Search' }))

    expect(await screen.findByText('No matches.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create new user' })).toBeInTheDocument()
  })

  it('only shows the permission-role picker once a search has run, not alongside the search bar', async () => {
    mockAsAdmin()
    searchUsersNotInLab.mockResolvedValue([])
    const typeUser = userEvent.setup()

    renderWithProviders(<LabMembers labId={9} />)
    await screen.findByPlaceholderText('Search by name…')
    expect(screen.queryByLabelText('Permission role')).not.toBeInTheDocument()

    await typeUser.type(screen.getByPlaceholderText('Search by name…'), 'Nonexistent Person')
    await typeUser.click(screen.getByRole('button', { name: 'Search' }))

    expect(await screen.findByLabelText('Permission role')).toBeInTheDocument()
  })

  it('creates and adds a new user, closing the form and clearing search state on success', async () => {
    mockAsAdmin()
    createLabMembershipForNewUser.mockResolvedValue({
      id: 5,
      first_name: 'New',
      last_name: 'Hire',
      email: 'new-hire@example.edu',
      invite_email_sent: true,
    } satisfies CreateLabMembershipForNewUserResult)
    const typeUser = userEvent.setup()

    renderWithProviders(<LabMembers labId={9} />)
    await openCreateUserForm(typeUser)
    await typeUser.type(screen.getByLabelText('Email'), 'new-hire@example.edu')
    await typeUser.type(screen.getByLabelText('First name'), 'New')
    await typeUser.type(screen.getByLabelText('Last name'), 'Hire')
    await typeUser.click(screen.getByRole('button', { name: 'Create and add' }))

    await waitFor(() =>
      expect(createLabMembershipForNewUser).toHaveBeenCalledWith(9, {
        email: 'new-hire@example.edu',
        first_name: 'New',
        last_name: 'Hire',
        role_id: 1,
      }),
    )
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Create and add' })).not.toBeInTheDocument())
    expect(screen.queryByText('No matches.')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows an actionable error when the invite email fails to send', async () => {
    mockAsAdmin()
    createLabMembershipForNewUser.mockResolvedValue({
      id: 6,
      first_name: 'No',
      last_name: 'Email',
      email: 'no-email@example.edu',
      invite_email_sent: false,
    } satisfies CreateLabMembershipForNewUserResult)
    const typeUser = userEvent.setup()

    renderWithProviders(<LabMembers labId={9} />)
    await openCreateUserForm(typeUser)
    await typeUser.type(screen.getByLabelText('Email'), 'no-email@example.edu')
    await typeUser.type(screen.getByLabelText('First name'), 'No')
    await typeUser.type(screen.getByLabelText('Last name'), 'Email')
    await typeUser.click(screen.getByRole('button', { name: 'Create and add' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'no-email@example.edu was added, but the invite email failed to send',
    )
  })

  it('shows a generic error and keeps the form open when creation is rejected', async () => {
    mockAsAdmin()
    createLabMembershipForNewUser.mockRejectedValue(new Error('boom'))
    const typeUser = userEvent.setup()

    renderWithProviders(<LabMembers labId={9} />)
    await openCreateUserForm(typeUser)
    await typeUser.type(screen.getByLabelText('Email'), 'dup@example.edu')
    await typeUser.type(screen.getByLabelText('First name'), 'Dup')
    await typeUser.type(screen.getByLabelText('Last name'), 'Licate')
    await typeUser.click(screen.getByRole('button', { name: 'Create and add' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to create user.')
    expect(screen.getByRole('button', { name: 'Create and add' })).toBeInTheDocument()
  })

  it('cancels without submitting', async () => {
    mockAsAdmin()
    const typeUser = userEvent.setup()

    renderWithProviders(<LabMembers labId={9} />)
    await openCreateUserForm(typeUser)
    await typeUser.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(screen.queryByRole('button', { name: 'Create and add' })).not.toBeInTheDocument()
    expect(createLabMembershipForNewUser).not.toHaveBeenCalled()
  })
})
