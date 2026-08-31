import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import LookupTable from './LookupTable'
import {
  createCondition,
  createConditionValue,
  createEquipment,
  createExperimentRole,
  createGrant,
  createProtocol,
  createZipCode,
  deactivateCondition,
  deactivateConditionValue,
  deactivateEquipment,
  deactivateExperimentRole,
  deactivateGrant,
  deactivateProtocol,
  deactivateZipCode,
  errorMessage,
  listConditionValues,
  listConditions,
  listEquipment,
  listExperimentRoles,
  listGrants,
  listProtocols,
  listZipCodes,
  setExperimentRoleSitter,
  updateCondition,
  updateConditionValue,
  updateEquipment,
  updateExperimentRole,
  updateGrant,
  updateProtocol,
  updateZipCode,
  type ExperimentRole,
} from './api'

type Tab = 'conditions' | 'equipment' | 'roles' | 'protocols' | 'grants' | 'zipcodes'

const TABS: { key: Tab; label: string }[] = [
  { key: 'conditions', label: 'Conditions' },
  { key: 'equipment', label: 'Equipment' },
  { key: 'roles', label: 'Roles' },
  { key: 'protocols', label: 'Protocols' },
  { key: 'grants', label: 'Grants' },
  { key: 'zipcodes', label: 'Zip Codes' },
]

export default function LabSetup() {
  const { labId } = useParams<{ labId: string }>()
  const [tab, setTab] = useState<Tab>('conditions')
  const id = Number(labId)

  return (
    <div className="lab-setup">
      <div className="tabs">
        {TABS.map((t) => (
          <button key={t.key} type="button" className={tab === t.key ? 'active' : ''} onClick={() => setTab(t.key)}>
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'conditions' && (
        <LookupTable
          queryKey={['conditions', id]}
          fields={[{ key: 'name', label: 'Name', type: 'text' }]}
          list={() => listConditions(id)}
          create={(values) => createCondition(id, values.name)}
          update={(rowId, values) => updateCondition(rowId, values.name)}
          deactivate={(rowId) => deactivateCondition(rowId)}
          renderExpanded={(condition) => (
            <LookupTable
              queryKey={['condition-values', condition.id]}
              fields={[{ key: 'name', label: 'Value', type: 'text' }]}
              list={() => listConditionValues(condition.id)}
              create={(values) => createConditionValue(condition.id, values.name)}
              update={(rowId, values) => updateConditionValue(rowId, values.name)}
              deactivate={(rowId) => deactivateConditionValue(rowId)}
            />
          )}
        />
      )}

      {tab === 'equipment' && (
        <LookupTable
          queryKey={['equipment', id]}
          fields={[
            { key: 'name', label: 'Name', type: 'text' },
            { key: 'quantity', label: 'Quantity', type: 'number' },
          ]}
          list={() => listEquipment(id)}
          create={(values) => createEquipment(id, values.name, Number(values.quantity))}
          update={(rowId, values) => updateEquipment(rowId, values.name, Number(values.quantity))}
          deactivate={(rowId) => deactivateEquipment(rowId)}
        />
      )}

      {tab === 'roles' && <RolesTable labId={id} />}

      {tab === 'protocols' && (
        <LookupTable
          queryKey={['protocols', id]}
          fields={[{ key: 'name', label: 'Name', type: 'text' }]}
          list={() => listProtocols(id)}
          create={(values) => createProtocol(id, values.name)}
          update={(rowId, values) => updateProtocol(rowId, values.name)}
          deactivate={(rowId) => deactivateProtocol(rowId)}
        />
      )}

      {tab === 'grants' && (
        <LookupTable
          queryKey={['grants', id]}
          fields={[{ key: 'name', label: 'Name', type: 'text' }]}
          list={() => listGrants(id)}
          create={(values) => createGrant(id, values.name)}
          update={(rowId, values) => updateGrant(rowId, values.name)}
          deactivate={(rowId) => deactivateGrant(rowId)}
        />
      )}

      {tab === 'zipcodes' && (
        <LookupTable
          queryKey={['zipcodes', id]}
          fields={[
            { key: 'zip_code', label: 'Zip Code', type: 'text' },
            { key: 'priority', label: 'Priority', type: 'text' },
          ]}
          list={() => listZipCodes(id)}
          create={(values) => createZipCode(id, values.zip_code, values.priority)}
          update={(rowId, values) => updateZipCode(rowId, values.zip_code, values.priority)}
          deactivate={(rowId) => deactivateZipCode(rowId)}
        />
      )}
    </div>
  )
}

// Roles gets its own small wrapper rather than an inline LookupTable
// instantiation: the sitter toggle is a dedicated action
// (setExperimentRoleSitter), not a regular field edit, and needs its
// own mutation + error handling alongside LookupTable's standard ones.
function RolesTable({ labId }: { labId: number }) {
  const queryClient = useQueryClient()
  const [sitterError, setSitterError] = useState<string | null>(null)

  const sitterMutation = useMutation({
    mutationFn: ({ id, isSitterRole }: { id: number; isSitterRole: boolean }) =>
      setExperimentRoleSitter(id, isSitterRole),
    onSuccess: () => {
      setSitterError(null)
      queryClient.invalidateQueries({ queryKey: ['roles', labId] })
    },
    onError: (err) => setSitterError(errorMessage(err, 'Failed to set sitter role.')),
  })

  return (
    <div>
      {sitterError && (
        <p className="error" role="alert">
          {sitterError}
        </p>
      )}
      <LookupTable
        queryKey={['roles', labId]}
        fields={[{ key: 'name', label: 'Name', type: 'text' }]}
        list={() => listExperimentRoles(labId)}
        create={(values) => createExperimentRole(labId, values.name)}
        update={(rowId, values) => updateExperimentRole(rowId, values.name)}
        deactivate={(rowId) => deactivateExperimentRole(rowId)}
        extraActions={(role: ExperimentRole) => (
          <button
            type="button"
            onClick={() => sitterMutation.mutate({ id: role.id, isSitterRole: !role.is_sitter_role })}
            disabled={sitterMutation.isPending}
          >
            {role.is_sitter_role ? 'Unset sitter role' : 'Set as sitter role'}
          </button>
        )}
      />
    </div>
  )
}
