import { useQuery } from '@tanstack/react-query'
import { Navigate, Link } from 'react-router-dom'
import { errorMessage, getLabs } from './api'

interface Props {
  // Where to send the user once a lab is picked or auto-selected --
  // e.g. `(id) => \`/app/labs/${id}/setup\`` for Lab Setup,
  // `(id) => \`/app/labs/${id}/experiments\`` for Experiments. Lets
  // this one component serve as the entry point for every lab-scoped
  // section rather than duplicating the "redirect straight through if
  // there's only one lab, otherwise show a picker" logic per section.
  buildPath: (labId: number) => string
}

// A lab-scoped section's index route: with exactly one lab (the common
// case), skips straight to buildPath(labId); with more than one, a
// plain list to pick from; with none, a message rather than a
// broken/empty screen.
export default function LabPicker({ buildPath }: Props) {
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
    return <Navigate to={buildPath(labs[0].id)} replace />
  }

  return (
    <div className="lab-picker">
      <h2>Choose a lab</h2>
      <ul>
        {labs.map((lab) => (
          <li key={lab.id}>
            <Link to={buildPath(lab.id)}>{lab.name}</Link>
          </li>
        ))}
      </ul>
    </div>
  )
}
