import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import Availability from './Availability'
import { renderWithProviders } from './test/renderWithProviders'
import type { LabAvailabilityGeneral, LabMembership } from './api'

// Scoped to the coordinator/admin "Whose availability" picker.

const {
  getMe,
  listLabMemberships,
  listLabAvailabilityGeneral,
  listLabAvailabilitySpecific,
  listLabAvailabilityGeneralForUser,
  listLabAvailabilitySpecificForUser,
  createLabAvailabilityGeneralForUser,
} = vi.hoisted(() => ({
  getMe: vi.fn(),
  listLabMemberships: vi.fn(),
  listLabAvailabilityGeneral: vi.fn(),
  listLabAvailabilitySpecific: vi.fn(),
  listLabAvailabilityGeneralForUser: vi.fn(),
  listLabAvailabilitySpecificForUser: vi.fn(),
  createLabAvailabilityGeneralForUser: vi.fn(),
}))

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  getMe,
  listLabMemberships,
  listLabAvailabilityGeneral,
  listLabAvailabilitySpecific,
  listLabAvailabilityGeneralForUser,
  listLabAvailabilitySpecificForUser,
  createLabAvailabilityGeneralForUser,
}))

function membership(user_id: number, first_name: string, role_name: string): LabMembership {
  return {
    user_id,
    first_name,
    last_name: 'Person',
    email: `${first_name.toLowerCase()}@example.edu`,
    role_id: 1,
    role_name,
    priority: 'undergrad_no_project',
  }
}

function generalRow(id: number, user_id: number, start_time: string): LabAvailabilityGeneral {
  return { id, user_id, lab_id: 1, weekday: 1, start_time, end_time: '17:00', created_at: '' }
}

function setup(myRole: string) {
  getMe.mockResolvedValue({
    user: { id: 1, email: 'me@example.edu', first_name: 'Me', last_name: 'Person', is_platform_admin: false },
  })
  listLabMemberships.mockResolvedValue([
    membership(1, 'Me', myRole),
    membership(2, 'Student', 'staff'),
    membership(3, 'Other', 'staff'),
  ])
  listLabAvailabilityGeneral.mockResolvedValue([generalRow(10, 1, '08:00')])
  listLabAvailabilitySpecific.mockResolvedValue([])
  listLabAvailabilityGeneralForUser.mockImplementation(async (_labId: number, userId: number) =>
    userId === 2 ? [generalRow(20, 2, '13:00')] : [generalRow(30, 3, '15:00')],
  )
  listLabAvailabilitySpecificForUser.mockResolvedValue([])

  renderWithProviders(
    <Routes>
      <Route path="/labs/:labId/availability" element={<Availability />} />
    </Routes>,
    { route: '/labs/1/availability' },
  )
  return userEvent.setup()
}

afterEach(() => {
  vi.clearAllMocks()
})

describe('Availability member picker', () => {
  it('lets a coordinator switch to another member and add availability for them', async () => {
    const user = setup('coordinator')
    await screen.findByText('08:00')

    const picker = await screen.findByLabelText('Whose availability')
    expect(picker).toHaveValue('')

    await user.selectOptions(picker, 'Person, Student')

    expect(await screen.findByText('13:00')).toBeInTheDocument()
    expect(screen.queryByText('08:00')).not.toBeInTheDocument()
    expect(listLabAvailabilityGeneralForUser).toHaveBeenCalledWith(1, 2)

    createLabAvailabilityGeneralForUser.mockResolvedValue(generalRow(21, 2, '09:00'))
    const [dayPicker] = screen.getAllByLabelText('Day')
    await user.selectOptions(dayPicker, 'Tuesday')
    const [start] = screen.getAllByPlaceholderText('Start time')
    const [end] = screen.getAllByPlaceholderText('End time')
    await user.type(start, '09:00')
    await user.type(end, '12:00')
    await user.click(screen.getAllByRole('button', { name: 'Add' })[0])

    await waitFor(() => expect(createLabAvailabilityGeneralForUser).toHaveBeenCalledWith(1, 2, 2, '09:00', '12:00'))

    await user.selectOptions(picker, 'Myself')
    expect(await screen.findByText('08:00')).toBeInTheDocument()
    expect(screen.queryByText('13:00')).not.toBeInTheDocument()
  })

  it('switching directly between two other members shows only the new member, with a fresh add form', async () => {
    const user = setup('coordinator')
    const picker = await screen.findByLabelText('Whose availability')

    await user.selectOptions(picker, 'Person, Student')
    expect(await screen.findByText('13:00')).toBeInTheDocument()

    // Half-filled add form for Student, which must not carry over.
    const [studentStart] = screen.getAllByPlaceholderText('Start time')
    await user.type(studentStart, '10:00')
    expect(studentStart).toHaveValue('10:00')

    await user.selectOptions(picker, 'Person, Other')

    expect(await screen.findByText('15:00')).toBeInTheDocument()
    expect(screen.queryByText('13:00')).not.toBeInTheDocument()
    expect(listLabAvailabilityGeneralForUser).toHaveBeenLastCalledWith(1, 3)
    const [otherStart] = screen.getAllByPlaceholderText('Start time')
    expect(otherStart).toHaveValue('')
  })

  it('also shows the picker to an admin', async () => {
    setup('admin')
    expect(await screen.findByLabelText('Whose availability')).toBeInTheDocument()
  })

  it('does not show the picker to staff', async () => {
    setup('staff')
    await screen.findByText('08:00')
    await waitFor(() => expect(listLabMemberships).toHaveBeenCalled())

    expect(screen.queryByLabelText('Whose availability')).not.toBeInTheDocument()
    expect(listLabAvailabilityGeneralForUser).not.toHaveBeenCalled()
  })
})
