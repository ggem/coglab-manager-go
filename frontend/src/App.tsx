import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Navigate, Route, Routes } from 'react-router-dom'
import LoginForm from './LoginForm'
import Layout from './Layout'
import ParticipantSearch from './ParticipantSearch'
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
      <Route element={<Layout user={data.user} />}>
        <Route index element={<Navigate to="/participants" replace />} />
        <Route path="participants" element={<ParticipantSearch />} />
      </Route>
    </Routes>
  )
}

export default App
