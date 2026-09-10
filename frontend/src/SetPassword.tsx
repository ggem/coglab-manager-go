import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { errorMessage, setPassword, type User } from './api'

interface Props {
  onLogin: (user: User) => void
}

// Public page (rendered by App.tsx before the ['me'] gate, same as
// LoginForm) that a person lands on from their account-creation invite
// email. Reads ?token= directly from the URL rather than through React
// Router, since App isn't wrapped in <Routes> at the point this renders
// -- matches LoginForm's own handling of ?sso_error=1.
export default function SetPassword({ onLogin }: Props) {
  const navigate = useNavigate()
  const token = new URLSearchParams(window.location.search).get('token') ?? ''
  const [password, setPasswordValue] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    if (password !== confirmPassword) {
      setError('Passwords do not match.')
      return
    }
    setSubmitting(true)
    try {
      const { user } = await setPassword(token, password)
      onLogin(user)
      // The URL is still /set-password?token=... at this point, which
      // no route in App.tsx's <Routes> matches -- without this, the
      // person would land on a blank page despite being successfully
      // logged in. "/" is what the root route Navigates from into
      // /app/participants, same landing spot a normal login gets.
      navigate('/', { replace: true })
    } catch (err) {
      setError(errorMessage(err, 'Failed to set password.'))
    } finally {
      setSubmitting(false)
    }
  }

  if (!token) {
    return (
      <div className="login-form">
        <h1>CogLab Manager</h1>
        <p className="error" role="alert">
          This link is missing its token. Check that you copied the full URL from your invite email.
        </p>
      </div>
    )
  }

  return (
    <form className="login-form" onSubmit={handleSubmit}>
      <h1>Set your password</h1>
      <label htmlFor="set-password-password">Password</label>
      <input
        id="set-password-password"
        type="password"
        value={password}
        onChange={(e) => setPasswordValue(e.target.value)}
        autoComplete="new-password"
        minLength={8}
        required
      />
      <label htmlFor="set-password-confirm">Confirm password</label>
      <input
        id="set-password-confirm"
        type="password"
        value={confirmPassword}
        onChange={(e) => setConfirmPassword(e.target.value)}
        autoComplete="new-password"
        minLength={8}
        required
      />
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <button type="submit" disabled={submitting}>
        {submitting ? 'Setting password…' : 'Set password'}
      </button>
    </form>
  )
}
