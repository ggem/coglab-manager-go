// Shared value/label options for lab_memberships.priority -- kept in
// its own module (not exported from LabMembers.tsx directly) for the
// same reason as participantOptions.ts: any future read-only display
// of priority can reuse these labels without oxlint's
// react(only-export-components) Fast Refresh warning.
//
// Ordered the same way the scheduling search sorts candidates (see
// ListLabMemberTrainingsForRoleByPriority in
// internal/db/queries/lab_member_trainings.sql) -- lower priority
// scheduled first -- so the dropdown's order matches what picking a
// value actually does.

export const PRIORITY_OPTIONS = [
  { value: 'undergrad_no_project', label: 'Undergrad (no project)' },
  { value: 'undergrad_with_project', label: 'Undergrad (with project)' },
  { value: 'lab_coordinator', label: 'Lab coordinator' },
  { value: 'graduate_student', label: 'Graduate student' },
  { value: 'postdoc', label: 'Postdoc' },
  { value: 'lab_director', label: 'Lab director' },
]

export const PRIORITY_LABELS = Object.fromEntries(PRIORITY_OPTIONS.map((o) => [o.value, o.label]))
