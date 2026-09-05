import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { errorMessage } from './api'

// A genuinely different shape from LookupTable: attach/detach an
// *existing* resource (a lab's conditions, equipment, grants, roles,
// or members) rather than create/edit/deactivate a resource of its
// own. Used five times on ExperimentDetail's page -- the real
// justification for a new component here.

interface Item {
  id: number
  // Present on lookup-table rows (conditions, equipment, roles, grants,
  // experiment types); absent on plain records like LabMember. Optional
  // so the same generic component works for both without every caller
  // having to supply it.
  deactivated?: boolean
}

interface Props<T extends Item> {
  queryKey: unknown[]
  list: () => Promise<T[]>
  options: () => Promise<T[]>
  add: (id: number) => Promise<void>
  remove: (id: number) => Promise<void>
  label: (item: T) => string
  addLabel: string
}

export default function AttachList<T extends Item>({
  queryKey,
  list,
  options,
  add,
  remove,
  label,
  addLabel,
}: Props<T>) {
  const queryClient = useQueryClient()
  const { data: attached, isLoading, error } = useQuery({ queryKey, queryFn: list })
  const optionsQueryKey = [...queryKey, 'options']
  const { data: available } = useQuery({ queryKey: optionsQueryKey, queryFn: options })
  const [selectedId, setSelectedId] = useState('')
  const [actionError, setActionError] = useState<string | null>(null)

  const invalidate = () => queryClient.invalidateQueries({ queryKey })

  const addMutation = useMutation({
    mutationFn: (id: number) => add(id),
    onSuccess: () => {
      setSelectedId('')
      setActionError(null)
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to add.')),
  })

  const removeMutation = useMutation({
    mutationFn: (id: number) => remove(id),
    onSuccess: () => {
      setActionError(null)
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to remove.')),
  })

  if (isLoading) return <p>Loading…</p>
  if (error) {
    return (
      <p className="error" role="alert">
        {errorMessage(error, 'Failed to load.')}
      </p>
    )
  }

  const attachedIds = new Set((attached ?? []).map((item) => item.id))
  // Deactivated items stay visible in the already-attached list (that's
  // real history, not a live choice) but are excluded here so they can't
  // be newly attached.
  const choices = (available ?? []).filter((item) => !attachedIds.has(item.id) && !item.deactivated)

  return (
    <div className="attach-list">
      {actionError && (
        <p className="error" role="alert">
          {actionError}
        </p>
      )}
      {(attached ?? []).length === 0 ? (
        <p>None attached yet.</p>
      ) : (
        <ul>
          {(attached ?? []).map((item) => (
            <li key={item.id}>
              {label(item)}
              <button
                type="button"
                onClick={() => removeMutation.mutate(item.id)}
                disabled={removeMutation.isPending}
                aria-label={`Remove ${label(item)}`}
              >
                Remove
              </button>
            </li>
          ))}
        </ul>
      )}
      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (selectedId) addMutation.mutate(Number(selectedId))
        }}
      >
        <select value={selectedId} onChange={(e) => setSelectedId(e.target.value)} required aria-label={addLabel}>
          <option value="" disabled>
            {addLabel}
          </option>
          {choices.map((item) => (
            <option key={item.id} value={item.id}>
              {label(item)}
            </option>
          ))}
        </select>
        <button type="submit" disabled={addMutation.isPending || selectedId === ''}>
          {addMutation.isPending ? 'Adding…' : 'Add'}
        </button>
      </form>
    </div>
  )
}
