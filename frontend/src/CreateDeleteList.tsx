import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type SubmitEvent } from 'react'
import { errorMessage } from './api'
import type { LookupField } from './LookupTable'

// A third generic list shape, alongside LookupTable (create/edit/
// deactivate-with-visibility-toggle) and AttachList (attach/detach an
// *existing* lab-wide row by id). This one is "create your own new row,
// list your active rows, delete one" -- no edit, no show-deactivated
// toggle -- matching general/specific-date availability and schedule
// blockings exactly: their list queries already filter to
// deactivated_at is null, and none of the three has an update endpoint.
// Shares LookupField's config shape rather than inventing a new one.

interface Row {
  id: number
}

function fieldValue(row: Row, key: string): unknown {
  return (row as unknown as Record<string, unknown>)[key]
}

function fieldDisplay(row: Row, f: LookupField): string {
  if (f.type === 'select' && f.options) {
    // String() on both sides: option values are always strings (they
    // come from a <select>), but the API value behind a field like
    // weekday is a number -- compared with ===, `1 === '1'` is false.
    const opt = f.options.find((o) => o.value === String(fieldValue(row, f.key)))
    if (opt) return opt.label
  }
  return String(fieldValue(row, f.key))
}

function emptyValues(fields: LookupField[]): Record<string, string> {
  return Object.fromEntries(fields.map((f) => [f.key, '']))
}

interface Props<T extends Row> {
  queryKey: unknown[]
  fields: LookupField[]
  list: () => Promise<T[]>
  create: (values: Record<string, string>) => Promise<T>
  remove: (id: number) => Promise<void>
}

export default function CreateDeleteList<T extends Row>({ queryKey, fields, list, create, remove }: Props<T>) {
  const queryClient = useQueryClient()
  const { data, isLoading, error } = useQuery({ queryKey, queryFn: list })
  const [newValues, setNewValues] = useState<Record<string, string>>(emptyValues(fields))
  const [actionError, setActionError] = useState<string | null>(null)

  const invalidate = () => queryClient.invalidateQueries({ queryKey })

  const createMutation = useMutation({
    mutationFn: create,
    onSuccess: () => {
      setNewValues(emptyValues(fields))
      setActionError(null)
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to create.')),
  })

  const removeMutation = useMutation({
    mutationFn: remove,
    onSuccess: () => {
      setActionError(null)
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to remove.')),
  })

  function handleCreate(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    createMutation.mutate(newValues)
  }

  if (isLoading) return <p>Loading…</p>
  if (error) {
    return (
      <p className="error" role="alert">
        {errorMessage(error, 'Failed to load.')}
      </p>
    )
  }

  const rows = data ?? []

  return (
    <div className="lookup-table">
      {actionError && (
        <p className="error" role="alert">
          {actionError}
        </p>
      )}
      <table>
        <thead>
          <tr>
            {fields.map((f) => (
              <th key={f.key}>{f.label}</th>
            ))}
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.id}>
              {fields.map((f) => (
                <td key={f.key}>{fieldDisplay(row, f)}</td>
              ))}
              <td className="lookup-table-actions">
                <button
                  type="button"
                  onClick={() => removeMutation.mutate(row.id)}
                  disabled={removeMutation.isPending}
                >
                  Remove
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {rows.length === 0 && <p>None yet.</p>}
      <form onSubmit={handleCreate} className="lookup-table-add">
        {fields.map((f) =>
          f.type === 'select' ? (
            <select
              key={f.key}
              aria-label={f.label}
              value={newValues[f.key] ?? ''}
              onChange={(e) => setNewValues({ ...newValues, [f.key]: e.target.value })}
              required={f.required !== false}
            >
              <option value="" disabled={f.required !== false}>
                {f.label}
              </option>
              {f.options?.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          ) : (
            <input
              key={f.key}
              type={f.type}
              placeholder={f.label}
              value={newValues[f.key] ?? ''}
              onChange={(e) => setNewValues({ ...newValues, [f.key]: e.target.value })}
              required={f.required !== false}
            />
          ),
        )}
        <button type="submit" disabled={createMutation.isPending}>
          {createMutation.isPending ? 'Adding…' : 'Add'}
        </button>
      </form>
    </div>
  )
}
