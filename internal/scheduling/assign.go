package scheduling

// RoleCandidates is one role to fill and the ordered pool of members
// qualified for it. Order matters: FindAssignment tries roles in the
// order given, and callers should sort roles most-constrained-first
// (fewest candidates first) -- the same search-pruning heuristic the
// legacy engine used, since a role with few candidates is far more likely
// to force a backtracking, and finding that out early avoids wasted work
// deep in the search tree.
type RoleCandidates struct {
	RoleID    int64
	MemberIDs []int64
}

// Assignment maps each filled role to the member assigned to it.
type Assignment map[int64]int64 // role ID -> user ID

// FindAssignment runs the backtracking search for one simultaneous,
// conflict-free assignment of a member to every role: for each role (in
// the order given), try each remaining candidate not already assigned to
// an earlier role in this attempt; isAvailable reports whether a specific
// member can fill a specific role (at whatever slot the caller is
// searching -- this package has no notion of "the current slot" itself,
// see SearchAvailability). Backtracks to the previous role's next
// candidate whenever a role runs out of options. Returns ok=false if no
// valid assignment exists at all.
func FindAssignment(roles []RoleCandidates, isAvailable func(userID, roleID int64) bool) (Assignment, bool) {
	assignment := make(Assignment, len(roles))
	busy := make(map[int64]bool, len(roles))
	if findAssignment(roles, 0, assignment, busy, isAvailable) {
		return assignment, true
	}
	return nil, false
}

func findAssignment(
	roles []RoleCandidates, i int,
	assignment Assignment, busy map[int64]bool,
	isAvailable func(userID, roleID int64) bool,
) bool {
	if i == len(roles) {
		return true
	}
	role := roles[i]
	for _, member := range role.MemberIDs {
		if busy[member] || !isAvailable(member, role.RoleID) {
			continue
		}
		assignment[role.RoleID] = member
		busy[member] = true
		if findAssignment(roles, i+1, assignment, busy, isAvailable) {
			return true
		}
		delete(assignment, role.RoleID)
		delete(busy, member)
	}
	return false
}

// DesignateGreeter picks one assigned member to also serve as Greeter.
// Unlike every other role, Greeter isn't searched for: whoever's already
// assigned to the appointment can step out to meet the family for the
// first few minutes, so any of them is a valid choice -- this just needs
// to be a deterministic one for reproducibility (smallest role ID wins),
// not a meaningful preference. ok is false only if assignment is empty.
func DesignateGreeter(assignment Assignment) (userID int64, ok bool) {
	first := true
	var minRole int64
	for roleID, member := range assignment {
		if first || roleID < minRole {
			minRole, userID = roleID, member
			first = false
		}
	}
	return userID, !first
}
