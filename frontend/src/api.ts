// Thin fetch wrapper + types mirroring the Go handler DTOs
// (internal/httpapi/*_handlers.go). Handwritten rather than generated --
// there's no OpenAPI spec yet, and keeping the shapes visible here matches
// this project's preference for explicit code over codegen magic.

// The server responded, just not successfully -- status carries the HTTP
// status code, message is the server's `error` field when the body was
// JSON, or the raw response text otherwise.
export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// fetch() itself failed and no response was ever received (offline, DNS
// failure, connection refused). Distinct from ApiError, which means the
// server responded.
export class NetworkError extends Error {}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      ...init,
      headers: { 'Content-Type': 'application/json', ...init?.headers },
    })
  } catch (err) {
    throw new NetworkError(err instanceof Error ? err.message : 'network request failed')
  }

  if (!res.ok) {
    const text = await res.text()
    let message = text || `request failed: ${res.status}`
    try {
      const body: unknown = JSON.parse(text)
      if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') {
        message = body.error
      }
    } catch {
      // not JSON -- fall back to the raw text set above
    }
    throw new ApiError(res.status, message)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

// Turns a caught error into a message worth showing a user: the server's
// own message for ApiError, "can't reach the server" for NetworkError
// (rather than fetch's raw "Failed to fetch"), or fallback for anything
// else (a bug, most likely -- not worth showing verbatim).
export function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof NetworkError) return "Can't reach the server -- check your connection and try again."
  return fallback
}

export interface User {
  id: number
  email: string
  first_name: string
  last_name: string
}

export interface LoginResponse {
  user: User
}

export function login(email: string, password: string): Promise<LoginResponse> {
  return apiFetch<LoginResponse>('/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
}

export function logout(): Promise<void> {
  return apiFetch<void>('/logout', { method: 'POST' })
}

// Restores session state after a page refresh -- 401s (via ApiError) if
// there's no valid session cookie, which the caller treats the same as
// "not logged in."
export function getMe(): Promise<LoginResponse> {
  return apiFetch<LoginResponse>('/me')
}

export interface ChildSearchResult {
  id: number
  family_id: number
  first_name: string
  last_name: string
  sex: string
  birth_date: string | null
  deactivated: boolean
}

export function searchChildren(query: string): Promise<ChildSearchResult[]> {
  const params = new URLSearchParams()
  if (query) params.set('q', query)
  return apiFetch<ChildSearchResult[]>(`/children/search?${params.toString()}`)
}

export interface FamilySearchResult {
  id: number
  address: string
  city: string
  state: string
  zip: string
}

export function searchFamilies(query: string): Promise<FamilySearchResult[]> {
  const params = new URLSearchParams()
  if (query) params.set('q', query)
  return apiFetch<FamilySearchResult[]>(`/families/search?${params.toString()}`)
}

// --- Families, guardians, children, notes ---
//
// Families/children/guardians are not lab-scoped (unlike everything in
// the "Lab setup" section below) -- confirmed via the families/children
// migrations, matching legacy where participants were shared across the
// whole system, not siloed per lab.

export interface Family {
  id: number
  address: string
  city: string
  state: string
  zip: string
  preferred_contact_method: string | null
  created_at: string
  updated_at: string
}

export interface FamilyInput {
  address: string
  city: string
  state: string
  zip: string
  preferred_contact_method: string | null
}

export function createFamily(input: FamilyInput): Promise<Family> {
  return apiFetch<Family>('/families/', { method: 'POST', body: JSON.stringify(input) })
}
export function getFamily(id: number): Promise<Family> {
  return apiFetch<Family>(`/families/${id}/`)
}
export function updateFamily(id: number, input: FamilyInput): Promise<Family> {
  return apiFetch<Family>(`/families/${id}/`, { method: 'PUT', body: JSON.stringify(input) })
}

export interface Guardian {
  id: number
  family_id: number
  first_name: string
  last_name: string
  education: string
  occupation: string
  phone_number: string
  phone_type: string | null
  email: string
  deactivated: boolean
  created_at: string
  updated_at: string
}

export interface GuardianInput {
  first_name: string
  last_name: string
  education: string
  occupation: string
  phone_number: string
  phone_type: string | null
  email: string
}

export function listGuardiansByFamily(familyId: number): Promise<Guardian[]> {
  return apiFetch<Guardian[]>(`/families/${familyId}/guardians/`)
}
export function createGuardian(familyId: number, input: GuardianInput): Promise<Guardian> {
  return apiFetch<Guardian>(`/families/${familyId}/guardians/`, { method: 'POST', body: JSON.stringify(input) })
}
export function updateGuardian(id: number, input: GuardianInput): Promise<Guardian> {
  return apiFetch<Guardian>(`/guardians/${id}/`, { method: 'PUT', body: JSON.stringify(input) })
}
// Soft-delete (deactivated_at), despite the DELETE verb -- matches Child
// and the rest of this app's deactivate-not-destroy philosophy.
export function deactivateGuardian(id: number): Promise<void> {
  return apiFetch<void>(`/guardians/${id}/`, { method: 'DELETE' })
}

export interface Child {
  id: number
  family_id: number
  first_name: string
  last_name: string
  sex: string
  birth_date: string | null
  due_date: string | null
  gestational_age_weeks: number | null
  birth_weight: number | null
  apgar_1: number | null
  apgar_2: number | null
  premie: boolean | null
  birth_complications: boolean | null
  birth_complications_notes: string
  twin: boolean | null
  race_ethnicity: string[]
  languages: string[]
  recruitment_source_id: number | null
  recruitment_source_other: string
  response: string
  mcdi_percentile: number | null
  mcdi_date: string | null
  deactivated: boolean
  inactive_reason: string
  created_at: string
  updated_at: string
}

export interface ChildInput {
  first_name: string
  last_name: string
  sex: string
  birth_date: string | null
  due_date: string | null
  gestational_age_weeks: number | null
  birth_weight: number | null
  apgar_1: number | null
  apgar_2: number | null
  premie: boolean | null
  birth_complications: boolean | null
  birth_complications_notes: string
  twin: boolean | null
  race_ethnicity: string[]
  languages: string[]
  recruitment_source_id: number | null
  recruitment_source_other: string
  response: string
  // Manual staff data entry -- ignored by the create endpoint (a new
  // child can't have results yet), used by update.
  mcdi_percentile: number | null
  mcdi_date: string | null
}

export function listChildrenByFamily(familyId: number): Promise<Child[]> {
  return apiFetch<Child[]>(`/families/${familyId}/children/`)
}
export function createChild(familyId: number, input: ChildInput): Promise<Child> {
  return apiFetch<Child>(`/families/${familyId}/children/`, { method: 'POST', body: JSON.stringify(input) })
}
export function updateChild(id: number, input: ChildInput): Promise<Child> {
  return apiFetch<Child>(`/children/${id}/`, { method: 'PUT', body: JSON.stringify(input) })
}
export function deactivateChild(id: number, reason: string): Promise<void> {
  return apiFetch<void>(`/children/${id}/deactivate`, { method: 'POST', body: JSON.stringify({ reason }) })
}

export interface Note {
  id: number
  author_user_id: number
  body: string
  created_at: string
}

// Notes are child-only, matching legacy (families/guardians never had
// notes) and the backend, which hardcodes entity_type "child".
export function listChildNotes(childId: number): Promise<Note[]> {
  return apiFetch<Note[]>(`/children/${childId}/notes/`)
}
export function createChildNote(childId: number, body: string): Promise<Note> {
  return apiFetch<Note>(`/children/${childId}/notes/`, { method: 'POST', body: JSON.stringify({ body }) })
}

export interface RecruitmentSource {
  id: number
  name: string
}

// Populates the child form's recruitment-source dropdown, paired with
// a free-text "other" field (recruitment_source_other) always available
// alongside it, matching legacy's source/source_other pairing.
export function listRecruitmentSources(): Promise<RecruitmentSource[]> {
  return apiFetch<RecruitmentSource[]>('/recruitment-sources')
}
export function createRecruitmentSource(name: string): Promise<RecruitmentSource> {
  return apiFetch<RecruitmentSource>('/recruitment-sources', { method: 'POST', body: JSON.stringify({ name }) })
}

// --- Labs ---

export interface Lab {
  id: number
  name: string
  short_name: string
}

// The labs the current user belongs to -- the starting point for
// everything lab-scoped below, since nothing else lists lab IDs.
export function getLabs(): Promise<Lab[]> {
  return apiFetch<Lab[]>('/labs')
}

// --- Lab setup: conditions, equipment, experiment roles, protocols,
// grants, zip codes. All six share the same create/list/update/
// deactivate shape (LookupTable.tsx is the shared UI for it); the
// functions below are still written out individually rather than
// generated, matching this file's own stated preference for explicit
// code over codegen magic.

export interface LookupRow {
  id: number
  lab_id: number
  deactivated: boolean
  created_at: string
  updated_at: string
}

export interface Condition extends LookupRow {
  name: string
}

export function listConditions(labId: number): Promise<Condition[]> {
  return apiFetch<Condition[]>(`/labs/${labId}/conditions/`)
}
export function createCondition(labId: number, name: string): Promise<Condition> {
  return apiFetch<Condition>(`/labs/${labId}/conditions/`, { method: 'POST', body: JSON.stringify({ name }) })
}
export function updateCondition(id: number, name: string): Promise<Condition> {
  return apiFetch<Condition>(`/conditions/${id}/`, { method: 'PUT', body: JSON.stringify({ name }) })
}
export function deactivateCondition(id: number): Promise<void> {
  return apiFetch<void>(`/conditions/${id}/deactivate`, { method: 'POST' })
}

export interface ConditionValue extends LookupRow {
  condition_id: number
  name: string
}

export function listConditionValues(conditionId: number): Promise<ConditionValue[]> {
  return apiFetch<ConditionValue[]>(`/conditions/${conditionId}/values/`)
}
export function createConditionValue(conditionId: number, name: string): Promise<ConditionValue> {
  return apiFetch<ConditionValue>(`/conditions/${conditionId}/values/`, {
    method: 'POST',
    body: JSON.stringify({ name }),
  })
}
export function updateConditionValue(id: number, name: string): Promise<ConditionValue> {
  return apiFetch<ConditionValue>(`/condition-values/${id}/`, { method: 'PUT', body: JSON.stringify({ name }) })
}
export function deactivateConditionValue(id: number): Promise<void> {
  return apiFetch<void>(`/condition-values/${id}/deactivate`, { method: 'POST' })
}

export interface Equipment extends LookupRow {
  name: string
  quantity: number
}

export function listEquipment(labId: number): Promise<Equipment[]> {
  return apiFetch<Equipment[]>(`/labs/${labId}/equipment/`)
}
export function createEquipment(labId: number, name: string, quantity: number): Promise<Equipment> {
  return apiFetch<Equipment>(`/labs/${labId}/equipment/`, {
    method: 'POST',
    body: JSON.stringify({ name, quantity }),
  })
}
export function updateEquipment(id: number, name: string, quantity: number): Promise<Equipment> {
  return apiFetch<Equipment>(`/equipment/${id}/`, { method: 'PUT', body: JSON.stringify({ name, quantity }) })
}
export function deactivateEquipment(id: number): Promise<void> {
  return apiFetch<void>(`/equipment/${id}/deactivate`, { method: 'POST' })
}

export interface ExperimentRole extends LookupRow {
  name: string
  is_sitter_role: boolean
}

export function listExperimentRoles(labId: number): Promise<ExperimentRole[]> {
  return apiFetch<ExperimentRole[]>(`/labs/${labId}/experiment-roles/`)
}
export function createExperimentRole(labId: number, name: string): Promise<ExperimentRole> {
  return apiFetch<ExperimentRole>(`/labs/${labId}/experiment-roles/`, {
    method: 'POST',
    body: JSON.stringify({ name }),
  })
}
export function updateExperimentRole(id: number, name: string): Promise<ExperimentRole> {
  return apiFetch<ExperimentRole>(`/experiment-roles/${id}/`, { method: 'PUT', body: JSON.stringify({ name }) })
}
export function deactivateExperimentRole(id: number): Promise<void> {
  return apiFetch<void>(`/experiment-roles/${id}/deactivate`, { method: 'POST' })
}
// A dedicated action, not part of the regular update -- at most one role
// per lab can be the sitter role (enforced server-side), so setting it
// is a designation, not just editing a field.
export function setExperimentRoleSitter(id: number, isSitterRole: boolean): Promise<ExperimentRole> {
  return apiFetch<ExperimentRole>(`/experiment-roles/${id}/set-sitter`, {
    method: 'POST',
    body: JSON.stringify({ is_sitter_role: isSitterRole }),
  })
}

export interface Protocol extends LookupRow {
  name: string
}

export function listProtocols(labId: number): Promise<Protocol[]> {
  return apiFetch<Protocol[]>(`/labs/${labId}/protocols/`)
}
export function createProtocol(labId: number, name: string): Promise<Protocol> {
  return apiFetch<Protocol>(`/labs/${labId}/protocols/`, { method: 'POST', body: JSON.stringify({ name }) })
}
export function updateProtocol(id: number, name: string): Promise<Protocol> {
  return apiFetch<Protocol>(`/protocols/${id}/`, { method: 'PUT', body: JSON.stringify({ name }) })
}
export function deactivateProtocol(id: number): Promise<void> {
  return apiFetch<void>(`/protocols/${id}/deactivate`, { method: 'POST' })
}

export interface Grant extends LookupRow {
  name: string
}

export function listGrants(labId: number): Promise<Grant[]> {
  return apiFetch<Grant[]>(`/labs/${labId}/grants/`)
}
export function createGrant(labId: number, name: string): Promise<Grant> {
  return apiFetch<Grant>(`/labs/${labId}/grants/`, { method: 'POST', body: JSON.stringify({ name }) })
}
export function updateGrant(id: number, name: string): Promise<Grant> {
  return apiFetch<Grant>(`/grants/${id}/`, { method: 'PUT', body: JSON.stringify({ name }) })
}
export function deactivateGrant(id: number): Promise<void> {
  return apiFetch<void>(`/grants/${id}/deactivate`, { method: 'POST' })
}

export interface ZipCode extends LookupRow {
  zip_code: string
  priority: string
}

export function listZipCodes(labId: number): Promise<ZipCode[]> {
  return apiFetch<ZipCode[]>(`/labs/${labId}/zip-codes/`)
}
export function createZipCode(labId: number, zipCode: string, priority: string): Promise<ZipCode> {
  return apiFetch<ZipCode>(`/labs/${labId}/zip-codes/`, {
    method: 'POST',
    body: JSON.stringify({ zip_code: zipCode, priority }),
  })
}
export function updateZipCode(id: number, zipCode: string, priority: string): Promise<ZipCode> {
  return apiFetch<ZipCode>(`/zip-codes/${id}/`, {
    method: 'PUT',
    body: JSON.stringify({ zip_code: zipCode, priority }),
  })
}
export function deactivateZipCode(id: number): Promise<void> {
  return apiFetch<void>(`/zip-codes/${id}/deactivate`, { method: 'POST' })
}
