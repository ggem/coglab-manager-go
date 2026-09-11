import { Fragment, useState, type SubmitEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import AttachList from './AttachList'
import MemberAvailability from './MemberAvailability'
import {
  addLabMemberTraining,
  createLabMembership,
  createLabMembershipForNewUser,
  errorMessage,
  getMe,
  listExperimentRoles,
  listLabMemberTrainingsForUser,
  listLabMemberships,
  listRoles,
  removeLabMembership,
  removeLabMemberTraining,
  searchUsersNotInLab,
  updateLabMembership,
  type LabMembership,
  type SearchedUser,
} from './api'
import { PRIORITY_LABELS, PRIORITY_OPTIONS } from './priorityOptions'

// A bespoke component rather than a forced fit into LookupTable (no
// deactivated_at concept here -- removing a member is a real delete,
// not a soft one) or CreateDeleteList (adding a member needs a search
// step, not a bare create form). The nested per-member trainings list
// reuses AttachList as-is, wired exactly like ExperimentDetail.tsx's
// five existing AttachList instances.

interface Props {
  labId: number
}

// The non-admin counterpart to the AttachList below: assigning/
// unassigning a training now requires lab-admin
// (requireLabAdminForExperimentRole on the server), so a non-admin gets
// this plain, uneditable list instead of live add/remove controls --
// otherwise the "read-only view" for non-admins would be incomplete,
// showing member-management controls hidden but training-management
// controls still live.
function MemberTrainingsReadOnly({ labId, userId }: { labId: number; userId: number }) {
  const { data: trainings } = useQuery({
    queryKey: ['lab-member-trainings', labId, userId],
    queryFn: () => listLabMemberTrainingsForUser(labId, userId),
  })
  if (!trainings || trainings.length === 0) return <p>None attached yet.</p>
  return (
    <ul>
      {trainings.map((r) => (
        <li key={r.id}>{r.name}</li>
      ))}
    </ul>
  )
}

export default function LabMembers({ labId }: Props) {
  const queryClient = useQueryClient()
  const membershipsQueryKey = ['lab-memberships', labId]
  const { data: memberships, isLoading, error } = useQuery({
    queryKey: membershipsQueryKey,
    queryFn: () => listLabMemberships(labId),
  })
  const { data: roles } = useQuery({ queryKey: ['roles'], queryFn: listRoles })
  // ['me'] is already fetched (and cached) by App.tsx for the whole
  // session -- this dedupes against that fetch rather than threading the
  // current user down through Layout/Outlet, which isn't wired today.
  const { data: me } = useQuery({ queryKey: ['me'], queryFn: getMe })

  // Mirrors the server-side admin gate (requireLabAdminFromURL) so
  // non-admins see the same read-only view the server would actually
  // allow, rather than clickable controls that 403 -- the server check
  // is what actually matters, this is just not showing a false
  // affordance. Derived from the roster itself (the caller is always in
  // it, since viewing this page already required at least plain
  // membership) rather than a second request.
  const isAdmin = (memberships ?? []).some((m) => m.user_id === me?.user.id && m.role_name === 'admin')

  // Separate from isAdmin: a coordinator's whole job is scheduling, so
  // they can manage another member's availability without holding the
  // full admin role that member-management (edit/remove, trainings)
  // still requires.
  const isCoordinatorOrAdmin = (memberships ?? []).some(
    (m) => m.user_id === me?.user.id && (m.role_name === 'admin' || m.role_name === 'coordinator'),
  )

  const [expandedUserId, setExpandedUserId] = useState<number | null>(null)
  const [editingUserId, setEditingUserId] = useState<number | null>(null)
  const [editRoleId, setEditRoleId] = useState('')
  const [editPriority, setEditPriority] = useState('')
  const [actionError, setActionError] = useState<string | null>(null)

  const [query, setQuery] = useState('')
  const [addRoleId, setAddRoleId] = useState('')
  const [searchResults, setSearchResults] = useState<SearchedUser[] | null>(null)
  const [searching, setSearching] = useState(false)
  const [showCreateUser, setShowCreateUser] = useState(false)
  const [newUserEmail, setNewUserEmail] = useState('')
  const [newUserFirstName, setNewUserFirstName] = useState('')
  const [newUserLastName, setNewUserLastName] = useState('')

  const invalidate = () => queryClient.invalidateQueries({ queryKey: membershipsQueryKey })

  const addMutation = useMutation({
    mutationFn: (userId: number) => createLabMembership(labId, userId, Number(addRoleId)),
    onSuccess: () => {
      setActionError(null)
      setSearchResults(null)
      setQuery('')
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to add member.')),
  })

  const createUserMutation = useMutation({
    mutationFn: () =>
      createLabMembershipForNewUser(labId, {
        email: newUserEmail,
        first_name: newUserFirstName,
        last_name: newUserLastName,
        role_id: Number(addRoleId),
      }),
    onSuccess: (result) => {
      setActionError(
        result.invite_email_sent
          ? null
          : `${result.email} was added, but the invite email failed to send. A platform admin can resend it from the Users page.`,
      )
      setSearchResults(null)
      setQuery('')
      setShowCreateUser(false)
      setNewUserEmail('')
      setNewUserFirstName('')
      setNewUserLastName('')
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to create user.')),
  })

  const updateMutation = useMutation({
    mutationFn: ({ userId, roleId, priority }: { userId: number; roleId: number; priority: string }) =>
      updateLabMembership(labId, userId, roleId, priority),
    onSuccess: () => {
      setEditingUserId(null)
      setActionError(null)
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to save.')),
  })

  const removeMutation = useMutation({
    mutationFn: (userId: number) => removeLabMembership(labId, userId),
    onSuccess: () => {
      setActionError(null)
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to remove.')),
  })

  function startEdit(m: LabMembership) {
    setEditingUserId(m.user_id)
    setEditRoleId(String(m.role_id))
    setEditPriority(m.priority)
  }

  async function handleSearch(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    setActionError(null)
    setSearching(true)
    try {
      setSearchResults(await searchUsersNotInLab(labId, query))
    } catch (err) {
      setActionError(errorMessage(err, 'Search failed.'))
    } finally {
      setSearching(false)
    }
  }

  if (isLoading) return <p>Loading…</p>
  if (error) {
    return (
      <p className="error" role="alert">
        {errorMessage(error, 'Failed to load members.')}
      </p>
    )
  }

  const rows = memberships ?? []

  return (
    <div className="lab-members">
      {actionError && (
        <p className="error" role="alert">
          {actionError}
        </p>
      )}
      <table>
        <thead>
          <tr>
            <th></th>
            <th>Name</th>
            <th>Email</th>
            <th>Permission role</th>
            <th>Scheduling priority</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((m) => (
            <Fragment key={m.user_id}>
              <tr>
                <td>
                  <button
                    type="button"
                    className="expand-toggle"
                    onClick={() => setExpandedUserId(expandedUserId === m.user_id ? null : m.user_id)}
                    aria-expanded={expandedUserId === m.user_id}
                    aria-controls={`member-trainings-${m.user_id}`}
                    aria-label={`Show training roles for ${m.first_name} ${m.last_name}`}
                  >
                    {expandedUserId === m.user_id ? '▾' : '▸'}
                  </button>
                </td>
                <td>
                  {m.first_name} {m.last_name}
                </td>
                <td>{m.email}</td>
                <td>
                  {editingUserId === m.user_id ? (
                    <select value={editRoleId} onChange={(e) => setEditRoleId(e.target.value)}>
                      {(roles ?? []).map((r) => (
                        <option key={r.id} value={r.id}>
                          {r.name}
                        </option>
                      ))}
                    </select>
                  ) : (
                    m.role_name
                  )}
                </td>
                <td>
                  {editingUserId === m.user_id ? (
                    <select value={editPriority} onChange={(e) => setEditPriority(e.target.value)}>
                      {PRIORITY_OPTIONS.map((o) => (
                        <option key={o.value} value={o.value}>
                          {o.label}
                        </option>
                      ))}
                    </select>
                  ) : (
                    (PRIORITY_LABELS[m.priority] ?? m.priority)
                  )}
                </td>
                <td className="lookup-table-actions">
                  {editingUserId === m.user_id ? (
                    <>
                      <button
                        type="button"
                        onClick={() =>
                          updateMutation.mutate({ userId: m.user_id, roleId: Number(editRoleId), priority: editPriority })
                        }
                        disabled={updateMutation.isPending}
                      >
                        Save
                      </button>
                      <button type="button" onClick={() => setEditingUserId(null)}>
                        Cancel
                      </button>
                    </>
                  ) : (
                    isAdmin && (
                      <>
                        <button type="button" onClick={() => startEdit(m)}>
                          Edit
                        </button>
                        <button
                          type="button"
                          onClick={() => removeMutation.mutate(m.user_id)}
                          disabled={removeMutation.isPending}
                        >
                          Remove
                        </button>
                      </>
                    )
                  )}
                </td>
              </tr>
              {expandedUserId === m.user_id && (
                <tr className="expanded-row">
                  <td colSpan={6} id={`member-trainings-${m.user_id}`}>
                    {isAdmin ? (
                      <AttachList
                        queryKey={['lab-member-trainings', labId, m.user_id]}
                        list={() => listLabMemberTrainingsForUser(labId, m.user_id)}
                        options={() => listExperimentRoles(labId)}
                        add={(roleId) => addLabMemberTraining(roleId, m.user_id)}
                        remove={(roleId) => removeLabMemberTraining(roleId, m.user_id)}
                        label={(r) => r.name}
                        addLabel="Add a trained role…"
                      />
                    ) : (
                      <MemberTrainingsReadOnly labId={labId} userId={m.user_id} />
                    )}
                    {isCoordinatorOrAdmin && <MemberAvailability labId={labId} userId={m.user_id} />}
                  </td>
                </tr>
              )}
            </Fragment>
          ))}
        </tbody>
      </table>
      {rows.length === 0 && <p>No members yet.</p>}

      {isAdmin ? (
        <>
          <h4>Add an existing person to this lab</h4>
          <form onSubmit={handleSearch} className="lab-members-search">
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search by name…"
            />
            <button type="submit" disabled={searching || query.trim() === ''}>
              {searching ? 'Searching…' : 'Search'}
            </button>
          </form>
          {searchResults !== null && (
            <label className="lab-members-role-picker">
              Permission role for the person you add
              <select value={addRoleId} onChange={(e) => setAddRoleId(e.target.value)} aria-label="Permission role">
                <option value="" disabled>
                  Permission role
                </option>
                {(roles ?? []).map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name}
                  </option>
                ))}
              </select>
            </label>
          )}
          {searchResults && (
            <ul className="lab-members-search-results">
              {searchResults.length === 0 && (
                <li>
                  No matches.{' '}
                  {!showCreateUser && (
                    <button type="button" onClick={() => setShowCreateUser(true)}>
                      Create new user
                    </button>
                  )}
                </li>
              )}
              {searchResults.map((u) => (
                <li key={u.id}>
                  {u.first_name} {u.last_name} ({u.email})
                  <button
                    type="button"
                    onClick={() => addMutation.mutate(u.id)}
                    disabled={addMutation.isPending || addRoleId === ''}
                  >
                    Add
                  </button>
                </li>
              ))}
            </ul>
          )}
          {showCreateUser && (
            <form
              className="lab-members-create-user"
              onSubmit={(e) => {
                e.preventDefault()
                createUserMutation.mutate()
              }}
            >
              <h5>Create a new user and add them to this lab</h5>
              <label>
                Email
                <input
                  type="email"
                  value={newUserEmail}
                  onChange={(e) => setNewUserEmail(e.target.value)}
                  required
                />
              </label>
              <label>
                First name
                <input value={newUserFirstName} onChange={(e) => setNewUserFirstName(e.target.value)} required />
              </label>
              <label>
                Last name
                <input value={newUserLastName} onChange={(e) => setNewUserLastName(e.target.value)} required />
              </label>
              <button type="submit" disabled={createUserMutation.isPending || addRoleId === ''}>
                {createUserMutation.isPending ? 'Creating…' : 'Create and add'}
              </button>
              <button type="button" onClick={() => setShowCreateUser(false)}>
                Cancel
              </button>
            </form>
          )}
        </>
      ) : (
        <p>Only this lab's admins can add, edit, or remove members, or change their trained roles.</p>
      )}
    </div>
  )
}
