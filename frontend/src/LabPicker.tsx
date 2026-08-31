import { useQuery } from '@tanstack/react-query'
import { Navigate, Link } from 'react-router-dom'
import { errorMessage, getLabs } from './api'

// The /labs index route: with exactly one lab (the common case), skips
// straight to its setup page: with more than one, a plain list to pick
// from; with none, a message rather than a broken/empty setup screen.
export default function LabPicker() {
  const { data: labs, isLoading, error } = useQuery({ queryKey: ['labs'], queryFn: getLabs })

  if (isLoading) return <p>Loading…</p>
  if (error) {
    return (
      <p className="error" role="alert">
        {errorMessage(error, 'Failed to load labs.')}
      </p>
    )
  }
  if (!labs || labs.length === 0) {
    return <p>You're not a member of any lab yet.</p>
  }
  if (labs.length === 1) {
    return <Navigate to={`/app/labs/${labs[0].id}/setup`} replace />
  }

  return (
    <div className="lab-picker">
      <h2>Choose a lab</h2>
      <ul>
        {labs.map((lab) => (
          <li key={lab.id}>
            <Link to={`/app/labs/${lab.id}/setup`}>{lab.name}</Link>
          </li>
        ))}
      </ul>
    </div>
  )
}
