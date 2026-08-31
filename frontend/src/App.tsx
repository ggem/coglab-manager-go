import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Navigate, Route, Routes } from 'react-router-dom'
import LoginForm from './LoginForm'
import Layout from './Layout'
import ParticipantSearch from './ParticipantSearch'
import LabPicker from './LabPicker'
import LabSetup from './LabSetup'
import { getMe, type User } from './api'
import './App.css'

function App() {
  const queryClient = useQueryClient()
  // retry: false -- a 401 here just means "not logged in," not a
  // transient failure worth retrying.
  const { data, isLoading } = useQuery({
    queryKey: ['me'],
    queryFn: getMe,
    retry: false,
  })

  if (isLoading) {
    return <p className="loading">Loading…</p>
  }

  if (!data) {
    return (
      <LoginForm onLogin={(user: User) => queryClient.setQueryData(['me'], { user })} />
    )
  }

  return (
    <Routes>
      {/* Every client-side route lives under /app -- the backend owns
          every other short top-level path (/labs, /conditions, /equipment,
          ...; see vite.config.ts's apiPaths), so a bare /labs route here
          would collide with the real GET /labs API endpoint instead of
          rendering this app's own page. One shared prefix avoids having
          to re-check that collision for every route future milestones
          add. */}
      <Route path="/" element={<Navigate to="/app/participants" replace />} />
      <Route path="/app" element={<Layout user={data.user} />}>
        <Route index element={<Navigate to="/app/participants" replace />} />
        <Route path="participants" element={<ParticipantSearch />} />
        <Route path="labs" element={<LabPicker />} />
        <Route path="labs/:labId/setup" element={<LabSetup />} />
      </Route>
    </Routes>
  )
}

export default App
