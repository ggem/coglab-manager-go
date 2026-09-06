import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import AppointmentCallLog from './AppointmentCallLog'
import AppointmentScheduleFlow from './AppointmentScheduleFlow'
import {
  arriveAppointment,
  errorMessage,
  getChild,
  getChildAppointmentHistory,
  getFamily,
  listAppointmentExperimenters,
  listGuardiansByFamily,
  releaseAppointment,
  type Appointment,
} from './api'

interface Props {
  appointment: Appointment
  experimentId: number
  labId: number
}

export default function AppointmentRow({ appointment, experimentId, labId }: Props) {
  const queryClient = useQueryClient()
  const [expanded, setExpanded] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  // The child's name is shown whether or not the row is expanded, so
  // this fetch always runs; everything else below is gated on
  // `expanded` -- no reason to fetch a held child's whole file for a
  // row nobody's opened.
  const { data: child } = useQuery({ queryKey: ['child', appointment.child_id], queryFn: () => getChild(appointment.child_id) })

  const { data: family } = useQuery({
    queryKey: ['family', child?.family_id],
    queryFn: () => getFamily(child!.family_id),
    enabled: expanded && !!child,
  })
  const { data: guardians } = useQuery({
    queryKey: ['guardians', child?.family_id],
    queryFn: () => listGuardiansByFamily(child!.family_id),
    enabled: expanded && !!child,
  })
  const { data: history } = useQuery({
    queryKey: ['appointment-history', appointment.child_id],
    queryFn: () => getChildAppointmentHistory(appointment.child_id),
    enabled: expanded,
  })
  const { data: experimenters } = useQuery({
    queryKey: ['appointment-experimenters', appointment.id],
    queryFn: () => listAppointmentExperimenters(appointment.id),
    enabled: expanded && appointment.status !== 'to_be_scheduled',
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['appointments', experimentId] })

  const releaseMutation = useMutation({
    mutationFn: () => releaseAppointment(appointment.id),
    onSuccess: () => {
      setActionError(null)
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to release.')),
  })
  const arriveMutation = useMutation({
    mutationFn: () => arriveAppointment(appointment.id),
    onSuccess: () => {
      setActionError(null)
      void invalidate()
    },
    onError: (err) => setActionError(errorMessage(err, 'Failed to mark arrived.')),
  })

  function handleArrive() {
    if (window.confirm("Mark this appointment arrived? This can't be undone from here.")) {
      arriveMutation.mutate()
    }
  }

  return (
    <>
      <tr>
        <td>
          <button
            type="button"
            className="expand-toggle"
            onClick={() => setExpanded(!expanded)}
            aria-expanded={expanded}
          >
            {expanded ? '▾' : '▸'}
          </button>
        </td>
        <td>{child ? `${child.first_name} ${child.last_name}` : '…'}</td>
        <td>{appointment.status}</td>
        <td>{appointment.schedule_date ?? '—'}</td>
        <td>{appointment.schedule_time_start ?? '—'}</td>
        <td className="lookup-table-actions">
          {appointment.status === 'pending' && (
            <>
              <button type="button" onClick={() => releaseMutation.mutate()} disabled={releaseMutation.isPending}>
                Release
              </button>
              <button type="button" onClick={handleArrive} disabled={arriveMutation.isPending}>
                Arrive
              </button>
            </>
          )}
          {appointment.status === 'to_be_scheduled' && (
            <button type="button" onClick={() => releaseMutation.mutate()} disabled={releaseMutation.isPending}>
              Release
            </button>
          )}
        </td>
      </tr>
      {actionError && (
        <tr>
          <td colSpan={6}>
            <p className="error" role="alert">
              {actionError}
            </p>
          </td>
        </tr>
      )}
      {expanded && (
        <tr className="expanded-row">
          <td colSpan={6}>
            <div className="appointment-detail">
              {child && (
                <div>
                  <h4>Child</h4>
                  <p>
                    {child.first_name} {child.last_name} -- {child.sex}, born {child.birth_date ?? 'unknown'}
                  </p>
                  {child.birth_complications && <p>Birth complications: {child.birth_complications_notes}</p>}
                  {child.twin && <p>Twin</p>}
                  {child.languages.length > 0 && <p>Languages: {child.languages.join(', ')}</p>}
                </div>
              )}
              {family && (
                <div>
                  <h4>Family</h4>
                  <p>
                    {family.address}, {family.city}, {family.state} {family.zip}
                  </p>
                  <ul>
                    {(guardians ?? []).map((g) => (
                      <li key={g.id}>
                        {g.first_name} {g.last_name} -- {g.phone_number}
                        {g.email ? `, ${g.email}` : ''}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              {history && (
                <div>
                  <h4>Previous studies</h4>
                  {history.own.length === 0 && history.siblings.length === 0 && <p>None.</p>}
                  <ul>
                    {history.own.map((h) => (
                      <li key={h.appointment_id}>
                        {h.experiment_name} ({h.status}
                        {h.schedule_date ? `, ${h.schedule_date}` : ''})
                      </li>
                    ))}
                    {history.siblings.map((h) => (
                      <li key={h.appointment_id}>
                        {h.child_first_name} {h.child_last_name}: {h.experiment_name} ({h.status}
                        {h.schedule_date ? `, ${h.schedule_date}` : ''})
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              {appointment.status !== 'to_be_scheduled' && experimenters && experimenters.length > 0 && (
                <div>
                  <h4>Assigned staff</h4>
                  <ul>
                    {experimenters.map((e) => (
                      <li key={e.user_id}>
                        {e.role_name}: {e.first_name} {e.last_name}
                        {e.is_greeter ? ' (greeter)' : ''}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              <AppointmentCallLog appointmentId={appointment.id} />
              {appointment.status === 'to_be_scheduled' && (
                <AppointmentScheduleFlow
                  appointmentId={appointment.id}
                  experimentId={experimentId}
                  labId={labId}
                  onScheduled={() => setExpanded(false)}
                />
              )}
            </div>
          </td>
        </tr>
      )}
    </>
  )
}
