import { NavLink, Outlet } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { logout, type User } from './api'

interface Props {
  user: User
}

// The shell every routed page renders inside: nav on the left, sign-out
// on the right, <Outlet/> for whatever route matched. Later frontend
// milestones add their own <NavLink> here as their routes land.
export default function Layout({ user }: Props) {
  const queryClient = useQueryClient()

  async function handleLogout() {
    await logout()
    queryClient.removeQueries({ queryKey: ['me'] })
  }

  return (
    <div className="app">
      <header>
        <nav>
          <span className="app-name">CogLab Manager</span>
          <NavLink to="/participants">Participants</NavLink>
        </nav>
        <div className="header-user">
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
