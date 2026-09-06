import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import ExperimentForm from './ExperimentForm'
import AttachList from './AttachList'
import AppointmentsPanel from './AppointmentsPanel'
import {
  addExperimentCondition,
  addExperimentEquipment,
  addExperimentGrant,
  addExperimentPrincipalInvestigator,
  addExperimentTrainingRequirement,
  deactivateExperiment,
  errorMessage,
  getExperiment,
  getLabMembers,
  listConditions,
  listEquipment,
  listExperimentConditions,
  listExperimentEquipment,
  listExperimentGrants,
  listExperimentPrincipalInvestigators,
  listExperimentRoles,
  listExperimentTrainingRequirements,
  listGrants,
  removeExperimentCondition,
  removeExperimentEquipment,
  removeExperimentGrant,
  removeExperimentPrincipalInvestigator,
  removeExperimentTrainingRequirement,
  updateExperiment,
  type ExperimentInput,
} from './api'

export default function ExperimentDetail() {
  const { experimentId } = useParams<{ experimentId: string }>()
  const id = Number(experimentId)
  const queryClient = useQueryClient()

  const {
    data: experiment,
    isLoading,
    error,
  } = useQuery({ queryKey: ['experiment', id], queryFn: () => getExperiment(id) })

  const updateMutation = useMutation({
    mutationFn: (input: ExperimentInput) => updateExperiment(id, input),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['experiment', id] }),
  })

  const deactivateMutation = useMutation({
    mutationFn: () => deactivateExperiment(id),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['experiment', id] }),
  })

  function handleDeactivate() {
    if (!experiment) return
    if (window.confirm(`Deactivate "${experiment.name}"? This can't be undone from here.`)) {
      deactivateMutation.mutate()
    }
  }

  if (isLoading) return <p>Loading…</p>
  if (error) {
    return (
      <p className="error" role="alert">
        {errorMessage(error, 'Failed to load experiment.')}
      </p>
    )
  }
  if (!experiment) return null

  const labId = experiment.lab_id

  return (
    <div className="experiment-detail">
      <div className="experiment-detail-header">
        <h2>{experiment.name}</h2>
        {!experiment.deactivated && (
          <button type="button" onClick={handleDeactivate} disabled={deactivateMutation.isPending}>
            Deactivate
          </button>
        )}
        {experiment.deactivated && <span>Deactivated</span>}
      </div>
      {deactivateMutation.isError && (
        <p className="error" role="alert">
          {errorMessage(deactivateMutation.error, 'Failed to deactivate.')}
        </p>
      )}
      <ExperimentForm
        key={experiment.id}
        mode="edit"
        labId={labId}
        initial={experiment}
        onSubmit={(input) => updateMutation.mutate(input)}
        submitting={updateMutation.isPending}
        submitLabel="Save"
        error={updateMutation.isError ? errorMessage(updateMutation.error, 'Failed to save.') : null}
      />

      <h3>Conditions</h3>
      <AttachList
        queryKey={['experiment-conditions', id]}
        list={() => listExperimentConditions(id)}
        options={() => listConditions(labId)}
        add={(conditionId) => addExperimentCondition(id, conditionId)}
        remove={(conditionId) => removeExperimentCondition(id, conditionId)}
        label={(c) => c.name}
        addLabel="Add a condition…"
      />

      <h3>Equipment</h3>
      <AttachList
        queryKey={['experiment-equipment', id]}
        list={() => listExperimentEquipment(id)}
        options={() => listEquipment(labId)}
        add={(equipmentId) => addExperimentEquipment(id, equipmentId)}
        remove={(equipmentId) => removeExperimentEquipment(id, equipmentId)}
        label={(e) => e.name}
        addLabel="Add equipment…"
      />

      <h3>Training requirements</h3>
      <AttachList
        queryKey={['experiment-training-requirements', id]}
        list={() => listExperimentTrainingRequirements(id)}
        options={() => listExperimentRoles(labId)}
        add={(roleId) => addExperimentTrainingRequirement(id, roleId)}
        remove={(roleId) => removeExperimentTrainingRequirement(id, roleId)}
        label={(r) => r.name}
        addLabel="Add a training requirement…"
      />

      <h3>Grants</h3>
      <AttachList
        queryKey={['experiment-grants', id]}
        list={() => listExperimentGrants(id)}
        options={() => listGrants(labId)}
        add={(grantId) => addExperimentGrant(id, grantId)}
        remove={(grantId) => removeExperimentGrant(id, grantId)}
        label={(g) => g.name}
        addLabel="Add a grant…"
      />

      <h3>Principal investigators</h3>
      <AttachList
        queryKey={['experiment-pis', id]}
        list={() => listExperimentPrincipalInvestigators(id)}
        options={() => getLabMembers(labId)}
        add={(userId) => addExperimentPrincipalInvestigator(id, userId)}
        remove={(userId) => removeExperimentPrincipalInvestigator(id, userId)}
        label={(u) => `${u.first_name} ${u.last_name}`}
        addLabel="Add a principal investigator…"
      />

      <AppointmentsPanel experimentId={id} labId={labId} />
    </div>
  )
}
