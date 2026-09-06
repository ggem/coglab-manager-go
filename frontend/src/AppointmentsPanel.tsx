import { useState, type SubmitEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import AppointmentRow from './AppointmentRow'
import { errorMessage, holdChildrenForExperiment, listAppointmentsByExperiment, type HoldChildrenInput } from './api'

interface Props {
  experimentId: number
  labId: number
}

const STATUS_TABS: { key: string; label: string }[] = [
  { key: 'to_be_scheduled', label: 'To be scheduled' },
  { key: 'pending', label: 'Pending' },
  { key: 'released', label: 'Released' },
  { key: 'arrived', label: 'Arrived' },
]

interface HoldValues {
  start_date: string
  end_date: string
  count: string
  sort: 'oldest' | 'random'
  sex: string
}

function emptyHoldValues(): HoldValues {
  return { start_date: '', end_date: '', count: '10', sort: 'oldest', sex: '' }
}

export default function AppointmentsPanel({ experimentId, labId }: Props) {
  const queryClient = useQueryClient()
  const [statusTab, setStatusTab] = useState('to_be_scheduled')
  const [holdValues, setHoldValues] = useState(emptyHoldValues())
  const [holdError, setHoldError] = useState<string | null>(null)

  const {
    data: appointments,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['appointments', experimentId, statusTab],
    queryFn: () => listAppointmentsByExperiment(experimentId, statusTab),
  })

  const holdMutation = useMutation({
    mutationFn: (input: HoldChildrenInput) => holdChildrenForExperiment(experimentId, input),
    onSuccess: (held) => {
      setHoldError(null)
      void queryClient.invalidateQueries({ queryKey: ['appointments', experimentId] })
      if (held.length === 0) setHoldError('No eligible children found for that range.')
    },
    onError: (err) => setHoldError(errorMessage(err, 'Failed to hold children.')),
  })

  function handleHold(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    holdMutation.mutate({
      start_date: holdValues.start_date,
      end_date: holdValues.end_date,
      count: Number(holdValues.count),
      sort: holdValues.sort,
      sex: holdValues.sex || null,
    })
  }

  return (
    <div className="appointments-panel">
      <h3>Appointments</h3>

      <form onSubmit={handleHold} className="hold-children-form">
        <label>
          Start date
          <input
            type="date"
            value={holdValues.start_date}
            onChange={(e) => setHoldValues({ ...holdValues, start_date: e.target.value })}
            required
          />
        </label>
        <label>
          End date
          <input
            type="date"
            value={holdValues.end_date}
            onChange={(e) => setHoldValues({ ...holdValues, end_date: e.target.value })}
            required
          />
        </label>
        <label>
          Count
          <input
            type="number"
            min={1}
            max={100}
            step={1}
            value={holdValues.count}
            onChange={(e) => setHoldValues({ ...holdValues, count: e.target.value })}
            required
          />
        </label>
        <label>
          Sort
          <select
            value={holdValues.sort}
            onChange={(e) => setHoldValues({ ...holdValues, sort: e.target.value as 'oldest' | 'random' })}
          >
            <option value="oldest">Oldest first</option>
            <option value="random">Random</option>
          </select>
        </label>
        <label>
          Sex
          <select value={holdValues.sex} onChange={(e) => setHoldValues({ ...holdValues, sex: e.target.value })}>
            <option value="">Any</option>
            <option value="male">Male</option>
            <option value="female">Female</option>
          </select>
        </label>
        <button type="submit" disabled={holdMutation.isPending}>
          {holdMutation.isPending ? 'Holding…' : 'Hold children'}
        </button>
      </form>
      {holdError && (
        <p className="error" role="alert">
          {holdError}
        </p>
      )}

      <div className="tabs">
        {STATUS_TABS.map((t) => (
          <button
            key={t.key}
            type="button"
            className={statusTab === t.key ? 'active' : ''}
            onClick={() => setStatusTab(t.key)}
          >
            {t.label}
          </button>
        ))}
      </div>

      {isLoading && <p>Loading…</p>}
      {error && (
        <p className="error" role="alert">
          {errorMessage(error, 'Failed to load appointments.')}
        </p>
      )}
      {!isLoading && !error && (appointments ?? []).length === 0 && <p>No appointments in this status.</p>}
      {!isLoading && !error && (appointments ?? []).length > 0 && (
        <table>
          <caption>Appointments ({STATUS_TABS.find((t) => t.key === statusTab)?.label})</caption>
          <thead>
            <tr>
              <th></th>
              <th>Child</th>
              <th>Status</th>
              <th>Date</th>
              <th>Time</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {(appointments ?? []).map((a) => (
              <AppointmentRow key={a.id} appointment={a} experimentId={experimentId} labId={labId} />
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
