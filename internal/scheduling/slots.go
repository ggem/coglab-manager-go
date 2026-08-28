package scheduling

import "time"

// Time-grid constants, matching the legacy engine's defaults: a day runs
// from 06:00 for 14 hours, in 5-minute slots.
const (
	SlotDuration = 5 * time.Minute
	DayStart     = 6 * time.Hour
	NumSlots     = 168 // 14 hours / 5-minute slots
)

// Slots represents one day's availability as a boolean per 5-minute slot
// starting at DayStart: Slots[i] covers [DayStart+i*SlotDuration,
// DayStart+(i+1)*SlotDuration). true means available.
type Slots [NumSlots]bool

// AllAvailable returns a Slots with every slot marked available -- the
// identity value for IntersectAll, and the natural starting point before
// subtracting anything.
func AllAvailable() Slots {
	var s Slots
	for i := range s {
		s[i] = true
	}
	return s
}

// TimeRange is a half-open interval [Start, End) within a day, expressed
// as a duration since midnight so it doesn't need a specific date. Used
// both for "available during this window" facts and "busy during this
// window" facts -- the field names carry the meaning, the shape is the
// same either way.
type TimeRange struct {
	Start, End time.Duration
}

func slotIndex(t time.Duration) int {
	return int((t - DayStart) / SlotDuration)
}

func clampSlot(i int) int {
	switch {
	case i < 0:
		return 0
	case i > NumSlots:
		return NumSlots
	default:
		return i
	}
}

func (tr TimeRange) slotBounds() (start, end int) {
	return clampSlot(slotIndex(tr.Start)), clampSlot(slotIndex(tr.End))
}

// MarkAvailable sets every slot in timeRange to true.
func (s *Slots) MarkAvailable(tr TimeRange) {
	start, end := tr.slotBounds()
	for i := start; i < end; i++ {
		s[i] = true
	}
}

// MarkUnavailable sets every slot in timeRange to false -- for subtracting busy
// times (already-booked appointments) or blockings from an otherwise
// available schedule.
func (s *Slots) MarkUnavailable(tr TimeRange) {
	start, end := tr.slotBounds()
	for i := start; i < end; i++ {
		s[i] = false
	}
}

// MemberDaySchedule builds one member's availability for a single day.
// specific overrides general entirely when non-empty (matches the
// lab_availability_specific override semantics: if a member has declared
// any specific-date windows for this date, their general weekly schedule
// isn't consulted at all for that date). busy subtracts time ranges the
// member is already committed to (another Pending appointment).
func MemberDaySchedule(general, specific, busy []TimeRange) Slots {
	windows := general
	if len(specific) > 0 {
		windows = specific
	}
	var s Slots
	for _, w := range windows {
		s.MarkAvailable(w)
	}
	for _, b := range busy {
		s.MarkUnavailable(b)
	}
	return s
}

// EquipmentDaySchedule builds one equipment type's availability for a
// day: quantity units exist in total, and a slot remains available as
// long as no more than quantity of the busy (already-booked) ranges
// overlap it.
func EquipmentDaySchedule(quantity int, busy []TimeRange) Slots {
	var counts [NumSlots]int
	for i := range counts {
		counts[i] = quantity
	}
	for _, b := range busy {
		start, end := b.slotBounds()
		for i := start; i < end; i++ {
			counts[i]--
		}
	}
	var s Slots
	for i, c := range counts {
		s[i] = c >= 0
	}
	return s
}

// MergeRoleAvailability OR-merges several members' day schedules into
// one: a slot is available if at least one of them is free then --
// "is someone qualified for this role free at this time."
func MergeRoleAvailability(members []Slots) Slots {
	var merged Slots
	for _, m := range members {
		for i := range merged {
			if m[i] {
				merged[i] = true
			}
		}
	}
	return merged
}

// IntersectAll AND-merges several day schedules into one: a slot is
// available only if every input has it available. Used to combine every
// required role's and equipment's per-day availability into one overall
// vector.
func IntersectAll(schedules ...Slots) Slots {
	merged := AllAvailable()
	for _, s := range schedules {
		for i := range merged {
			if !s[i] {
				merged[i] = false
			}
		}
	}
	return merged
}

// requiredSlots is how many consecutive slots a duration needs, rounded
// up -- shared between TrimForDuration (the coarse per-day grid) and the
// exact per-member check in SearchAvailability, so both agree on exactly
// the same definition of "long enough."
func requiredSlots(duration time.Duration) int {
	n := int((duration + SlotDuration - 1) / SlotDuration)
	if n < 1 {
		return 1
	}
	return n
}

// TrimForDuration clears any slot that doesn't have enough consecutive
// available slots starting there to fit the given duration -- a slot
// surviving this is a valid *start* time for an appointment of that
// length, not just itself free.
func TrimForDuration(s Slots, duration time.Duration) Slots {
	return trimSlots(s, requiredSlots(duration))
}

func trimSlots(s Slots, timeSlots int) Slots {
	run := 0
	for i := NumSlots - 1; i >= 0; i-- {
		if s[i] {
			run++
		} else {
			run = 0
		}
		if run < timeSlots {
			s[i] = false
		}
	}
	return s
}

// hasConsecutiveAvailable reports whether s has count consecutive
// available slots starting at start -- the same "long enough" question
// TrimForDuration answers for a merged grid, asked here of one member's
// individual schedule during the exact backtracking search.
func hasConsecutiveAvailable(s Slots, start, count int) bool {
	if start < 0 || start+count > NumSlots {
		return false
	}
	for i := start; i < start+count; i++ {
		if !s[i] {
			return false
		}
	}
	return true
}

func slotToTime(slot int) time.Duration {
	return DayStart + time.Duration(slot)*SlotDuration
}
