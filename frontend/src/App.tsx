import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Navigate, Route, Routes } from 'react-router-dom'
import LoginForm from './LoginForm'
import SetPassword from './SetPassword'
import Layout from './Layout'
import ParticipantSearch from './ParticipantSearch'
import LabPicker from './LabPicker'
import LabSetup from './LabSetup'
import CreateFamily from './CreateFamily'
import FamilyDetail from './FamilyDetail'
import Experiments from './Experiments'
import CreateExperiment from './CreateExperiment'
import ExperimentDetail from './ExperimentDetail'
import Availability from './Availability'
import Reports from './Reports'
import AdminUsers from './AdminUsers'
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
    const onLogin = (user: User) => queryClient.setQueryData(['me'], { user })
    // /set-password is reachable while logged out (it's how an
    // invite link lands) -- checked directly against window.location
    // rather than through <Routes>, since nothing below here is
    // wrapped in a router match until after login, same as LoginForm's
    // own ?sso_error=1 handling.
    if (window.location.pathname === '/set-password') {
      return <SetPassword onLogin={onLogin} />
    }
    return <LoginForm onLogin={onLogin} />
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
        <Route path="labs" element={<LabPicker buildPath={(id) => `/app/labs/${id}/setup`} />} />
        <Route path="labs/:labId/setup" element={<LabSetup />} />
        <Route path="experiments" element={<LabPicker buildPath={(id) => `/app/labs/${id}/experiments`} />} />
        <Route path="labs/:labId/experiments" element={<Experiments />} />
        <Route path="labs/:labId/experiments/new" element={<CreateExperiment />} />
        <Route path="experiments/:experimentId" element={<ExperimentDetail />} />
        <Route path="availability" element={<LabPicker buildPath={(id) => `/app/labs/${id}/availability`} />} />
        <Route path="labs/:labId/availability" element={<Availability />} />
        <Route path="reports" element={<LabPicker buildPath={(id) => `/app/labs/${id}/reports`} />} />
        <Route path="labs/:labId/reports" element={<Reports />} />
        <Route path="families" element={<Navigate to="/app/participants" replace />} />
        <Route path="families/new" element={<CreateFamily />} />
        <Route path="families/:familyId" element={<FamilyDetail />} />
        <Route path="admin/users" element={<AdminUsers />} />
      </Route>
    </Routes>
  )
}

export default App
