import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { errorMessage, listExperiments } from './api'

const STATUS_LABELS: Record<string, string> = {
  not_run: 'Not run',
  pilot: 'Pilot',
  run: 'Run',
}

export default function Experiments() {
  const { labId } = useParams<{ labId: string }>()
  const id = Number(labId)
  const { data: experiments, isLoading, error } = useQuery({
    queryKey: ['experiments', id],
    queryFn: () => listExperiments(id),
  })

  if (isLoading) return <p>Loading…</p>
  if (error) {
    return (
      <p className="error" role="alert">
        {errorMessage(error, 'Failed to load experiments.')}
      </p>
    )
  }

  return (
    <div className="experiments">
      <div className="tabs">
        <Link to={`/app/labs/${id}/experiments/new`} className="add-family">
          Add experiment
        </Link>
      </div>
      {(experiments ?? []).length === 0 ? (
        <p>No experiments yet.</p>
      ) : (
        <table>
          <caption>Experiments in this lab</caption>
          <thead>
            <tr>
              <th>Name</th>
              <th>Status</th>
              <th>Sessions</th>
              <th>Start date</th>
              <th>End date</th>
            </tr>
          </thead>
          <tbody>
            {(experiments ?? []).map((e) => (
              <tr key={e.id}>
                <td>
                  <Link to={`/app/experiments/${e.id}`}>{e.name}</Link>
                </td>
                <td>{STATUS_LABELS[e.status] ?? e.status}</td>
                <td>{e.sessions}</td>
                <td>{e.start_date ?? '—'}</td>
                <td>{e.end_date ?? '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
