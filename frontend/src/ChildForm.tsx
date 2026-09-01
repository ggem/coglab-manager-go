import { useState, type SubmitEvent } from 'react'
import type { Child, ChildInput, RecruitmentSource } from './api'

// The ~15-field child form, shared by creation (a fresh row below the
// children table) and editing (an existing child's expanded row) --
// building it once avoids two copies of the same field list drifting
// apart. Numeric/tri-state/array fields are kept as plain strings in
// local form state (the same "flat strings, converted at the edges"
// approach LookupTable uses) and only converted to ChildInput's real
// types on submit.

interface Values {
  first_name: string
  last_name: string
  sex: string
  birth_date: string
  due_date: string
  gestational_age_weeks: string
  birth_weight: string
  apgar_1: string
  apgar_2: string
  premie: string
  birth_complications: string
  birth_complications_notes: string
  twin: string
  race_ethnicity: string[]
  languages: string
  recruitment_source_id: string
  recruitment_source_other: string
  response: string
  mcdi_percentile: string
  mcdi_date: string
}

function triToSelect(v: boolean | null): string {
  return v === null ? 'unknown' : v ? 'yes' : 'no'
}
function selectToTri(v: string): boolean | null {
  return v === 'unknown' ? null : v === 'yes'
}
function numToStr(v: number | null): string {
  return v === null ? '' : String(v)
}
function strToNum(v: string): number | null {
  return v === '' ? null : Number(v)
}

function emptyValues(): Values {
  return {
    first_name: '',
    last_name: '',
    sex: 'unknown',
    birth_date: '',
    due_date: '',
    gestational_age_weeks: '',
    birth_weight: '',
    apgar_1: '',
    apgar_2: '',
    premie: 'unknown',
    birth_complications: 'unknown',
    birth_complications_notes: '',
    twin: 'unknown',
    race_ethnicity: [],
    languages: 'English',
    recruitment_source_id: '',
    recruitment_source_other: '',
    response: 'unknown',
    mcdi_percentile: '',
    mcdi_date: '',
  }
}

function valuesFromChild(child: Child): Values {
  return {
    first_name: child.first_name,
    last_name: child.last_name,
    sex: child.sex,
    birth_date: child.birth_date ?? '',
    due_date: child.due_date ?? '',
    gestational_age_weeks: numToStr(child.gestational_age_weeks),
    birth_weight: numToStr(child.birth_weight),
    apgar_1: numToStr(child.apgar_1),
    apgar_2: numToStr(child.apgar_2),
    premie: triToSelect(child.premie),
    birth_complications: triToSelect(child.birth_complications),
    birth_complications_notes: child.birth_complications_notes,
    twin: triToSelect(child.twin),
    race_ethnicity: child.race_ethnicity,
    languages: child.languages.join(', '),
    recruitment_source_id: child.recruitment_source_id === null ? '' : String(child.recruitment_source_id),
    recruitment_source_other: child.recruitment_source_other,
    response: child.response,
    mcdi_percentile: numToStr(child.mcdi_percentile),
    mcdi_date: child.mcdi_date ?? '',
  }
}

function splitList(v: string): string[] {
  return v
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s !== '')
}

function toChildInput(v: Values): ChildInput {
  return {
    first_name: v.first_name,
    last_name: v.last_name,
    sex: v.sex,
    birth_date: v.birth_date || null,
    due_date: v.due_date || null,
    gestational_age_weeks: strToNum(v.gestational_age_weeks),
    birth_weight: strToNum(v.birth_weight),
    apgar_1: strToNum(v.apgar_1),
    apgar_2: strToNum(v.apgar_2),
    premie: selectToTri(v.premie),
    birth_complications: selectToTri(v.birth_complications),
    birth_complications_notes: v.birth_complications === 'no' ? '' : v.birth_complications_notes,
    twin: selectToTri(v.twin),
    race_ethnicity: v.race_ethnicity,
    languages: splitList(v.languages),
    recruitment_source_id: v.recruitment_source_id === '' ? null : Number(v.recruitment_source_id),
    recruitment_source_other: v.recruitment_source_other,
    response: v.response,
    mcdi_percentile: strToNum(v.mcdi_percentile),
    mcdi_date: v.mcdi_date || null,
  }
}

const SEX_OPTIONS = [
  { value: 'unknown', label: 'Unknown' },
  { value: 'male', label: 'Male' },
  { value: 'female', label: 'Female' },
]
const TRI_OPTIONS = [
  { value: 'unknown', label: 'Unknown' },
  { value: 'yes', label: 'Yes' },
  { value: 'no', label: 'No' },
]
// Matches the children.race_ethnicity CHECK constraint exactly -- a
// fixed set, not free text, and multivalued (a child can be more than
// one), hence checkboxes rather than a single <select>.
const RACE_ETHNICITY_OPTIONS = [
  { value: 'american_indian_or_alaska_native', label: 'American Indian or Alaska Native' },
  { value: 'asian', label: 'Asian' },
  { value: 'black_or_african_american', label: 'Black or African American' },
  { value: 'hispanic_or_latino', label: 'Hispanic or Latino' },
  { value: 'middle_eastern_or_north_african', label: 'Middle Eastern or North African' },
  { value: 'native_hawaiian_or_pacific_islander', label: 'Native Hawaiian or Pacific Islander' },
  { value: 'white', label: 'White' },
]
// Matches the children.response CHECK constraint exactly.
const RESPONSE_OPTIONS = [
  { value: 'unknown', label: 'Unknown' },
  { value: 'email', label: 'Email' },
  { value: 'snail_mail', label: 'Mail' },
  { value: 'phone', label: 'Phone' },
  { value: 'web_page', label: 'Web page' },
]

interface Props {
  mode: 'create' | 'edit'
  initial?: Child
  recruitmentSources: RecruitmentSource[]
  onSubmit: (input: ChildInput) => void
  submitting: boolean
  submitLabel: string
  error?: string | null
}

export default function ChildForm({
  mode,
  initial,
  recruitmentSources,
  onSubmit,
  submitting,
  submitLabel,
  error,
}: Props) {
  const [values, setValues] = useState<Values>(initial ? valuesFromChild(initial) : emptyValues())

  function set<K extends keyof Values>(key: K, value: string) {
    setValues((prev) => ({ ...prev, [key]: value }))
  }

  function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    onSubmit(toChildInput(values))
  }

  return (
    <form onSubmit={handleSubmit} className="child-form">
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <label>
        First name
        <input value={values.first_name} onChange={(e) => set('first_name', e.target.value)} required />
      </label>
      <label>
        Last name
        <input value={values.last_name} onChange={(e) => set('last_name', e.target.value)} required />
      </label>
      <label>
        Sex
        <select value={values.sex} onChange={(e) => set('sex', e.target.value)}>
          {SEX_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </label>
      <label>
        Birth date
        <input type="date" value={values.birth_date} onChange={(e) => set('birth_date', e.target.value)} />
      </label>
      <label>
        Due date
        <input type="date" value={values.due_date} onChange={(e) => set('due_date', e.target.value)} />
      </label>
      <label>
        Gestational age (weeks)
        <input
          type="number"
          value={values.gestational_age_weeks}
          onChange={(e) => set('gestational_age_weeks', e.target.value)}
        />
      </label>
      <label>
        Birth weight
        <input type="number" value={values.birth_weight} onChange={(e) => set('birth_weight', e.target.value)} />
      </label>
      <label>
        Apgar (1 min)
        <input type="number" value={values.apgar_1} onChange={(e) => set('apgar_1', e.target.value)} />
      </label>
      <label>
        Apgar (5 min)
        <input type="number" value={values.apgar_2} onChange={(e) => set('apgar_2', e.target.value)} />
      </label>
      <label>
        Premie
        <select value={values.premie} onChange={(e) => set('premie', e.target.value)}>
          {TRI_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </label>
      <label>
        Birth complications
        <select value={values.birth_complications} onChange={(e) => set('birth_complications', e.target.value)}>
          {TRI_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </label>
      {values.birth_complications !== 'no' && (
        <label>
          Birth complications, detail
          <textarea
            value={values.birth_complications_notes}
            onChange={(e) => set('birth_complications_notes', e.target.value)}
          />
        </label>
      )}
      <label>
        Twin
        <select value={values.twin} onChange={(e) => set('twin', e.target.value)}>
          {TRI_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </label>
      <fieldset className="race-ethnicity">
        <legend>Race/ethnicity</legend>
        {RACE_ETHNICITY_OPTIONS.map((o) => (
          <label key={o.value} className="checkbox-label">
            <input
              type="checkbox"
              checked={values.race_ethnicity.includes(o.value)}
              onChange={(e) =>
                setValues((prev) => ({
                  ...prev,
                  race_ethnicity: e.target.checked
                    ? [...prev.race_ethnicity, o.value]
                    : prev.race_ethnicity.filter((v) => v !== o.value),
                }))
              }
            />
            {o.label}
          </label>
        ))}
      </fieldset>
      <label>
        Languages (comma-separated)
        <input value={values.languages} onChange={(e) => set('languages', e.target.value)} />
      </label>
      <label>
        Recruitment source
        <select
          value={values.recruitment_source_id}
          onChange={(e) => set('recruitment_source_id', e.target.value)}
        >
          <option value="">—</option>
          {recruitmentSources.map((rs) => (
            <option key={rs.id} value={rs.id}>
              {rs.name}
            </option>
          ))}
        </select>
      </label>
      <label>
        Recruitment source, other
        <input
          value={values.recruitment_source_other}
          onChange={(e) => set('recruitment_source_other', e.target.value)}
        />
      </label>
      <label>
        Response
        <select value={values.response} onChange={(e) => set('response', e.target.value)}>
          {RESPONSE_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </label>
      {mode === 'edit' && (
        <>
          <label>
            MCDI percentile
            <input
              type="number"
              value={values.mcdi_percentile}
              onChange={(e) => set('mcdi_percentile', e.target.value)}
            />
          </label>
          <label>
            MCDI date
            <input type="date" value={values.mcdi_date} onChange={(e) => set('mcdi_date', e.target.value)} />
          </label>
        </>
      )}
      <button type="submit" disabled={submitting}>
        {submitting ? 'Saving…' : submitLabel}
      </button>
    </form>
  )
}
