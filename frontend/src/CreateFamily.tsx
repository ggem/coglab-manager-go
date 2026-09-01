import { useState, type SubmitEvent } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { createFamily, errorMessage, type FamilyInput } from './api'

const CONTACT_METHODS = [
  { value: 'home_phone', label: 'Home phone' },
  { value: 'work_phone', label: 'Work phone' },
  { value: 'mobile_phone', label: 'Mobile phone' },
  { value: 'fax', label: 'Fax' },
  { value: 'email', label: 'Email' },
  { value: 'snail_mail', label: 'Mail' },
]

export default function CreateFamily() {
  const navigate = useNavigate()
  const [values, setValues] = useState<FamilyInput>({
    address: '',
    city: '',
    state: '',
    zip: '',
    preferred_contact_method: null,
  })

  const createMutation = useMutation({
    mutationFn: () => createFamily(values),
    onSuccess: (family) => navigate(`/app/families/${family.id}`),
  })

  function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    createMutation.mutate()
  }

  return (
    <div className="create-family">
      <h2>Add family</h2>
      {createMutation.isError && (
        <p className="error" role="alert">
          {errorMessage(createMutation.error, 'Failed to create family.')}
        </p>
      )}
      <form onSubmit={handleSubmit}>
        <label>
          Address
          <input value={values.address} onChange={(e) => setValues({ ...values, address: e.target.value })} />
        </label>
        <label>
          City
          <input value={values.city} onChange={(e) => setValues({ ...values, city: e.target.value })} />
        </label>
        <label>
          State
          <input value={values.state} onChange={(e) => setValues({ ...values, state: e.target.value })} />
        </label>
        <label>
          Zip
          <input value={values.zip} onChange={(e) => setValues({ ...values, zip: e.target.value })} />
        </label>
        <label>
          Preferred contact
          <select
            value={values.preferred_contact_method ?? ''}
            onChange={(e) =>
              setValues({ ...values, preferred_contact_method: e.target.value === '' ? null : e.target.value })
            }
          >
            <option value="">—</option>
            {CONTACT_METHODS.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </label>
        <button type="submit" disabled={createMutation.isPending}>
          {createMutation.isPending ? 'Creating…' : 'Create family'}
        </button>
      </form>
    </div>
  )
}
