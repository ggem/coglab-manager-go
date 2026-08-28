package scheduling

import (
	"testing"
	"time"
)

func countTrue(s Slots) int {
	n := 0
	for _, v := range s {
		if v {
			n++
		}
	}
	return n
}

func TestMarkAvailable_ClampsToGrid(t *testing.T) {
	var s Slots
	// Starts before DayStart (06:00) and ends after the grid's end
	// (06:00+14h=20:00) -- both ends should clamp rather than panic or
	// silently do nothing.
	s.MarkAvailable(TimeRange{Start: 0, End: 24 * time.Hour})
	for i, v := range s {
		if !v {
			t.Fatalf("slot %d not marked available after a full-day range", i)
		}
	}
}

func TestMarkAvailable_ExactSlot(t *testing.T) {
	var s Slots
	// 09:00-09:05 is exactly slot index (9h-6h)/5m = 36.
	s.MarkAvailable(TimeRange{Start: 9 * time.Hour, End: 9*time.Hour + 5*time.Minute})
	if countTrue(s) != 1 || !s[36] {
		t.Fatalf("expected only slot 36 true, got %d true slots, s[36]=%v", countTrue(s), s[36])
	}
}

func TestMarkUnavailable_SubtractsFromAvailable(t *testing.T) {
	s := AllAvailable()
	s.MarkUnavailable(TimeRange{Start: 9 * time.Hour, End: 10 * time.Hour})
	// 9:00-10:00 is 12 slots (60min/5min).
	if got := NumSlots - countTrue(s); got != 12 {
		t.Errorf("cleared %d slots, want 12", got)
	}
}

func TestMemberDaySchedule_SpecificOverridesGeneral(t *testing.T) {
	general := []TimeRange{{Start: 9 * time.Hour, End: 17 * time.Hour}}
	specific := []TimeRange{{Start: 10 * time.Hour, End: 11 * time.Hour}}

	s := MemberDaySchedule(general, specific, nil)

	// Only the specific window should be available -- general is ignored
	// entirely once any specific override exists for the date.
	want := 12 // 10:00-11:00 = 12 slots
	if got := countTrue(s); got != want {
		t.Errorf("countTrue = %d, want %d (specific should fully replace general)", got, want)
	}
}

func TestMemberDaySchedule_GeneralUsedWhenNoSpecific(t *testing.T) {
	general := []TimeRange{{Start: 9 * time.Hour, End: 10 * time.Hour}}

	s := MemberDaySchedule(general, nil, nil)

	if got := countTrue(s); got != 12 {
		t.Errorf("countTrue = %d, want 12", got)
	}
}

func TestMemberDaySchedule_BusySubtractsFromWindows(t *testing.T) {
	general := []TimeRange{{Start: 9 * time.Hour, End: 11 * time.Hour}} // 24 slots
	busy := []TimeRange{{Start: 9 * time.Hour, End: 10 * time.Hour}}    // 12 slots

	s := MemberDaySchedule(general, nil, busy)

	if got := countTrue(s); got != 12 {
		t.Errorf("countTrue = %d, want 12 (24 available minus 12 busy)", got)
	}
}

func TestEquipmentDaySchedule_WithinQuantity(t *testing.T) {
	busy := []TimeRange{
		{Start: 9 * time.Hour, End: 10 * time.Hour},
		{Start: 9*time.Hour + 30*time.Minute, End: 10*time.Hour + 30*time.Minute},
	}
	s := EquipmentDaySchedule(2, busy)

	// Both overlapping bookings fit within quantity=2, so the whole
	// overlapping window should remain available.
	if !hasConsecutiveAvailable(s, slotIndex(9*time.Hour), requiredSlots(90*time.Minute)) {
		t.Error("expected 9:00-10:30 to remain available with quantity=2 and two overlapping bookings")
	}
}

func TestEquipmentDaySchedule_ExceedsQuantity(t *testing.T) {
	busy := []TimeRange{
		{Start: 9 * time.Hour, End: 10 * time.Hour},
		{Start: 9*time.Hour + 15*time.Minute, End: 10*time.Hour + 15*time.Minute},
	}
	s := EquipmentDaySchedule(1, busy)

	// The overlap region (9:15-10:00) exceeds quantity=1 and must be
	// unavailable; the non-overlapping edges should still be free.
	overlapStart := slotIndex(9*time.Hour + 15*time.Minute)
	if s[overlapStart] {
		t.Errorf("slot %d (in the double-booked overlap) should be unavailable with quantity=1", overlapStart)
	}
	edgeStart := slotIndex(9 * time.Hour)
	if !s[edgeStart] {
		t.Errorf("slot %d (single-booked edge) should still be available with quantity=1", edgeStart)
	}
}

func TestMergeRoleAvailability_ORsMembers(t *testing.T) {
	var a, b Slots
	a.MarkAvailable(TimeRange{Start: 9 * time.Hour, End: 10 * time.Hour})
	b.MarkAvailable(TimeRange{Start: 11 * time.Hour, End: 12 * time.Hour})

	merged := MergeRoleAvailability([]Slots{a, b})

	if !merged[slotIndex(9*time.Hour)] || !merged[slotIndex(11*time.Hour)] {
		t.Error("merged availability should include both members' windows")
	}
	if merged[slotIndex(10*time.Hour)] {
		t.Error("a slot neither member is free at should stay unavailable")
	}
}

func TestIntersectAll_RequiresEveryInput(t *testing.T) {
	a := AllAvailable()
	b := AllAvailable()
	b.MarkUnavailable(TimeRange{Start: 9 * time.Hour, End: 10 * time.Hour})

	merged := IntersectAll(a, b)

	if merged[slotIndex(9*time.Hour)] {
		t.Error("intersection should exclude a slot any single input excludes")
	}
	if !merged[slotIndex(11*time.Hour)] {
		t.Error("intersection should keep a slot every input has available")
	}
}

func TestIntersectAll_NoInputsIsAllAvailable(t *testing.T) {
	merged := IntersectAll()
	if countTrue(merged) != NumSlots {
		t.Error("IntersectAll with no inputs should be the identity (all available)")
	}
}

func TestTrimForDuration_ExactMultiple(t *testing.T) {
	var s Slots
	s.MarkAvailable(TimeRange{Start: 9 * time.Hour, End: 9*time.Hour + 15*time.Minute}) // 3 slots

	trimmed := TrimForDuration(s, 15*time.Minute) // needs exactly 3 slots

	start := slotIndex(9 * time.Hour)
	if !trimmed[start] {
		t.Error("the only valid start in a run exactly the required length should survive")
	}
	if countTrue(trimmed) != 1 {
		t.Errorf("countTrue = %d, want 1 (only the single valid start slot)", countTrue(trimmed))
	}
}

func TestTrimForDuration_RoundsUpNonMultiple(t *testing.T) {
	var s Slots
	// A 12-minute duration needs ceil(12/5)=3 slots, same as a 15-minute
	// one -- this is the "duration doesn't evenly divide the grid" case.
	s.MarkAvailable(TimeRange{Start: 9 * time.Hour, End: 9*time.Hour + 15*time.Minute}) // 3 slots

	trimmed := TrimForDuration(s, 12*time.Minute)

	if countTrue(trimmed) != 1 {
		t.Errorf("countTrue = %d, want 1 -- a 12-minute duration should round up to needing 3 slots, same as 15 minutes", countTrue(trimmed))
	}
}

func TestTrimForDuration_RunTooShortClearsEntirely(t *testing.T) {
	var s Slots
	s.MarkAvailable(TimeRange{Start: 9 * time.Hour, End: 9*time.Hour + 10*time.Minute}) // 2 slots

	trimmed := TrimForDuration(s, 15*time.Minute) // needs 3 slots, only 2 available

	if countTrue(trimmed) != 0 {
		t.Errorf("countTrue = %d, want 0 -- a run shorter than the required duration must be entirely cleared", countTrue(trimmed))
	}
}

func TestRequiredSlots_ZeroOrNegativeFloorsAtOne(t *testing.T) {
	if got := requiredSlots(0); got != 1 {
		t.Errorf("requiredSlots(0) = %d, want 1", got)
	}
	if got := requiredSlots(-time.Minute); got != 1 {
		t.Errorf("requiredSlots(-1m) = %d, want 1 (a nonsensical negative duration shouldn't produce a non-positive requirement)", got)
	}
}

func TestHasConsecutiveAvailable_NegativeStart(t *testing.T) {
	s := AllAvailable()
	if hasConsecutiveAvailable(s, -1, 1) {
		t.Error("a negative start index should never be considered available")
	}
}

func TestTrimForDuration_LongerRunKeepsMultipleValidStarts(t *testing.T) {
	var s Slots
	s.MarkAvailable(TimeRange{Start: 9 * time.Hour, End: 9*time.Hour + 20*time.Minute}) // 4 slots

	trimmed := TrimForDuration(s, 15*time.Minute) // needs 3 slots

	// A run of 4 with a requirement of 3 has 2 valid starts (positions 0
	// and 1 within the run), and the last 2 positions (which don't have
	// enough room after them) should be cleared.
	start := slotIndex(9 * time.Hour)
	if !trimmed[start] || !trimmed[start+1] {
		t.Error("expected the first two slots of a 4-slot run to remain valid starts for a 3-slot duration")
	}
	if trimmed[start+2] || trimmed[start+3] {
		t.Error("expected the last two slots of the run to be cleared (not enough room after them)")
	}
}
