import { useState, type SubmitEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createUser,
  deactivateUser,
  errorMessage,
  getMe,
  listLabs,
  listRoles,
  listUsers,
  resendInvite,
  setPlatformAdmin,
  type CreateUserInput,
} from './api'

const usersQueryKey = ['admin-users']

export default function AdminUsers() {
  const queryClient = useQueryClient()
  const { data: users, isLoading, error } = useQuery({ queryKey: usersQueryKey, queryFn: listUsers })
  // ['me'] is already fetched (and cached) by App.tsx for the whole
  // session -- this dedupes against that fetch rather than a second
  // request, same pattern LabMembers.tsx uses. Needed only to hide the
  // deactivate/platform-admin actions on the caller's own row (the
  // server also refuses those, this just avoids offering a button that
  // would 400).
  const { data: me } = useQuery({ queryKey: ['me'], queryFn: getMe })
  // isLoading/isError matter here, not just `data`: the lab/role selects
  // below render (labs ?? []) / (roles ?? []), which is indistinguishable
  // from "this lab/role list is genuinely empty" while either query is
  // still loading or has failed -- without surfacing that separately, an
  // admin could submit the form believing "No lab" is the only option
  // when the picker actually just failed to load, silently creating a
  // lab-less account instead of the intended assignment.
  const { data: labs, isLoading: labsLoading, isError: labsError } = useQuery({ queryKey: ['labs-all'], queryFn: listLabs })
  const { data: roles, isLoading: rolesLoading, isError: rolesError } = useQuery({ queryKey: ['roles'], queryFn: listRoles })
  const labOrRolePickerUnavailable = labsLoading || labsError || rolesLoading || rolesError

  const [values, setValues] = useState<CreateUserInput>({
    email: '',
    first_name: '',
    last_name: '',
    is_platform_admin: false,
  })
  // Separate string state for the two optional selects (rather than
  // living directly on `values`) since an empty selection needs its own
  // representation ('') distinct from "no lab assigned" (undefined) --
  // only converted to CreateUserInput's lab_id/role_id on submit.
  const [labId, setLabId] = useState('')
  const [roleId, setRoleId] = useState('')
  // Same default-hidden "Show deactivated" convention LookupTable.tsx
  // uses -- most accounts here end up deactivated over time, and the
  // roster is unusable once that list is long.
  const [showDeactivated, setShowDeactivated] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)
  const [validationError, setValidationError] = useState<string | null>(null)

  const createMutation = useMutation({
    mutationFn: () =>
      createUser({
        ...values,
        lab_id: labId === '' ? undefined : Number(labId),
        role_id: roleId === '' ? undefined : Number(roleId),
      }),
    onSuccess: (result) => {
      setValues({ email: '', first_name: '', last_name: '', is_platform_admin: false })
      setLabId('')
      setRoleId('')
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

  const platformAdminMutation = useMutation({
    mutationFn: ({ userId, grant }: { userId: number; grant: boolean }) => setPlatformAdmin(userId, grant),
    onSuccess: () => {
      setNotice(null)
      void queryClient.invalidateQueries({ queryKey: usersQueryKey })
    },
    onError: (err) => setNotice(errorMessage(err, 'Failed to change platform-admin status.')),
  })

  function handleDeactivate(userId: number, email: string) {
    if (window.confirm(`Deactivate ${email}? They will be signed out immediately and won't be able to log in again.`)) {
      deactivateMutation.mutate(userId)
    }
  }

  function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    if ((labId === '') !== (roleId === '')) {
      setValidationError('Select both a lab and a permission role, or leave both blank.')
      return
    }
    setValidationError(null)
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
          {(users ?? []).filter((u) => showDeactivated || !u.deactivated).map((u) => (
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
                {u.id !== me?.user.id && (
                  <>
                    {!u.deactivated && (
                      <button
                        type="button"
                        onClick={() =>
                          platformAdminMutation.mutate({ userId: u.id, grant: !u.is_platform_admin })
                        }
                        disabled={platformAdminMutation.isPending}
                      >
                        {u.is_platform_admin ? 'Revoke platform admin' : 'Grant platform admin'}
                      </button>
                    )}
                    {!u.deactivated && (
                      <button
                        type="button"
                        onClick={() => handleDeactivate(u.id, u.email)}
                        disabled={deactivateMutation.isPending}
                      >
                        Deactivate
                      </button>
                    )}
                  </>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {(users ?? []).filter((u) => showDeactivated || !u.deactivated).length === 0 && (
        <p>No {showDeactivated ? '' : 'active '}users yet.</p>
      )}
      <label className="show-deactivated">
        <input type="checkbox" checked={showDeactivated} onChange={(e) => setShowDeactivated(e.target.checked)} />
        Show deactivated
      </label>

      <h3>Create user</h3>
      {validationError && (
        <p className="error" role="alert">
          {validationError}
        </p>
      )}
      {createMutation.isError && (
        <p className="error" role="alert">
          {errorMessage(createMutation.error, 'Failed to create user.')}
        </p>
      )}
      {(labsError || rolesError) && (
        <p className="error" role="alert">
          Failed to load the lab/permission-role picker. You can still create a user without a lab assignment, but
          assigning one isn't possible until this is fixed -- reload the page to retry.
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
        <label>
          Lab (optional)
          <select value={labId} onChange={(e) => setLabId(e.target.value)} disabled={labOrRolePickerUnavailable}>
            <option value="">{labsLoading ? 'Loading labs…' : labsError ? 'Failed to load labs' : 'No lab'}</option>
            {(labs ?? []).map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Permission role (optional)
          <select value={roleId} onChange={(e) => setRoleId(e.target.value)} disabled={labOrRolePickerUnavailable}>
            <option value="">{rolesLoading ? 'Loading roles…' : rolesError ? 'Failed to load roles' : 'No role'}</option>
            {(roles ?? []).map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
          </select>
        </label>
        <button type="submit" disabled={createMutation.isPending}>
          {createMutation.isPending ? 'Creating…' : 'Create user'}
        </button>
      </form>
      <p>The new user will receive an email with a link to set their password.</p>
    </div>
  )
}
