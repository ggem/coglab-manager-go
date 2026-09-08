// Shared value/label option lists for participant enum fields -- kept
// in their own module (not exported from ChildForm.tsx/FamilyDetail.tsx
// directly) so components that only display these values (e.g.
// DemographicsReport.tsx) can show the same labels the edit forms use,
// without oxlint's react(only-export-components) Fast Refresh warning
// that comes from exporting non-component values out of a component file.

export const SEX_OPTIONS = [
  { value: 'unknown', label: 'Unknown' },
  { value: 'male', label: 'Male' },
  { value: 'female', label: 'Female' },
]

// Matches the children.race_ethnicity CHECK constraint exactly -- a
// fixed set, not free text, and multivalued (a child can be more than
// one), hence checkboxes rather than a single <select> in ChildForm.
export const RACE_ETHNICITY_OPTIONS = [
  { value: 'american_indian_or_alaska_native', label: 'American Indian or Alaska Native' },
  { value: 'asian', label: 'Asian' },
  { value: 'black_or_african_american', label: 'Black or African American' },
  { value: 'hispanic_or_latino', label: 'Hispanic or Latino' },
  { value: 'middle_eastern_or_north_african', label: 'Middle Eastern or North African' },
  { value: 'native_hawaiian_or_pacific_islander', label: 'Native Hawaiian or Pacific Islander' },
  { value: 'white', label: 'White' },
]

export const EDUCATION_OPTIONS = [
  { value: 'unknown', label: 'Unknown' },
  { value: 'without_high_school_diploma', label: 'Without high school diploma' },
  { value: 'hs_grad_no_college', label: 'High school graduate, no college' },
  { value: 'hs_grad_some_college', label: 'High school graduate, some college' },
  { value: 'degree_from_4yr_college_or_higher', label: '4-year degree or higher' },
  { value: 'left_blank', label: 'Left blank' },
]
