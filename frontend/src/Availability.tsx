import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import CreateDeleteList from './CreateDeleteList'
import MemberAvailability from './MemberAvailability'
import {
  getMe,
  listLabMemberships,
  createLabAvailabilityGeneral,
  createLabAvailabilitySpecific,
  deactivateLabAvailabilityGeneral,
  deactivateLabAvailabilitySpecific,
  listLabAvailabilityGeneral,
  listLabAvailabilitySpecific,
  type LabAvailabilityGeneral,
  type LabAvailabilitySpecific,
} from './api'

const WEEKDAY_OPTIONS = [
  { value: '0', label: 'Sunday' },
  { value: '1', label: 'Monday' },
  { value: '2', label: 'Tuesday' },
  { value: '3', label: 'Wednesday' },
  { value: '4', label: 'Thursday' },
  { value: '5', label: 'Friday' },
  { value: '6', label: 'Saturday' },
]

export default function Availability() {
  const { labId } = useParams<{ labId: string }>()
  const id = Number(labId)
  // '' means "myself" -- the self-service view every member gets.
  const [targetUserId, setTargetUserId] = useState('')

  const { data: me } = useQuery({ queryKey: ['me'], queryFn: getMe })
  const { data: memberships } = useQuery({
    queryKey: ['lab-memberships', id],
    queryFn: () => listLabMemberships(id),
  })

  // Coordinators and admins can also manage other members' availability
  // (the server enforces this; see requireLabCoordinatorOrAdminFromURL).
  // The same editor also lives in Lab Setup -> Members, but this page is
  // where a coordinator naturally looks for it.
  const isCoordinatorOrAdmin = (memberships ?? []).some(
    (m) => m.user_id === me?.user.id && (m.role_name === 'admin' || m.role_name === 'coordinator'),
  )
  const others = (memberships ?? [])
    .filter((m) => m.user_id !== me?.user.id)
    .sort((a, b) => `${a.last_name} ${a.first_name}`.localeCompare(`${b.last_name} ${b.first_name}`))

  const picker = isCoordinatorOrAdmin && (
    <label className="availability-member-picker">
      Whose availability
      <select value={targetUserId} onChange={(e) => setTargetUserId(e.target.value)}>
        <option value="">Myself</option>
        {others.map((m) => (
          <option key={m.user_id} value={String(m.user_id)}>
            {m.last_name}, {m.first_name}
          </option>
        ))}
      </select>
    </label>
  )

  if (isCoordinatorOrAdmin && targetUserId !== '') {
    return (
      <div className="availability">
        {picker}
        <MemberAvailability key={targetUserId} labId={id} userId={Number(targetUserId)} />
      </div>
    )
  }

  return (
    <div className="availability">
      {picker}
      <h2>General availability</h2>
      <p>Hours you're available every week, on a recurring basis.</p>
      <CreateDeleteList<LabAvailabilityGeneral>
        queryKey={['lab-availability-general', id]}
        fields={[
          { key: 'weekday', label: 'Day', type: 'select', options: WEEKDAY_OPTIONS },
          { key: 'start_time', label: 'Start time', type: 'time' },
          { key: 'end_time', label: 'End time', type: 'time' },
        ]}
        list={() => listLabAvailabilityGeneral(id)}
        create={(v) => createLabAvailabilityGeneral(id, Number(v.weekday), v.start_time, v.end_time)}
        remove={deactivateLabAvailabilityGeneral}
      />

      <h2>Specific-date availability</h2>
      <p>One-off hours for a particular date, on top of (or instead of) your general availability.</p>
      <CreateDeleteList<LabAvailabilitySpecific>
        queryKey={['lab-availability-specific', id]}
        fields={[
          { key: 'date', label: 'Date', type: 'date' },
          { key: 'start_time', label: 'Start time', type: 'time' },
          { key: 'end_time', label: 'End time', type: 'time' },
        ]}
        list={() => listLabAvailabilitySpecific(id)}
        create={(v) => createLabAvailabilitySpecific(id, v.date, v.start_time, v.end_time)}
        remove={deactivateLabAvailabilitySpecific}
      />
    </div>
  )
}
