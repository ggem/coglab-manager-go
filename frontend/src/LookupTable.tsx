import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Fragment, useState, type ReactNode, type SubmitEvent } from 'react'
import { errorMessage } from './api'

// The six lab-setup lookup tables (conditions, equipment, experiment
// roles, protocols, grants, zip codes) share the same create/list/
// update/deactivate shape.  This is the one generic table+form built
// for all of them, configured per resource by its caller (see
// LabSetup.tsx) rather than knowing about any specific resource itself.

export interface LookupField {
  key: string
  label: string
  type: 'text' | 'number' | 'select'
  // Required when type is 'select' -- the fixed set of values a column
  // like guardians.education/phone_type's CHECK constraint allows.
  options?: { value: string; label: string }[]
  // Defaults to true (every existing lookup table's fields are
  // required) -- set false for an optional enum like phone_type, which
  // has no NOT NULL constraint.
  required?: boolean
}

interface Row {
  id: number
  deactivated: boolean
}

// Dynamic field access (row[f.key], driven by the caller's field config)
// needs an index signature TypeScript won't infer from a generic type
// parameter alone -- this one cast, contained to this single access
// point, stands in for it rather than forcing every concrete row type
// (Condition, Equipment, ...) to declare an index signature of its own.
function fieldValue(row: Row, key: string): unknown {
  return (row as unknown as Record<string, unknown>)[key]
}

// A select field displays its option's label, not the raw stored value
// (e.g. "High school graduate" rather than "hs_grad_no_college") --
// every other field type keeps its existing plain String() display.
function fieldDisplay(row: Row, f: LookupField): string {
  if (f.type === 'select' && f.options) {
    const opt = f.options.find((o) => o.value === fieldValue(row, f.key))
    if (opt) return opt.label
  }
  return String(fieldValue(row, f.key))
}

interface Props<T extends Row> {
  queryKey: unknown[]
  fields: LookupField[]
  list: () => Promise<T[]>
  create: (values: Record<string, string>) => Promise<T>
  update: (id: number, values: Record<string, string>) => Promise<T>
  deactivate: (id: number) => Promise<void>
  // Extra per-row actions alongside the standard Edit/Deactivate (e.g.
  // the experiment-roles sitter toggle, which is its own dedicated
  // endpoint, not a regular field edit).
  extraActions?: (row: T) => ReactNode
  // Nested content shown in an extra row when a row is expanded (e.g.
  // a condition's values) -- LookupTable doesn't know what this is,
  // just renders whatever the caller returns.
  renderExpanded?: (row: T) => ReactNode
}

function emptyValues(fields: LookupField[]): Record<string, string> {
  return Object.fromEntries(fields.map((f) => [f.key, '']))
}

export default function LookupTable<T extends Row>({
  queryKey,
  fields,
  list,
  create,
  update,
  deactivate,
  extraActions,
  renderExpanded,
}: Props<T>) {
  const queryClient = useQueryClient()
  const { data, isLoading, error } = useQuery({ queryKey, queryFn: list })
  const [showDeactivated, setShowDeactivated] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editValues, setEditValues] = useState<Record<string, string>>({})
  const [newValues, setNewValues] = useState<Record<string, string>>(emptyValues(fields))
  const [actionError, setActionError] = useState<string | null>(null)
  const [expandedId, setExpandedId] = useState<number | null>(null)

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

  const updateMutation = useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, string> }) => update(id, values),
    onSuccess: () => {
      setEditingId(null)
      setActionError(null)
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to save.')),
  })

  const deactivateMutation = useMutation({
    mutationFn: deactivate,
    onSuccess: () => {
      setActionError(null)
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to deactivate.')),
  })

  function handleCreate(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    createMutation.mutate(newValues)
  }

  function startEdit(row: T) {
    setEditingId(row.id)
    setEditValues(Object.fromEntries(fields.map((f) => [f.key, String(fieldValue(row, f.key) ?? '')])))
  }

  if (isLoading) return <p>Loading…</p>
  if (error) {
    return (
      <p className="error" role="alert">
        {errorMessage(error, 'Failed to load.')}
      </p>
    )
  }

  const rows = (data ?? []).filter((row) => showDeactivated || !row.deactivated)

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
            {renderExpanded && <th></th>}
            {fields.map((f) => (
              <th key={f.key}>{f.label}</th>
            ))}
            <th>Status</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <Fragment key={row.id}>
              <tr>
                {renderExpanded && (
                  <td>
                    <button
                      type="button"
                      className="expand-toggle"
                      onClick={() => setExpandedId(expandedId === row.id ? null : row.id)}
                      aria-expanded={expandedId === row.id}
                    >
                      {expandedId === row.id ? '▾' : '▸'}
                    </button>
                  </td>
                )}
                {fields.map((f) => (
                  <td key={f.key}>
                    {editingId === row.id ? (
                      f.type === 'select' ? (
                        <select
                          value={editValues[f.key] ?? ''}
                          onChange={(e) => setEditValues({ ...editValues, [f.key]: e.target.value })}
                        >
                          {f.required === false && <option value="">—</option>}
                          {f.options?.map((o) => (
                            <option key={o.value} value={o.value}>
                              {o.label}
                            </option>
                          ))}
                        </select>
                      ) : (
                        <input
                          type={f.type}
                          value={editValues[f.key] ?? ''}
                          onChange={(e) => setEditValues({ ...editValues, [f.key]: e.target.value })}
                        />
                      )
                    ) : (
                      fieldDisplay(row, f)
                    )}
                  </td>
                ))}
                <td>{row.deactivated ? 'Deactivated' : 'Active'}</td>
                <td className="lookup-table-actions">
                  {editingId === row.id ? (
                    <>
                      <button
                        type="button"
                        onClick={() => updateMutation.mutate({ id: row.id, values: editValues })}
                        disabled={updateMutation.isPending}
                      >
                        Save
                      </button>
                      <button type="button" onClick={() => setEditingId(null)}>
                        Cancel
                      </button>
                    </>
                  ) : (
                    <>
                      <button type="button" onClick={() => startEdit(row)}>
                        Edit
                      </button>
                      {!row.deactivated && (
                        <button
                          type="button"
                          onClick={() => deactivateMutation.mutate(row.id)}
                          disabled={deactivateMutation.isPending}
                        >
                          Deactivate
                        </button>
                      )}
                      {extraActions?.(row)}
                    </>
                  )}
                </td>
              </tr>
              {renderExpanded && expandedId === row.id && (
                <tr className="expanded-row">
                  <td colSpan={fields.length + 3}>{renderExpanded(row)}</td>
                </tr>
              )}
            </Fragment>
          ))}
        </tbody>
      </table>
      {rows.length === 0 && <p>No {showDeactivated ? '' : 'active '}rows yet.</p>}
      <label className="show-deactivated">
        <input
          type="checkbox"
          checked={showDeactivated}
          onChange={(e) => setShowDeactivated(e.target.checked)}
        />
        Show deactivated
      </label>
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
