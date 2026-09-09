import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import LookupTable from './LookupTable'
import CreateDeleteList from './CreateDeleteList'
import LabMembers from './LabMembers'
import {
  createCondition,
  createConditionValue,
  createEquipment,
  createExperimentRole,
  createExperimentType,
  createGrant,
  createProtocol,
  createScheduleBlocking,
  createZipCode,
  deactivateCondition,
  deactivateConditionValue,
  deactivateEquipment,
  deactivateExperimentRole,
  deactivateExperimentType,
  deactivateGrant,
  deactivateProtocol,
  deactivateScheduleBlocking,
  deactivateZipCode,
  errorMessage,
  listConditionValues,
  listConditions,
  listEquipment,
  listExperimentRoles,
  listExperimentTypes,
  listGrants,
  listProtocols,
  listScheduleBlockings,
  listZipCodes,
  setExperimentRoleGreeter,
  setExperimentRoleSitter,
  updateCondition,
  updateConditionValue,
  updateEquipment,
  updateExperimentRole,
  updateExperimentType,
  updateGrant,
  updateProtocol,
  updateZipCode,
  type ExperimentRole,
  type ScheduleBlocking,
} from './api'

type Tab =
  | 'conditions'
  | 'equipment'
  | 'roles'
  | 'protocols'
  | 'grants'
  | 'zipcodes'
  | 'experimenttypes'
  | 'scheduleblockings'
  | 'members'

const TABS: { key: Tab; label: string }[] = [
  { key: 'conditions', label: 'Conditions' },
  { key: 'equipment', label: 'Equipment' },
  { key: 'roles', label: 'Roles' },
  { key: 'protocols', label: 'Protocols' },
  { key: 'grants', label: 'Grants' },
  { key: 'zipcodes', label: 'Zip Codes' },
  { key: 'experimenttypes', label: 'Experiment Types' },
  { key: 'scheduleblockings', label: 'Schedule Blockings' },
  { key: 'members', label: 'Members' },
]

export default function LabSetup() {
  const { labId } = useParams<{ labId: string }>()
  const [tab, setTab] = useState<Tab>('conditions')
  const id = Number(labId)

  return (
    <div className="lab-setup">
      <div className="tabs" role="tablist" aria-label="Lab setup">
        {TABS.map((t) => (
          <button
            key={t.key}
            type="button"
            role="tab"
            id={`lab-setup-tab-${t.key}`}
            aria-selected={tab === t.key}
            aria-controls={`lab-setup-panel-${t.key}`}
            tabIndex={tab === t.key ? 0 : -1}
            className={tab === t.key ? 'active' : ''}
            onClick={() => setTab(t.key)}
          >
            {t.label}
          </button>
        ))}
      </div>

      <div role="tabpanel" id={`lab-setup-panel-${tab}`} aria-labelledby={`lab-setup-tab-${tab}`}>
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

      {tab === 'experimenttypes' && (
        <LookupTable
          queryKey={['experiment-types', id]}
          fields={[{ key: 'name', label: 'Name', type: 'text' }]}
          list={() => listExperimentTypes(id)}
          create={(values) => createExperimentType(id, values.name)}
          update={(rowId, values) => updateExperimentType(rowId, values.name)}
          deactivate={(rowId) => deactivateExperimentType(rowId)}
        />
      )}

      {tab === 'scheduleblockings' && (
        <CreateDeleteList<ScheduleBlocking>
          queryKey={['schedule-blockings', id]}
          fields={[
            { key: 'date', label: 'Date', type: 'date' },
            { key: 'start_time', label: 'Start time', type: 'time' },
            { key: 'end_time', label: 'End time', type: 'time' },
            { key: 'reason', label: 'Reason', type: 'text', required: false },
          ]}
          list={() => listScheduleBlockings(id)}
          create={(v) => createScheduleBlocking(id, v.date, v.start_time, v.end_time, v.reason)}
          remove={deactivateScheduleBlocking}
        />
      )}

      {tab === 'members' && <LabMembers labId={id} />}
      </div>
    </div>
  )
}

// Roles gets its own small wrapper rather than an inline LookupTable
// instantiation: the sitter/greeter toggles are dedicated actions
// (setExperimentRoleSitter/setExperimentRoleGreeter), not regular field
// edits, and need their own mutations + error handling alongside
// LookupTable's standard ones.
function RolesTable({ labId }: { labId: number }) {
  const queryClient = useQueryClient()
  const [sitterError, setSitterError] = useState<string | null>(null)
  const [greeterError, setGreeterError] = useState<string | null>(null)

  const sitterMutation = useMutation({
    mutationFn: ({ id, isSitterRole }: { id: number; isSitterRole: boolean }) =>
      setExperimentRoleSitter(id, isSitterRole),
    onSuccess: () => {
      setSitterError(null)
      void queryClient.invalidateQueries({ queryKey: ['roles', labId] })
    },
    onError: (err) => setSitterError(errorMessage(err, 'Failed to set sitter role.')),
  })
  const greeterMutation = useMutation({
    mutationFn: ({ id, isGreeterRole }: { id: number; isGreeterRole: boolean }) =>
      setExperimentRoleGreeter(id, isGreeterRole),
    onSuccess: () => {
      setGreeterError(null)
      void queryClient.invalidateQueries({ queryKey: ['roles', labId] })
    },
    onError: (err) => setGreeterError(errorMessage(err, 'Failed to set greeter role.')),
  })

  return (
    <div>
      {sitterError && (
        <p className="error" role="alert">
          {sitterError}
        </p>
      )}
      {greeterError && (
        <p className="error" role="alert">
          {greeterError}
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
          <>
            <button
              type="button"
              onClick={() => sitterMutation.mutate({ id: role.id, isSitterRole: !role.is_sitter_role })}
              disabled={sitterMutation.isPending}
            >
              {role.is_sitter_role ? 'Unset sitter role' : 'Set as sitter role'}
            </button>
            <button
              type="button"
              onClick={() => greeterMutation.mutate({ id: role.id, isGreeterRole: !role.is_greeter_role })}
              disabled={greeterMutation.isPending}
            >
              {role.is_greeter_role ? 'Unset greeter role' : 'Set as greeter role'}
            </button>
          </>
        )}
      />
    </div>
  )
}
