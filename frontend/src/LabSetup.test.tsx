import { describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import LabSetup from './LabSetup'
import { renderWithProviders } from './test/renderWithProviders'
import type { ScheduleBlocking } from './api'

// Scoped to the Schedule Blockings tab's default date filter.

const { listConditions, listScheduleBlockings } = vi.hoisted(() => ({
  listConditions: vi.fn(),
  listScheduleBlockings: vi.fn(),
}))

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  listConditions,
  listScheduleBlockings,
}))

function daysFromToday(n: number): string {
  const d = new Date()
  d.setDate(d.getDate() + n)
  const pad = (x: number) => String(x).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function blocking(id: number, date: string, reason: string): ScheduleBlocking {
  return { id, lab_id: 1, date, start_time: '09:00', end_time: '17:00', reason, created_at: '' }
}

function renderBlockingsTab() {
  renderWithProviders(
    <Routes>
      <Route path="/labs/:labId/setup" element={<LabSetup />} />
    </Routes>,
    { route: '/labs/1/setup' },
  )
  return userEvent.setup()
}

describe('LabSetup schedule blockings', () => {
  it('shows only blockings from a week ago onward by default, and all of them via "Show all blockings"', async () => {
    listConditions.mockResolvedValue([])
    listScheduleBlockings.mockResolvedValue([
      blocking(1, daysFromToday(-30), 'Old closure'),
      blocking(2, daysFromToday(-8), 'Just too old'),
      blocking(3, daysFromToday(-7), 'Exactly a week ago'),
      blocking(4, daysFromToday(10), 'Upcoming holiday'),
    ])
    const user = renderBlockingsTab()

    await user.click(await screen.findByRole('tab', { name: 'Schedule Blockings' }))
    await screen.findByText('Upcoming holiday')

    expect(screen.getByText('Exactly a week ago')).toBeInTheDocument()
    expect(screen.queryByText('Just too old')).not.toBeInTheDocument()
    expect(screen.queryByText('Old closure')).not.toBeInTheDocument()

    const showAll = screen.getByRole('checkbox', { name: 'Show all blockings' })
    expect(showAll).not.toBeChecked()
    await user.click(showAll)

    expect(screen.getByText('Old closure')).toBeInTheDocument()
    expect(screen.getByText('Just too old')).toBeInTheDocument()

    await user.click(showAll)

    expect(screen.queryByText('Old closure')).not.toBeInTheDocument()
    expect(screen.queryByText('Just too old')).not.toBeInTheDocument()
    expect(screen.getByText('Upcoming holiday')).toBeInTheDocument()
  })

  it('says so when every blocking is older than a week, and still reveals them via "Show all blockings"', async () => {
    listConditions.mockResolvedValue([])
    listScheduleBlockings.mockResolvedValue([blocking(1, daysFromToday(-30), 'Old closure')])
    const user = renderBlockingsTab()

    await user.click(await screen.findByRole('tab', { name: 'Schedule Blockings' }))

    expect(await screen.findByText('No blockings from the past week onward.')).toBeInTheDocument()

    await user.click(screen.getByRole('checkbox', { name: 'Show all blockings' }))

    expect(screen.getByText('Old closure')).toBeInTheDocument()
    expect(screen.queryByText('No blockings from the past week onward.')).not.toBeInTheDocument()
  })
})
