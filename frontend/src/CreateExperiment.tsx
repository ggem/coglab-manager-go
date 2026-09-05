import { useMutation } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import ExperimentForm from './ExperimentForm'
import { createExperiment, errorMessage, type ExperimentInput } from './api'

export default function CreateExperiment() {
  const { labId } = useParams<{ labId: string }>()
  const id = Number(labId)
  const navigate = useNavigate()

  const createMutation = useMutation({
    mutationFn: (input: ExperimentInput) => createExperiment(id, input),
    onSuccess: (experiment) => navigate(`/app/experiments/${experiment.id}`),
  })

  return (
    <div className="create-experiment">
      <h2>Add experiment</h2>
      <ExperimentForm
        mode="create"
        labId={id}
        onSubmit={(input) => createMutation.mutate(input)}
        submitting={createMutation.isPending}
        submitLabel="Create experiment"
        error={createMutation.isError ? errorMessage(createMutation.error, 'Failed to create experiment.') : null}
      />
    </div>
  )
}
