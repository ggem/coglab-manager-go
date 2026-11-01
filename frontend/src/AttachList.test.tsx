import { describe, expect, it, vi } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import AttachList from './AttachList'
import { ApiError } from './api'
import { renderWithProviders } from './test/renderWithProviders'

interface Thing {
  id: number
  name: string
  deactivated?: boolean
}

const ADD_LABEL = 'Add a thing…'

// A small in-memory backend: list() reads the attached set, and add/remove
// change it, so refetch-after-mutation is exercised the way the real API
// would be.
function fakeBackend(initial: Thing[], available: Thing[]) {
  let attached = [...initial]
  return {
    list: vi.fn(async () => attached),
    options: vi.fn(async () => available),
    add: vi.fn(async (id: number) => {
      attached = [...attached, available.find((t) => t.id === id)!]
    }),
    remove: vi.fn(async (id: number) => {
      attached = attached.filter((t) => t.id !== id)
    }),
  }
}

function renderList(backend: ReturnType<typeof fakeBackend>, emptyText?: string) {
  renderWithProviders(
    <AttachList<Thing>
      queryKey={['things']}
      list={backend.list}
      options={backend.options}
      add={backend.add}
      remove={backend.remove}
      label={(t) => t.name}
      addLabel={ADD_LABEL}
      emptyText={emptyText}
    />,
  )
  return userEvent.setup()
}

const apple: Thing = { id: 1, name: 'Apple' }
const banana: Thing = { id: 2, name: 'Banana' }
const retired: Thing = { id: 3, name: 'Retired', deactivated: true }
const discontinued: Thing = { id: 4, name: 'Discontinued', deactivated: true }

function optionNames() {
  return within(screen.getByRole('combobox', { name: ADD_LABEL }))
    .getAllByRole('option')
    .map((o) => o.textContent)
}

describe('AttachList', () => {
  it('shows the default empty message when nothing is attached', async () => {
    renderList(fakeBackend([], [apple]))
    expect(await screen.findByText('None attached yet.')).toBeInTheDocument()
  })

  it('shows a custom empty message when one is given', async () => {
    renderList(fakeBackend([], [apple]), 'No fruit yet.')
    expect(await screen.findByText('No fruit yet.')).toBeInTheDocument()
    expect(screen.queryByText('None attached yet.')).not.toBeInTheDocument()
  })

  it('lists attached items, and offers only unattached, active ones to add', async () => {
    renderList(fakeBackend([apple, retired], [apple, banana, retired, discontinued]))

    expect(await screen.findByText('Apple')).toBeInTheDocument()
    // Deactivated but already attached: still listed as history.
    expect(screen.getByText('Retired')).toBeInTheDocument()
    await waitFor(() => expect(optionNames()).toEqual([ADD_LABEL, 'Banana']))
    expect(screen.getByRole('button', { name: 'Add' })).toBeDisabled()
  })

  it('adds the selected item and refreshes the list', async () => {
    const backend = fakeBackend([], [apple, banana])
    const user = renderList(backend)
    await screen.findByText('None attached yet.')
    await waitFor(() => expect(optionNames()).toContain('Banana'))

    await user.selectOptions(screen.getByRole('combobox', { name: ADD_LABEL }), 'Banana')
    await user.click(screen.getByRole('button', { name: 'Add' }))

    expect(backend.add).toHaveBeenCalledWith(2)
    expect(await screen.findByRole('button', { name: 'Remove Banana' })).toBeInTheDocument()
    expect(screen.queryByText('None attached yet.')).not.toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: ADD_LABEL })).toHaveValue('')
    expect(optionNames()).toEqual([ADD_LABEL, 'Apple'])
  })

  it('removes an item and refreshes the list', async () => {
    const backend = fakeBackend([apple], [apple])
    const user = renderList(backend)

    await user.click(await screen.findByRole('button', { name: 'Remove Apple' }))

    expect(backend.remove).toHaveBeenCalledWith(1)
    expect(await screen.findByText('None attached yet.')).toBeInTheDocument()
  })

  it("shows the server's message when adding fails, and keeps the list as it was", async () => {
    const backend = fakeBackend([], [apple])
    backend.add.mockRejectedValueOnce(new ApiError(409, 'Apple is already attached'))
    const user = renderList(backend)
    await waitFor(() => expect(optionNames()).toContain('Apple'))

    await user.selectOptions(screen.getByRole('combobox', { name: ADD_LABEL }), 'Apple')
    await user.click(screen.getByRole('button', { name: 'Add' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Apple is already attached')
    expect(screen.getByText('None attached yet.')).toBeInTheDocument()
  })

  it('shows a fallback message when removing fails for a non-API reason', async () => {
    const backend = fakeBackend([apple], [apple])
    backend.remove.mockRejectedValueOnce(new Error('boom'))
    const user = renderList(backend)

    await user.click(await screen.findByRole('button', { name: 'Remove Apple' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to remove.')
    expect(screen.getByText('Apple')).toBeInTheDocument()
  })

  it('clears an earlier error after a later action succeeds', async () => {
    const backend = fakeBackend([apple], [apple])
    backend.remove.mockRejectedValueOnce(new Error('boom'))
    const user = renderList(backend)

    await user.click(await screen.findByRole('button', { name: 'Remove Apple' }))
    await screen.findByRole('alert')
    await user.click(screen.getByRole('button', { name: 'Remove Apple' }))

    expect(await screen.findByText('None attached yet.')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows an error instead of the list when loading fails', async () => {
    const backend = fakeBackend([], [apple])
    backend.list.mockRejectedValueOnce(new ApiError(500, 'Database unavailable'))
    renderList(backend)

    expect(await screen.findByRole('alert')).toHaveTextContent('Database unavailable')
    expect(screen.queryByRole('combobox', { name: ADD_LABEL })).not.toBeInTheDocument()
  })
})
