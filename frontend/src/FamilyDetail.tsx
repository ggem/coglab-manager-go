import { Fragment, useState, type SubmitEvent } from 'react'
import { useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import LookupTable from './LookupTable'
import ChildForm from './ChildForm'
import {
  createChild,
  createChildNote,
  createGuardian,
  deactivateChild,
  deactivateGuardian,
  errorMessage,
  getFamily,
  listChildNotes,
  listChildrenByFamily,
  listGuardiansByFamily,
  listRecruitmentSources,
  updateChild,
  updateFamily,
  updateGuardian,
  type ChildInput,
  type Family,
  type FamilyInput,
  type Guardian,
  type GuardianInput,
  type RecruitmentSource,
} from './api'

const CONTACT_METHODS = [
  { value: 'home_phone', label: 'Home phone' },
  { value: 'work_phone', label: 'Work phone' },
  { value: 'mobile_phone', label: 'Mobile phone' },
  { value: 'fax', label: 'Fax' },
  { value: 'email', label: 'Email' },
  { value: 'snail_mail', label: 'Mail' },
]

const EDUCATION_OPTIONS = [
  { value: 'unknown', label: 'Unknown' },
  { value: 'without_high_school_diploma', label: 'Without high school diploma' },
  { value: 'hs_grad_no_college', label: 'High school graduate, no college' },
  { value: 'hs_grad_some_college', label: 'High school graduate, some college' },
  { value: 'degree_from_4yr_college_or_higher', label: '4-year degree or higher' },
  { value: 'left_blank', label: 'Left blank' },
]

const PHONE_TYPE_OPTIONS = [
  { value: 'home', label: 'Home' },
  { value: 'work', label: 'Work' },
  { value: 'mobile', label: 'Mobile' },
  { value: 'fax', label: 'Fax' },
  { value: 'pager', label: 'Pager' },
  { value: 'disconnected', label: 'Disconnected' },
  { value: 'other', label: 'Other' },
]

export default function FamilyDetail() {
  const { familyId } = useParams<{ familyId: string }>()
  const id = Number(familyId)
  const { data: recruitmentSources } = useQuery({
    queryKey: ['recruitment-sources'],
    queryFn: listRecruitmentSources,
  })

  return (
    <div className="family-detail">
      <FamilyFields familyId={id} />
      <h3>Guardians</h3>
      <GuardiansTable familyId={id} />
      <h3>Children</h3>
      <ChildrenSection familyId={id} recruitmentSources={recruitmentSources ?? []} />
    </div>
  )
}

function FamilyFields({ familyId }: { familyId: number }) {
  const queryClient = useQueryClient()
  const {
    data: family,
    isLoading,
    error,
  } = useQuery({ queryKey: ['family', familyId], queryFn: () => getFamily(familyId) })
  const [editing, setEditing] = useState(false)
  const [values, setValues] = useState<FamilyInput | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  const updateMutation = useMutation({
    mutationFn: (input: FamilyInput) => updateFamily(familyId, input),
    onSuccess: () => {
      setEditing(false)
      setActionError(null)
      void queryClient.invalidateQueries({ queryKey: ['family', familyId] })
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to save.')),
  })

  if (isLoading) return <p>Loading…</p>
  if (error) {
    return (
      <p className="error" role="alert">
        {errorMessage(error, 'Failed to load family.')}
      </p>
    )
  }
  if (!family) return null

  function startEdit(f: Family) {
    setValues({
      address: f.address,
      city: f.city,
      state: f.state,
      zip: f.zip,
      preferred_contact_method: f.preferred_contact_method,
    })
    setEditing(true)
  }

  function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    if (values) updateMutation.mutate(values)
  }

  return (
    <section className="family-fields">
      {actionError && (
        <p className="error" role="alert">
          {actionError}
        </p>
      )}
      {editing && values ? (
        <form onSubmit={handleSubmit}>
          <label>
            Address
            <input value={values.address} onChange={(e) => setValues({ ...values, address: e.target.value })} />
          </label>
          <label>
            City
            <input value={values.city} onChange={(e) => setValues({ ...values, city: e.target.value })} />
          </label>
          <label>
            State
            <input value={values.state} onChange={(e) => setValues({ ...values, state: e.target.value })} />
          </label>
          <label>
            Zip
            <input value={values.zip} onChange={(e) => setValues({ ...values, zip: e.target.value })} />
          </label>
          <label>
            Preferred contact
            <select
              value={values.preferred_contact_method ?? ''}
              onChange={(e) =>
                setValues({ ...values, preferred_contact_method: e.target.value === '' ? null : e.target.value })
              }
            >
              <option value="">—</option>
              {CONTACT_METHODS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </label>
          <button type="submit" disabled={updateMutation.isPending}>
            {updateMutation.isPending ? 'Saving…' : 'Save'}
          </button>
          <button type="button" onClick={() => setEditing(false)}>
            Cancel
          </button>
        </form>
      ) : (
        <div>
          <p>
            {family.address}, {family.city}, {family.state} {family.zip}
          </p>
          <p>
            Preferred contact:{' '}
            {CONTACT_METHODS.find((o) => o.value === family.preferred_contact_method)?.label ?? '—'}
          </p>
          <button type="button" onClick={() => startEdit(family)}>
            Edit
          </button>
        </div>
      )}
    </section>
  )
}

function toGuardianInput(values: Record<string, string>): GuardianInput {
  return {
    first_name: values.first_name,
    last_name: values.last_name,
    education: values.education,
    occupation: values.occupation,
    phone_number: values.phone_number,
    phone_type: values.phone_type === '' ? null : values.phone_type,
    email: values.email,
  }
}

function GuardiansTable({ familyId }: { familyId: number }) {
  return (
    <LookupTable<Guardian>
      queryKey={['guardians', familyId]}
      fields={[
        { key: 'first_name', label: 'First name', type: 'text' },
        { key: 'last_name', label: 'Last name', type: 'text' },
        { key: 'education', label: 'Education', type: 'select', options: EDUCATION_OPTIONS },
        { key: 'occupation', label: 'Occupation', type: 'text' },
        { key: 'phone_number', label: 'Phone number', type: 'text' },
        { key: 'phone_type', label: 'Phone type', type: 'select', options: PHONE_TYPE_OPTIONS, required: false },
        { key: 'email', label: 'Email', type: 'text' },
      ]}
      list={() => listGuardiansByFamily(familyId)}
      create={(values) => createGuardian(familyId, toGuardianInput(values))}
      update={(id, values) => updateGuardian(id, toGuardianInput(values))}
      deactivate={(id) => deactivateGuardian(id)}
    />
  )
}

function DeactivateChildButton({
  onConfirm,
  pending,
}: {
  onConfirm: (reason: string) => void
  pending: boolean
}) {
  const [reason, setReason] = useState<string | null>(null)

  if (reason === null) {
    return (
      <button type="button" onClick={() => setReason('')}>
        Deactivate
      </button>
    )
  }

  return (
    <span className="deactivate-reason">
      <input placeholder="Reason" value={reason} onChange={(e) => setReason(e.target.value)} />
      <button type="button" onClick={() => onConfirm(reason)} disabled={pending || reason.trim() === ''}>
        Confirm
      </button>
      <button type="button" onClick={() => setReason(null)}>
        Cancel
      </button>
    </span>
  )
}

function ChildNotes({ childId }: { childId: number }) {
  const queryClient = useQueryClient()
  const {
    data: notes,
    isLoading,
    error,
  } = useQuery({ queryKey: ['notes', childId], queryFn: () => listChildNotes(childId) })
  const [body, setBody] = useState('')
  const [actionError, setActionError] = useState<string | null>(null)

  const createMutation = useMutation({
    mutationFn: (body: string) => createChildNote(childId, body),
    onSuccess: () => {
      setBody('')
      setActionError(null)
      void queryClient.invalidateQueries({ queryKey: ['notes', childId] })
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to add note.')),
  })

  if (isLoading) return <p>Loading notes…</p>
  if (error) {
    return (
      <p className="error" role="alert">
        {errorMessage(error, 'Failed to load notes.')}
      </p>
    )
  }

  const sorted = [...(notes ?? [])].sort((a, b) => b.created_at.localeCompare(a.created_at))

  function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    if (body.trim()) createMutation.mutate(body)
  }

  return (
    <div className="child-notes">
      <h4>Notes</h4>
      {actionError && (
        <p className="error" role="alert">
          {actionError}
        </p>
      )}
      {sorted.length === 0 && <p>No notes yet.</p>}
      <ul>
        {sorted.map((n) => (
          <li key={n.id}>
            <p>{n.body}</p>
            <span className="note-meta">{new Date(n.created_at).toLocaleString()}</span>
          </li>
        ))}
      </ul>
      <form onSubmit={handleSubmit}>
        <textarea value={body} onChange={(e) => setBody(e.target.value)} placeholder="Add a note…" required />
        <button type="submit" disabled={createMutation.isPending}>
          {createMutation.isPending ? 'Adding…' : 'Add note'}
        </button>
      </form>
    </div>
  )
}

function ChildrenSection({
  familyId,
  recruitmentSources,
}: {
  familyId: number
  recruitmentSources: RecruitmentSource[]
}) {
  const queryClient = useQueryClient()
  const {
    data: children,
    isLoading,
    error,
  } = useQuery({ queryKey: ['children', familyId], queryFn: () => listChildrenByFamily(familyId) })
  const [showDeactivated, setShowDeactivated] = useState(false)
  const [expandedId, setExpandedId] = useState<number | null>(null)
  const [showAddForm, setShowAddForm] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['children', familyId] })

  const createMutation = useMutation({
    mutationFn: (input: ChildInput) => createChild(familyId, input),
    onSuccess: () => {
      setActionError(null)
      setShowAddForm(false)
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to create.')),
  })
  const updateMutation = useMutation({
    mutationFn: ({ id, input }: { id: number; input: ChildInput }) => updateChild(id, input),
    onSuccess: () => {
      setActionError(null)
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to save.')),
  })
  const deactivateMutation = useMutation({
    mutationFn: ({ id, reason }: { id: number; reason: string }) => deactivateChild(id, reason),
    onSuccess: () => {
      setActionError(null)
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to deactivate.')),
  })

  if (isLoading) return <p>Loading…</p>
  if (error) {
    return (
      <p className="error" role="alert">
        {errorMessage(error, 'Failed to load children.')}
      </p>
    )
  }

  const rows = (children ?? []).filter((c) => showDeactivated || !c.deactivated)

  return (
    <div className="children-section">
      {actionError && (
        <p className="error" role="alert">
          {actionError}
        </p>
      )}
      <table>
        <thead>
          <tr>
            <th></th>
            <th>Name</th>
            <th>Sex</th>
            <th>Birth date</th>
            <th>Status</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((c) => (
            <Fragment key={c.id}>
              <tr>
                <td>
                  <button
                    type="button"
                    className="expand-toggle"
                    onClick={() => setExpandedId(expandedId === c.id ? null : c.id)}
                    aria-expanded={expandedId === c.id}
                  >
                    {expandedId === c.id ? '▾' : '▸'}
                  </button>
                </td>
                <td>
                  {c.first_name} {c.last_name}
                </td>
                <td>{c.sex}</td>
                <td>{c.birth_date ?? '—'}</td>
                <td>{c.deactivated ? 'Deactivated' : 'Active'}</td>
                <td className="lookup-table-actions">
                  {!c.deactivated && (
                    <DeactivateChildButton
                      onConfirm={(reason) => deactivateMutation.mutate({ id: c.id, reason })}
                      pending={deactivateMutation.isPending}
                    />
                  )}
                </td>
              </tr>
              {expandedId === c.id && (
                <tr className="expanded-row">
                  <td colSpan={6}>
                    <ChildForm
                      mode="edit"
                      initial={c}
                      recruitmentSources={recruitmentSources}
                      onSubmit={(input) => updateMutation.mutate({ id: c.id, input })}
                      submitting={updateMutation.isPending}
                      submitLabel="Save"
                    />
                    <ChildNotes childId={c.id} />
                  </td>
                </tr>
              )}
            </Fragment>
          ))}
        </tbody>
      </table>
      {rows.length === 0 && <p>No {showDeactivated ? '' : 'active '}children yet.</p>}
      <label className="show-deactivated">
        <input
          type="checkbox"
          checked={showDeactivated}
          onChange={(e) => setShowDeactivated(e.target.checked)}
        />
        Show deactivated
      </label>
      {showAddForm ? (
        <ChildForm
          mode="create"
          recruitmentSources={recruitmentSources}
          onSubmit={(input) => createMutation.mutate(input)}
          submitting={createMutation.isPending}
          submitLabel="Add child"
          error={null}
        />
      ) : (
        <button type="button" onClick={() => setShowAddForm(true)}>
          Add child
        </button>
      )}
    </div>
  )
}
