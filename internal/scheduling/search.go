package scheduling

import "time"

// DayAvailability is one candidate day's pre-computed availability: each
// required equipment type's day schedule (built via EquipmentDaySchedule)
// and that day's lab-wide blocked time ranges. Role availability isn't
// stored here -- it's derived from RoleCandidatesForSearch's per-member
// Available data instead, so there's exactly one source of truth for it,
// used both by the coarse per-day filter and the exact per-member check.
type DayAvailability struct {
	Date           time.Time
	EquipmentSlots []Slots
	Blocked        []TimeRange
}

// RoleCandidatesForSearch is RoleCandidates plus each candidate's
// availability across the whole search: one Slots per day, in the same
// order as the days passed to SearchAvailability.
type RoleCandidatesForSearch struct {
	RoleCandidates
	Available map[int64][]Slots // user ID -> per-day Slots
}

// SitterRequirement controls whether/how the search accounts for the
// sitter role, driven by an appointment's sibling_coming value.
type SitterRequirement int

const (
	// SitterNotNeeded skips the sitter role entirely (sibling_coming =
	// 'not_coming').
	SitterNotNeeded SitterRequirement = iota
	// SitterSoft tries to find a sitter but doesn't fail the whole search
	// if none is free (sibling_coming = 'unknown' -- not yet confirmed).
	SitterSoft
	// SitterRequired fails a slot outright if no sitter is free
	// (sibling_coming = 'coming').
	SitterRequired
)

// CandidateSlot is one date/time the search found workable, with the
// staff assignment it would produce if chosen.
type CandidateSlot struct {
	Date       time.Time
	StartTime  time.Duration
	Assignment Assignment
	GreeterID  int64
	HasSitter  bool
}

// SearchAvailability finds every date/time across days when a valid
// staff assignment exists for an appointment of the given duration,
// requiring every role in roles plus (depending on sitterRequirement) the
// sitter role. For each day it first computes a coarse merged-availability
// grid (cheap OR/AND-merge across all candidates, trimmed for duration) to
// find which slots are worth trying at all, then runs the exact
// backtracking search (FindAssignment) only on those.  See the package
// doc for why both phases exist.
func SearchAvailability(
	days []DayAvailability,
	roles []RoleCandidatesForSearch,
	sitterRole *RoleCandidatesForSearch,
	sitterRequirement SitterRequirement,
	duration time.Duration,
) []CandidateSlot {
	// A sitter is required but there's no sitter role to draw candidates
	// from at all -- that can never be satisfied, so there's nothing to
	// search for. Guarding explicitly here (rather than letting the
	// sitterRole==nil checks below just skip the requirement) avoids
	// silently ignoring a hard requirement the caller can't fulfill.
	if sitterRequirement == SitterRequired && sitterRole == nil {
		return nil
	}

	timeSlots := requiredSlots(duration)

	plainRoles, withSitterRoles := plainAndWithSitter(roles, sitterRole)
	plainLookup := availabilityLookup(plainRoles)
	sitterLookup := availabilityLookup(withSitterRoles)

	var results []CandidateSlot
	for dayIdx, day := range days {
		grid := dayGrid(day, dayIdx, roles, sitterRole, sitterRequirement, timeSlots)

		for slot := range NumSlots {
			if !grid[slot] {
				continue
			}

			if sitterRole != nil && sitterRequirement != SitterNotNeeded {
				isAvail := availabilityCheck(sitterLookup, dayIdx, slot, timeSlots)
				if assignment, ok := FindAssignment(roleCandidates(withSitterRoles), isAvail); ok {
					results = append(results, toCandidateSlot(day.Date, slot, assignment, true))
					continue
				}
				if sitterRequirement == SitterRequired {
					continue
				}
			}

			isAvail := availabilityCheck(plainLookup, dayIdx, slot, timeSlots)
			if assignment, ok := FindAssignment(roleCandidates(plainRoles), isAvail); ok {
				results = append(results, toCandidateSlot(day.Date, slot, assignment, false))
			}
		}
	}
	return results
}

func plainAndWithSitter(roles []RoleCandidatesForSearch, sitterRole *RoleCandidatesForSearch) (plain, withSitter []RoleCandidatesForSearch) {
	if sitterRole == nil {
		return roles, roles
	}
	withSitter = make([]RoleCandidatesForSearch, 0, len(roles)+1)
	withSitter = append(withSitter, roles...)
	withSitter = append(withSitter, *sitterRole)
	return roles, withSitter
}

func roleCandidates(roles []RoleCandidatesForSearch) []RoleCandidates {
	out := make([]RoleCandidates, len(roles))
	for i, r := range roles {
		out[i] = r.RoleCandidates
	}
	return out
}

func dayGrid(
	day DayAvailability, dayIdx int,
	roles []RoleCandidatesForSearch, sitterRole *RoleCandidatesForSearch,
	sitterRequirement SitterRequirement, timeSlots int,
) Slots {
	grid := AllAvailable()
	for _, role := range roles {
		grid = IntersectAll(grid, mergedRoleSlotsForDay(role, dayIdx))
	}
	if sitterRole != nil && sitterRequirement == SitterRequired {
		grid = IntersectAll(grid, mergedRoleSlotsForDay(*sitterRole, dayIdx))
	}
	for _, eq := range day.EquipmentSlots {
		grid = IntersectAll(grid, eq)
	}
	for _, b := range day.Blocked {
		grid.MarkUnavailable(b)
	}
	return trimSlots(grid, timeSlots)
}

func mergedRoleSlotsForDay(role RoleCandidatesForSearch, dayIdx int) Slots {
	members := make([]Slots, 0, len(role.MemberIDs))
	for _, id := range role.MemberIDs {
		if s := role.Available[id]; dayIdx < len(s) {
			members = append(members, s[dayIdx])
		}
	}
	return MergeRoleAvailability(members)
}

// availabilityLookup indexes roleID -> userID -> per-day Slots, so the
// exact backtracking search can look up any (role, candidate) pair's
// availability directly instead of re-scanning RoleCandidatesForSearch.
func availabilityLookup(roles []RoleCandidatesForSearch) map[int64]map[int64][]Slots {
	lookup := make(map[int64]map[int64][]Slots, len(roles))
	for _, r := range roles {
		lookup[r.RoleID] = r.Available
	}
	return lookup
}

func availabilityCheck(lookup map[int64]map[int64][]Slots, dayIdx, slot, timeSlots int) func(userID, roleID int64) bool {
	return func(userID, roleID int64) bool {
		daySlots := lookup[roleID][userID]
		if dayIdx >= len(daySlots) {
			return false
		}
		return hasConsecutiveAvailable(daySlots[dayIdx], slot, timeSlots)
	}
}

func toCandidateSlot(date time.Time, slot int, assignment Assignment, hasSitter bool) CandidateSlot {
	greeter, _ := DesignateGreeter(assignment)
	return CandidateSlot{
		Date:       date,
		StartTime:  slotToTime(slot),
		Assignment: assignment,
		GreeterID:  greeter,
		HasSitter:  hasSitter,
	}
}
