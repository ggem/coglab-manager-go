import { useState, type SubmitEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createUser,
  deactivateUser,
  errorMessage,
  getMe,
  listUsers,
  resendInvite,
  type CreateUserInput,
} from './api'

const usersQueryKey = ['admin-users']

export default function AdminUsers() {
  const queryClient = useQueryClient()
  const { data: users, isLoading, error } = useQuery({ queryKey: usersQueryKey, queryFn: listUsers })
  // ['me'] is already fetched (and cached) by App.tsx for the whole
  // session -- this dedupes against that fetch rather than a second
  // request, same pattern LabMembers.tsx uses. Needed only to hide the
  // deactivate action on the caller's own row (the server also refuses
  // it, this just avoids offering a button that would 400).
  const { data: me } = useQuery({ queryKey: ['me'], queryFn: getMe })

  const [values, setValues] = useState<CreateUserInput>({
    email: '',
    first_name: '',
    last_name: '',
    is_platform_admin: false,
  })
  const [notice, setNotice] = useState<string | null>(null)

  const createMutation = useMutation({
    mutationFn: () => createUser(values),
    onSuccess: (result) => {
      setValues({ email: '', first_name: '', last_name: '', is_platform_admin: false })
      setNotice(
        result.invite_email_sent
          ? null
          : `${result.email} was created, but the invite email failed to send. Use "Resend invite" below once the problem is fixed.`,
      )
      void queryClient.invalidateQueries({ queryKey: usersQueryKey })
    },
  })

  const resendMutation = useMutation({
    mutationFn: (userId: number) => resendInvite(userId),
    onSuccess: (result, userId) => {
      setNotice(result.invite_email_sent ? null : `Resending the invite for user ${userId} failed again -- check the mail server.`)
    },
    onError: (err) => setNotice(errorMessage(err, 'Failed to resend invite.')),
  })

  const deactivateMutation = useMutation({
    mutationFn: (userId: number) => deactivateUser(userId),
    onSuccess: () => {
      setNotice(null)
      void queryClient.invalidateQueries({ queryKey: usersQueryKey })
    },
    onError: (err) => setNotice(errorMessage(err, 'Failed to deactivate user.')),
  })

  function handleDeactivate(userId: number, email: string) {
    if (window.confirm(`Deactivate ${email}? They will be signed out immediately and won't be able to log in again.`)) {
      deactivateMutation.mutate(userId)
    }
  }

  function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    createMutation.mutate()
  }

  if (isLoading) return <p>Loading…</p>
  if (error) {
    return (
      <p className="error" role="alert">
        {errorMessage(error, 'Failed to load users.')}
      </p>
    )
  }

  return (
    <div className="admin-users">
      <h2>Users</h2>
      {notice && <p role="status">{notice}</p>}
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Email</th>
            <th>Platform admin</th>
            <th>Status</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {(users ?? []).map((u) => (
            <tr key={u.id}>
              <td>
                {u.first_name} {u.last_name}
              </td>
              <td>{u.email}</td>
              <td>{u.is_platform_admin ? 'Yes' : ''}</td>
              <td>{u.deactivated ? 'Deactivated' : u.has_password ? 'Active' : 'Invite pending'}</td>
              <td className="lookup-table-actions">
                {!u.deactivated && !u.has_password && (
                  <button
                    type="button"
                    onClick={() => resendMutation.mutate(u.id)}
                    disabled={resendMutation.isPending}
                  >
                    Resend invite
                  </button>
                )}
                {u.id !== me?.user.id && !u.deactivated && (
                  <button
                    type="button"
                    onClick={() => handleDeactivate(u.id, u.email)}
                    disabled={deactivateMutation.isPending}
                  >
                    Deactivate
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {(users ?? []).length === 0 && <p>No users yet.</p>}

      <h3>Create user</h3>
      {createMutation.isError && (
        <p className="error" role="alert">
          {errorMessage(createMutation.error, 'Failed to create user.')}
        </p>
      )}
      <form onSubmit={handleSubmit}>
        <label>
          Email
          <input
            type="email"
            value={values.email}
            onChange={(e) => setValues({ ...values, email: e.target.value })}
            required
          />
        </label>
        <label>
          First name
          <input
            value={values.first_name}
            onChange={(e) => setValues({ ...values, first_name: e.target.value })}
            required
          />
        </label>
        <label>
          Last name
          <input
            value={values.last_name}
            onChange={(e) => setValues({ ...values, last_name: e.target.value })}
            required
          />
        </label>
        <label>
          <input
            type="checkbox"
            checked={values.is_platform_admin}
            onChange={(e) => setValues({ ...values, is_platform_admin: e.target.checked })}
          />
          Grant platform admin
        </label>
        <button type="submit" disabled={createMutation.isPending}>
          {createMutation.isPending ? 'Creating…' : 'Create user'}
        </button>
      </form>
      <p>The new user will receive an email with a link to set their password.</p>
    </div>
  )
}
