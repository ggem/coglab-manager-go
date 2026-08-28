package scheduling

import (
	"testing"
	"time"
)

func availableAllDay() Slots {
	return AllAvailable()
}

func availableWindow(tr TimeRange) Slots {
	var s Slots
	s.MarkAvailable(tr)
	return s
}

func TestSearchAvailability_ZeroCandidatesForARole(t *testing.T) {
	days := []DayAvailability{{Date: time.Now()}}
	roles := []RoleCandidatesForSearch{
		{
			RoleCandidates: RoleCandidates{RoleID: 1, MemberIDs: nil}, // no one qualified
			Available:      map[int64][]Slots{},
		},
	}

	results := SearchAvailability(days, roles, nil, SitterNotNeeded, 30*time.Minute)

	if len(results) != 0 {
		t.Fatalf("results = %+v, want none -- a role with zero candidates can never be filled", results)
	}
}

func TestSearchAvailability_SitterRequiredButUnavailable_Fails(t *testing.T) {
	days := []DayAvailability{{Date: time.Now()}}
	roles := []RoleCandidatesForSearch{
		{
			RoleCandidates: RoleCandidates{RoleID: 1, MemberIDs: []int64{10}},
			Available:      map[int64][]Slots{10: {availableAllDay()}},
		},
	}
	sitterRole := &RoleCandidatesForSearch{
		RoleCandidates: RoleCandidates{RoleID: 2, MemberIDs: []int64{20}},
		Available:      map[int64][]Slots{20: {{}}}, // present, but available nowhere
	}

	results := SearchAvailability(days, roles, sitterRole, SitterRequired, 30*time.Minute)

	if len(results) != 0 {
		t.Fatalf("results = %+v, want none -- sitter required but no sitter is ever free", results)
	}
}

func TestSearchAvailability_SitterSoftButUnavailable_StillSucceeds(t *testing.T) {
	days := []DayAvailability{{Date: time.Now()}}
	roles := []RoleCandidatesForSearch{
		{
			RoleCandidates: RoleCandidates{RoleID: 1, MemberIDs: []int64{10}},
			Available:      map[int64][]Slots{10: {availableAllDay()}},
		},
	}
	sitterRole := &RoleCandidatesForSearch{
		RoleCandidates: RoleCandidates{RoleID: 2, MemberIDs: []int64{20}},
		Available:      map[int64][]Slots{20: {{}}}, // never free
	}

	results := SearchAvailability(days, roles, sitterRole, SitterSoft, 30*time.Minute)

	if len(results) == 0 {
		t.Fatal("expected candidate slots even without a sitter, since the sitter is only a soft requirement")
	}
	for _, r := range results {
		if r.HasSitter {
			t.Errorf("slot %v: HasSitter = true, but no sitter was ever available", r.StartTime)
		}
	}
}

func TestSearchAvailability_SitterRequiredButNoSitterRoleConfigured_Fails(t *testing.T) {
	// A sitter is required (sibling_coming = 'coming') but the lab has no
	// sitter role designated at all (sitterRole == nil) -- this must
	// never be silently treated as "sitter not needed."
	days := []DayAvailability{{Date: time.Now()}}
	roles := []RoleCandidatesForSearch{
		{
			RoleCandidates: RoleCandidates{RoleID: 1, MemberIDs: []int64{10}},
			Available:      map[int64][]Slots{10: {availableAllDay()}},
		},
	}

	results := SearchAvailability(days, roles, nil, SitterRequired, 30*time.Minute)

	if len(results) != 0 {
		t.Fatalf("results = %+v, want none -- a required sitter with no sitter role configured can never be satisfied", results)
	}
}

func TestSearchAvailability_SitterSoftAndAvailable_PrefersWithSitter(t *testing.T) {
	days := []DayAvailability{{Date: time.Now()}}
	window := TimeRange{Start: 9 * time.Hour, End: 10 * time.Hour}
	roles := []RoleCandidatesForSearch{
		{
			RoleCandidates: RoleCandidates{RoleID: 1, MemberIDs: []int64{10}},
			Available:      map[int64][]Slots{10: {availableWindow(window)}},
		},
	}
	sitterRole := &RoleCandidatesForSearch{
		RoleCandidates: RoleCandidates{RoleID: 2, MemberIDs: []int64{20}},
		Available:      map[int64][]Slots{20: {availableWindow(window)}},
	}

	results := SearchAvailability(days, roles, sitterRole, SitterSoft, 30*time.Minute)

	if len(results) == 0 {
		t.Fatal("expected candidate slots")
	}
	for _, r := range results {
		if !r.HasSitter {
			t.Errorf("slot %v: HasSitter = false, but a sitter was available -- should be preferred", r.StartTime)
		}
		if _, ok := r.Assignment[2]; !ok {
			t.Errorf("slot %v: sitter role (2) missing from assignment despite HasSitter=true", r.StartTime)
		}
	}
}

func TestSearchAvailability_ExactBoundaryWindow(t *testing.T) {
	// Member available for exactly one duration's worth of time -- only
	// the single slot at the window's start should come back.
	days := []DayAvailability{{Date: time.Now()}}
	window := TimeRange{Start: 9 * time.Hour, End: 9*time.Hour + 30*time.Minute}
	roles := []RoleCandidatesForSearch{
		{
			RoleCandidates: RoleCandidates{RoleID: 1, MemberIDs: []int64{10}},
			Available:      map[int64][]Slots{10: {availableWindow(window)}},
		},
	}

	results := SearchAvailability(days, roles, nil, SitterNotNeeded, 30*time.Minute)

	if len(results) != 1 {
		t.Fatalf("got %d results, want exactly 1", len(results))
	}
	if results[0].StartTime != 9*time.Hour {
		t.Errorf("StartTime = %v, want 09:00", results[0].StartTime)
	}
}

func TestSearchAvailability_DurationDoesNotEvenlyDivideGrid(t *testing.T) {
	// A 22-minute duration needs ceil(22/5)=5 slots (25 minutes of room),
	// same requirement as a 25-minute duration would have.
	days := []DayAvailability{{Date: time.Now()}}

	fitsExactly := TimeRange{Start: 9 * time.Hour, End: 9*time.Hour + 25*time.Minute}
	roles := []RoleCandidatesForSearch{
		{
			RoleCandidates: RoleCandidates{RoleID: 1, MemberIDs: []int64{10}},
			Available:      map[int64][]Slots{10: {availableWindow(fitsExactly)}},
		},
	}
	if got := SearchAvailability(days, roles, nil, SitterNotNeeded, 22*time.Minute); len(got) != 1 {
		t.Errorf("25-minute window with a 22-minute duration: got %d results, want 1", len(got))
	}

	tooShort := TimeRange{Start: 9 * time.Hour, End: 9*time.Hour + 20*time.Minute} // only 4 slots
	roles[0].Available[10] = []Slots{availableWindow(tooShort)}
	if got := SearchAvailability(days, roles, nil, SitterNotNeeded, 22*time.Minute); len(got) != 0 {
		t.Errorf("20-minute window with a 22-minute duration (rounds up to needing 25min): got %d results, want 0", len(got))
	}
}

func TestSearchAvailability_EquipmentConstrains(t *testing.T) {
	days := []DayAvailability{{
		Date: time.Now(),
		EquipmentSlots: []Slots{
			availableWindow(TimeRange{Start: 9 * time.Hour, End: 9*time.Hour + 30*time.Minute}),
		},
	}}
	roles := []RoleCandidatesForSearch{
		{
			RoleCandidates: RoleCandidates{RoleID: 1, MemberIDs: []int64{10}},
			Available:      map[int64][]Slots{10: {availableAllDay()}},
		},
	}

	results := SearchAvailability(days, roles, nil, SitterNotNeeded, 30*time.Minute)

	if len(results) != 1 || results[0].StartTime != 9*time.Hour {
		t.Fatalf("results = %+v, want exactly the equipment's 09:00 window", results)
	}
}

func TestSearchAvailability_BlockingRemovesSlot(t *testing.T) {
	days := []DayAvailability{{
		Date:    time.Now(),
		Blocked: []TimeRange{{Start: 9 * time.Hour, End: 12 * time.Hour}},
	}}
	roles := []RoleCandidatesForSearch{
		{
			RoleCandidates: RoleCandidates{RoleID: 1, MemberIDs: []int64{10}},
			Available:      map[int64][]Slots{10: {availableWindow(TimeRange{Start: 9 * time.Hour, End: 10 * time.Hour})}},
		},
	}

	results := SearchAvailability(days, roles, nil, SitterNotNeeded, 30*time.Minute)

	if len(results) != 0 {
		t.Fatalf("results = %+v, want none -- the only available window is fully blocked", results)
	}
}

func TestSearchAvailability_MultipleDaysIndexedCorrectly(t *testing.T) {
	day0 := time.Now()
	day1 := day0.AddDate(0, 0, 1)
	days := []DayAvailability{{Date: day0}, {Date: day1}}

	window := TimeRange{Start: 9 * time.Hour, End: 10 * time.Hour}
	roles := []RoleCandidatesForSearch{
		{
			RoleCandidates: RoleCandidates{RoleID: 1, MemberIDs: []int64{10}},
			// Only free on day 1 (index 1), not day 0.
			Available: map[int64][]Slots{10: {{}, availableWindow(window)}},
		},
	}

	results := SearchAvailability(days, roles, nil, SitterNotNeeded, 30*time.Minute)

	if len(results) == 0 {
		t.Fatal("expected results on day 1")
	}
	for _, r := range results {
		if !r.Date.Equal(day1) {
			t.Errorf("result on %v, want only day1 (%v)", r.Date, day1)
		}
	}
}

func TestSearchAvailability_TwoRolesProduceCompleteAssignment(t *testing.T) {
	days := []DayAvailability{{Date: time.Now()}}
	window := TimeRange{Start: 9 * time.Hour, End: 10 * time.Hour}
	roles := []RoleCandidatesForSearch{
		{
			RoleCandidates: RoleCandidates{RoleID: 1, MemberIDs: []int64{10}},
			Available:      map[int64][]Slots{10: {availableWindow(window)}},
		},
		{
			RoleCandidates: RoleCandidates{RoleID: 2, MemberIDs: []int64{20}},
			Available:      map[int64][]Slots{20: {availableWindow(window)}},
		},
	}

	results := SearchAvailability(days, roles, nil, SitterNotNeeded, 30*time.Minute)

	if len(results) == 0 {
		t.Fatal("expected results")
	}
	for _, r := range results {
		if r.Assignment[1] != 10 || r.Assignment[2] != 20 {
			t.Errorf("assignment = %+v, want {1:10, 2:20}", r.Assignment)
		}
		if r.GreeterID != 10 && r.GreeterID != 20 {
			t.Errorf("GreeterID = %d, want one of the assigned members", r.GreeterID)
		}
	}
}
