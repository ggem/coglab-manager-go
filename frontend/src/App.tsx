import { useState } from 'react'
import LoginForm from './LoginForm'
import ParticipantSearch from './ParticipantSearch'
import { logout, type User } from './api'
import './App.css'

// There's no "check for an existing session" endpoint yet, so login state
// lives only in memory -- a page refresh always requires signing in again.
// Revisit once there's a real need to persist across reloads.
function App() {
  const [user, setUser] = useState<User | null>(null)

  async function handleLogout() {
    await logout()
    setUser(null)
  }

  if (!user) {
    return <LoginForm onLogin={setUser} />
  }

  return (
    <div className="app">
      <header>
        <span>
          Signed in as {user.first_name} {user.last_name}
        </span>
        <button type="button" onClick={handleLogout}>
          Sign out
        </button>
      </header>
      <ParticipantSearch />
    </div>
  )
}

export default App
