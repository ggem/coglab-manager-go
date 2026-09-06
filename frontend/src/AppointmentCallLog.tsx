import { useState, type SubmitEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createAppointmentNote, errorMessage, listAppointmentNotes } from './api'

// The call log a coordinator keeps while working an appointment through
// hold -> call -> schedule -- mirrors FamilyDetail.tsx's ChildNotes
// exactly (same list + add-note shape), just against the appointment's
// own notes (entity_type "appointment" on the backend) instead of the
// child's.
export default function AppointmentCallLog({ appointmentId }: { appointmentId: number }) {
  const queryClient = useQueryClient()
  const {
    data: notes,
    isLoading,
    error,
  } = useQuery({ queryKey: ['appointment-notes', appointmentId], queryFn: () => listAppointmentNotes(appointmentId) })
  const [body, setBody] = useState('')
  const [actionError, setActionError] = useState<string | null>(null)

  const createMutation = useMutation({
    mutationFn: (body: string) => createAppointmentNote(appointmentId, body),
    onSuccess: () => {
      setBody('')
      setActionError(null)
      void queryClient.invalidateQueries({ queryKey: ['appointment-notes', appointmentId] })
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to add note.')),
  })

  if (isLoading) return <p>Loading call log…</p>
  if (error) {
    return (
      <p className="error" role="alert">
        {errorMessage(error, 'Failed to load call log.')}
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
      <h4>Calling log</h4>
      {actionError && (
        <p className="error" role="alert">
          {actionError}
        </p>
      )}
      {sorted.length === 0 && <p>No calls logged yet.</p>}
      <ul>
        {sorted.map((n) => (
          <li key={n.id}>
            <p>{n.body}</p>
            <span className="note-meta">{new Date(n.created_at).toLocaleString()}</span>
          </li>
        ))}
      </ul>
      <form onSubmit={handleSubmit}>
        <textarea value={body} onChange={(e) => setBody(e.target.value)} placeholder="Log this call…" required />
        <button type="submit" disabled={createMutation.isPending}>
          {createMutation.isPending ? 'Logging…' : 'Log call'}
        </button>
      </form>
    </div>
  )
}
