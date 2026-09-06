import { useParams } from 'react-router-dom'
import CreateDeleteList from './CreateDeleteList'
import {
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

  return (
    <div className="availability">
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
