import { useState, type SubmitEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  listExperimentTypes,
  listProtocols,
  type Experiment,
  type ExperimentInput,
} from './api'

// Shared create/edit form, mirroring ChildForm.tsx's mode: 'create' |
// 'edit' pattern. Numeric/array fields are kept as plain strings in
// local form state (same "flat strings, converted at the edges"
// approach as ChildForm/LookupTable) and only converted to
// ExperimentInput's real types on submit.

interface Values {
  name: string
  description: string
  sessions: string
  age_range_min_months: string
  age_range_max_months: string
  start_date: string
  end_date: string
  status: string
  duration_minutes: string
  filter_premies: boolean
  filter_min_languages: string
  filter_languages: string
  protocol_id: string
  experiment_type_id: string
  create_experimenter_role: boolean
}

function emptyValues(): Values {
  return {
    name: '',
    description: '',
    sessions: '1',
    age_range_min_months: '',
    age_range_max_months: '',
    start_date: '',
    end_date: '',
    status: 'not_run',
    duration_minutes: '60',
    filter_premies: true,
    filter_min_languages: '0',
    filter_languages: '',
    protocol_id: '',
    experiment_type_id: '',
    create_experimenter_role: false,
  }
}

function valuesFromExperiment(e: Experiment): Values {
  return {
    name: e.name,
    description: e.description,
    sessions: String(e.sessions),
    age_range_min_months: e.age_range_min_months === null ? '' : String(e.age_range_min_months),
    age_range_max_months: e.age_range_max_months === null ? '' : String(e.age_range_max_months),
    start_date: e.start_date ?? '',
    end_date: e.end_date ?? '',
    status: e.status,
    duration_minutes: String(e.duration_minutes),
    filter_premies: e.filter_premies,
    filter_min_languages: String(e.filter_min_languages),
    filter_languages: e.filter_languages.join(', '),
    protocol_id: e.protocol_id === null ? '' : String(e.protocol_id),
    experiment_type_id: e.experiment_type_id === null ? '' : String(e.experiment_type_id),
    create_experimenter_role: false,
  }
}

function splitList(v: string): string[] {
  return v
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s !== '')
}

function toExperimentInput(v: Values): ExperimentInput {
  return {
    name: v.name,
    description: v.description,
    sessions: Number(v.sessions),
    age_range_min_months: v.age_range_min_months === '' ? null : Number(v.age_range_min_months),
    age_range_max_months: v.age_range_max_months === '' ? null : Number(v.age_range_max_months),
    start_date: v.start_date || null,
    end_date: v.end_date || null,
    status: v.status,
    duration_minutes: Number(v.duration_minutes),
    filter_premies: v.filter_premies,
    filter_min_languages: Number(v.filter_min_languages),
    filter_languages: splitList(v.filter_languages),
    protocol_id: v.protocol_id === '' ? null : Number(v.protocol_id),
    experiment_type_id: v.experiment_type_id === '' ? null : Number(v.experiment_type_id),
    create_experimenter_role: v.create_experimenter_role,
  }
}

const STATUS_OPTIONS = [
  { value: 'not_run', label: 'Not run' },
  { value: 'pilot', label: 'Pilot' },
  { value: 'run', label: 'Run' },
]

interface Props {
  mode: 'create' | 'edit'
  labId: number
  initial?: Experiment
  onSubmit: (input: ExperimentInput) => void
  submitting: boolean
  submitLabel: string
  error?: string | null
}

export default function ExperimentForm({ mode, labId, initial, onSubmit, submitting, submitLabel, error }: Props) {
  // useState's initializer only runs once per mount. If this component
  // were reused across experiments (React Router keeps the same element
  // instance when only a route param changes), that would leave stale
  // values on screen -- ExperimentDetail avoids that by keying its
  // <ExperimentForm> on the experiment's id, so switching experiments
  // remounts this component (and re-runs the initializer) instead of
  // reusing it.
  const [values, setValues] = useState<Values>(initial ? valuesFromExperiment(initial) : emptyValues())
  const { data: protocols } = useQuery({ queryKey: ['protocols', labId], queryFn: () => listProtocols(labId) })
  const { data: experimentTypes } = useQuery({
    queryKey: ['experiment-types', labId],
    queryFn: () => listExperimentTypes(labId),
  })

  function set<K extends keyof Values>(key: K, value: Values[K]) {
    setValues((prev) => ({ ...prev, [key]: value }))
  }

  function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    onSubmit(toExperimentInput(values))
  }

  return (
    <form onSubmit={handleSubmit} className="experiment-form">
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <label>
        Name
        <input value={values.name} onChange={(e) => set('name', e.target.value)} required />
      </label>
      <label>
        Description
        <input value={values.description} onChange={(e) => set('description', e.target.value)} />
      </label>
      <label>
        Sessions
        <input
          type="number"
          min={1}
          step={1}
          value={values.sessions}
          onChange={(e) => set('sessions', e.target.value)}
          required
        />
      </label>
      <label>
        Age range min (months)
        <input
          type="number"
          min={0}
          step={0.01}
          value={values.age_range_min_months}
          onChange={(e) => set('age_range_min_months', e.target.value)}
          required
        />
      </label>
      <label>
        Age range max (months)
        <input
          type="number"
          min={0}
          step={0.01}
          value={values.age_range_max_months}
          onChange={(e) => set('age_range_max_months', e.target.value)}
          required
        />
      </label>
      <label>
        Start date
        <input type="date" value={values.start_date} onChange={(e) => set('start_date', e.target.value)} />
      </label>
      <label>
        End date
        <input type="date" value={values.end_date} onChange={(e) => set('end_date', e.target.value)} />
      </label>
      <label>
        Status
        <select value={values.status} onChange={(e) => set('status', e.target.value)}>
          {STATUS_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </label>
      <label>
        Duration (minutes)
        <input
          type="number"
          min={1}
          step={1}
          value={values.duration_minutes}
          onChange={(e) => set('duration_minutes', e.target.value)}
          required
        />
      </label>
      <label className="checkbox-label">
        <input
          type="checkbox"
          checked={values.filter_premies}
          onChange={(e) => set('filter_premies', e.target.checked)}
        />
        Filter out premies
      </label>
      <label>
        Min languages required
        <input
          type="number"
          min={0}
          step={1}
          value={values.filter_min_languages}
          onChange={(e) => set('filter_min_languages', e.target.value)}
        />
      </label>
      <label>
        Required languages (comma-separated)
        <input value={values.filter_languages} onChange={(e) => set('filter_languages', e.target.value)} />
      </label>
      <label>
        Protocol
        <select value={values.protocol_id} onChange={(e) => set('protocol_id', e.target.value)}>
          <option value="">—</option>
          {protocols
            ?.filter((p) => !p.deactivated || String(p.id) === values.protocol_id)
            .map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
                {p.deactivated ? ' (deactivated)' : ''}
              </option>
            ))}
        </select>
      </label>
      <label>
        Experiment type
        <select value={values.experiment_type_id} onChange={(e) => set('experiment_type_id', e.target.value)}>
          <option value="">—</option>
          {experimentTypes
            ?.filter((t) => !t.deactivated || String(t.id) === values.experiment_type_id)
            .map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
                {t.deactivated ? ' (deactivated)' : ''}
              </option>
            ))}
        </select>
      </label>
      {mode === 'create' && (
        <label className="checkbox-label">
          <input
            type="checkbox"
            checked={values.create_experimenter_role}
            onChange={(e) => set('create_experimenter_role', e.target.checked)}
          />
          Create a dedicated {values.name || '<name>'} Experimenter role for this experiment
        </label>
      )}
      <button type="submit" disabled={submitting}>
        {submitting ? 'Saving…' : submitLabel}
      </button>
    </form>
  )
}
