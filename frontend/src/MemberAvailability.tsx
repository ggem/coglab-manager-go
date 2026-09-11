import CreateDeleteList from './CreateDeleteList'
import {
  createLabAvailabilityGeneralForUser,
  createLabAvailabilitySpecificForUser,
  deactivateLabAvailabilityGeneral,
  deactivateLabAvailabilitySpecific,
  listLabAvailabilityGeneralForUser,
  listLabAvailabilitySpecificForUser,
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

interface MemberAvailabilityProps {
  labId: number
  userId: number
}

// A parameterized copy of Availability.tsx's body, for a coordinator or
// admin managing another member's schedule instead of their own --
// duplicated deliberately rather than sharing an abstraction with
// Availability.tsx, since the two differ only in which API functions they
// wire up (self-service vs. on-behalf-of). Deactivation reuses the same
// functions Availability.tsx uses: that's keyed by row ID, and the
// coordinator/admin relaxation happens entirely server-side.
export default function MemberAvailability({ labId, userId }: MemberAvailabilityProps) {
  return (
    <div className="availability">
      <h3>General availability</h3>
      <CreateDeleteList<LabAvailabilityGeneral>
        queryKey={['lab-availability-general-for-user', labId, userId]}
        fields={[
          { key: 'weekday', label: 'Day', type: 'select', options: WEEKDAY_OPTIONS },
          { key: 'start_time', label: 'Start time', type: 'time' },
          { key: 'end_time', label: 'End time', type: 'time' },
        ]}
        list={() => listLabAvailabilityGeneralForUser(labId, userId)}
        create={(v) => createLabAvailabilityGeneralForUser(labId, userId, Number(v.weekday), v.start_time, v.end_time)}
        remove={deactivateLabAvailabilityGeneral}
      />

      <h3>Specific-date availability</h3>
      <CreateDeleteList<LabAvailabilitySpecific>
        queryKey={['lab-availability-specific-for-user', labId, userId]}
        fields={[
          { key: 'date', label: 'Date', type: 'date' },
          { key: 'start_time', label: 'Start time', type: 'time' },
          { key: 'end_time', label: 'End time', type: 'time' },
        ]}
        list={() => listLabAvailabilitySpecificForUser(labId, userId)}
        create={(v) => createLabAvailabilitySpecificForUser(labId, userId, v.date, v.start_time, v.end_time)}
        remove={deactivateLabAvailabilitySpecific}
      />
    </div>
  )
}
