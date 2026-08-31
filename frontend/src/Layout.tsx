import { NavLink, Outlet, useNavigate, useParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { getLabs, logout, type User } from './api'

interface Props {
  user: User
}

// The shell every routed page renders inside: nav on the left, the lab
// selector + sign-out on the right, <Outlet/> for whatever route
// matched. Later frontend milestones add their own <NavLink> here as
// their routes land.
export default function Layout({ user }: Props) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  // Present (from the matched child route, e.g. /labs/:labId/setup) only
  // when currently on a lab-scoped page -- React Router merges child
  // route params up into every ancestor, Layout included.
  const { labId } = useParams<{ labId: string }>()
  // Fetched here (not just in LabPicker/LabSetup) so the selector can
  // render without waiting on whichever child route also needs it --
  // TanStack Query dedupes this against their own ['labs'] fetch.
  const { data: labs } = useQuery({ queryKey: ['labs'], queryFn: getLabs })

  async function handleLogout() {
    await logout()
    queryClient.removeQueries({ queryKey: ['me'] })
  }

  return (
    <div className="app">
      <header>
        <nav>
          <span className="app-name">CogLab Manager</span>
          <NavLink to="/app/participants">Participants</NavLink>
          <NavLink to="/app/labs">Lab Setup</NavLink>
        </nav>
        <div className="header-user">
          {labId && labs && labs.length > 1 && (
            <select
              value={labId}
              onChange={(e) => navigate(`/app/labs/${e.target.value}/setup`)}
              aria-label="Select lab"
            >
              {labs.map((lab) => (
                <option key={lab.id} value={lab.id}>
                  {lab.name}
                </option>
              ))}
            </select>
          )}
          <span>
            Signed in as {user.first_name} {user.last_name}
          </span>
          <button type="button" onClick={handleLogout}>
            Sign out
          </button>
        </div>
      </header>
      <main>
        <Outlet />
      </main>
    </div>
  )
}
