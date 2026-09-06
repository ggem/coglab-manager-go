import { useState, type SubmitEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  errorMessage,
  getLabMembers,
  listExperimentTrainingRequirements,
  scheduleAppointment,
  searchAppointmentAvailability,
  type CandidateSlot,
} from './api'

interface Props {
  appointmentId: number
  experimentId: number
  labId: number
  onScheduled: () => void
}

// The two-phase search's result surfaced to a coordinator: pick a date
// range, see every slot the backend proved has a complete, consistent
// staff assignment, and commit one. handleScheduleAppointment
// re-validates server-side at commit time, so a slot going stale between
// search and click just comes back as a 409 (surfaced via actionError)
// rather than corrupting anything.
export default function AppointmentScheduleFlow({ appointmentId, experimentId, labId, onScheduled }: Props) {
  const queryClient = useQueryClient()
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [searched, setSearched] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  const { data: roles } = useQuery({
    queryKey: ['experiment-training-requirements', experimentId],
    queryFn: () => listExperimentTrainingRequirements(experimentId),
  })
  const { data: members } = useQuery({ queryKey: ['lab-members', labId], queryFn: () => getLabMembers(labId) })

  const {
    data: slots,
    isFetching,
    error,
    refetch,
  } = useQuery({
    queryKey: ['appointment-availability', appointmentId, startDate, endDate],
    queryFn: () => searchAppointmentAvailability(appointmentId, startDate, endDate),
    enabled: false,
  })

  const scheduleMutation = useMutation({
    mutationFn: (slot: CandidateSlot) => scheduleAppointment(appointmentId, slot.date, slot.start_time),
    onSuccess: () => {
      setActionError(null)
      void queryClient.invalidateQueries({ queryKey: ['appointments', experimentId] })
      onScheduled()
    },
    onError: (err) => setActionError(errorMessage(err, 'That slot is no longer available.')),
  })

  function roleName(roleId: string): string {
    return roles?.find((r) => String(r.id) === roleId)?.name ?? `Role ${roleId}`
  }
  function memberName(userId: number): string {
    const m = members?.find((m) => m.id === userId)
    return m ? `${m.first_name} ${m.last_name}` : `User ${userId}`
  }

  function handleSearch(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    setSearched(true)
    void refetch()
  }

  return (
    <div className="schedule-flow">
      <h4>Find a date/time</h4>
      <form onSubmit={handleSearch}>
        <label>
          Start date
          <input type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} required />
        </label>
        <label>
          End date
          <input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} required />
        </label>
        <button type="submit" disabled={isFetching}>
          {isFetching ? 'Searching…' : 'Search'}
        </button>
      </form>
      {actionError && (
        <p className="error" role="alert">
          {actionError}
        </p>
      )}
      {error && (
        <p className="error" role="alert">
          {errorMessage(error, 'Failed to search availability.')}
        </p>
      )}
      {searched && !isFetching && !error && (
        <ul className="candidate-slots">
          {(slots ?? []).length === 0 && <li>No available slots in that range.</li>}
          {slots?.map((slot) => (
            <li key={`${slot.date}-${slot.start_time}`}>
              <span>
                {slot.date} at {slot.start_time}
                {' -- '}
                {Object.entries(slot.assignment)
                  .map(([roleId, userId]) => {
                    const isGreeter = userId === slot.greeter_id
                    return `${roleName(roleId)}: ${memberName(userId)}${isGreeter ? ' (greeter)' : ''}`
                  })
                  .join(', ')}
                {slot.has_sitter ? ' -- has sitter' : ''}
              </span>
              <button
                type="button"
                onClick={() => scheduleMutation.mutate(slot)}
                disabled={scheduleMutation.isPending}
              >
                Schedule this
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
