package scheduling

import "testing"

func TestFindAssignment_Success(t *testing.T) {
	roles := []RoleCandidates{
		{RoleID: 1, MemberIDs: []int64{10, 11}},
		{RoleID: 2, MemberIDs: []int64{20, 21}},
	}
	available := map[int64]bool{10: true, 11: true, 20: true, 21: true}

	assignment, ok := FindAssignment(roles, func(userID, roleID int64) bool { return available[userID] })

	if !ok {
		t.Fatal("expected a valid assignment")
	}
	if len(assignment) != 2 {
		t.Fatalf("assignment = %+v, want 2 entries", assignment)
	}
	if assignment[1] == assignment[2] {
		t.Errorf("the same member (%d) was assigned to both roles", assignment[1])
	}
}

func TestFindAssignment_NoCandidatesForARole(t *testing.T) {
	roles := []RoleCandidates{
		{RoleID: 1, MemberIDs: []int64{10}},
		{RoleID: 2, MemberIDs: nil}, // no one qualified at all
	}

	_, ok := FindAssignment(roles, func(userID, roleID int64) bool { return true })

	if ok {
		t.Fatal("expected no assignment when a role has zero candidates")
	}
}

func TestFindAssignment_BacktracksWhenOnlySharedCandidate(t *testing.T) {
	// Both roles can only be filled by member 10 -- the search must try
	// role 1 = member 10, then find role 2 has no one left (10 is busy),
	// and correctly report failure rather than double-booking member 10.
	roles := []RoleCandidates{
		{RoleID: 1, MemberIDs: []int64{10}},
		{RoleID: 2, MemberIDs: []int64{10}},
	}

	_, ok := FindAssignment(roles, func(userID, roleID int64) bool { return true })

	if ok {
		t.Fatal("expected failure: only one member is available for two roles that both need someone")
	}
}

func TestFindAssignment_BacktracksToLaterCandidate(t *testing.T) {
	// Role 1 prefers member 10 first, but 10 is the *only* candidate for
	// role 2. A correct backtracking search tries role1=10, discovers
	// role 2 has no one left, backtracks, and retries role1=11 -- which
	// then lets role 2 succeed with 10.
	roles := []RoleCandidates{
		{RoleID: 1, MemberIDs: []int64{10, 11}},
		{RoleID: 2, MemberIDs: []int64{10}},
	}

	assignment, ok := FindAssignment(roles, func(userID, roleID int64) bool { return true })

	if !ok {
		t.Fatal("expected the search to backtrack and find role1=11, role2=10")
	}
	if assignment[1] != 11 || assignment[2] != 10 {
		t.Errorf("assignment = %+v, want {1:11, 2:10}", assignment)
	}
}

func TestFindAssignment_RespectsIsAvailable(t *testing.T) {
	roles := []RoleCandidates{
		{RoleID: 1, MemberIDs: []int64{10, 11}},
	}
	// Member 10 is a candidate but not actually available right now.
	isAvailable := func(userID, roleID int64) bool { return userID != 10 }

	assignment, ok := FindAssignment(roles, isAvailable)

	if !ok {
		t.Fatal("expected member 11 to be found")
	}
	if assignment[1] != 11 {
		t.Errorf("assignment[1] = %d, want 11 (10 is unavailable)", assignment[1])
	}
}

func TestFindAssignment_EmptyRolesSucceedsWithEmptyAssignment(t *testing.T) {
	assignment, ok := FindAssignment(nil, func(int64, int64) bool { return true })
	if !ok {
		t.Fatal("no roles to fill should trivially succeed")
	}
	if len(assignment) != 0 {
		t.Errorf("assignment = %+v, want empty", assignment)
	}
}

func TestDesignateGreeter_PicksSmallestRoleDeterministically(t *testing.T) {
	assignment := Assignment{5: 100, 2: 200, 8: 300}

	userID, ok := DesignateGreeter(assignment)

	if !ok {
		t.Fatal("expected a greeter to be designated")
	}
	if userID != 200 {
		t.Errorf("userID = %d, want 200 (role 2 is the smallest role ID)", userID)
	}
}

func TestDesignateGreeter_EmptyAssignment(t *testing.T) {
	_, ok := DesignateGreeter(Assignment{})
	if ok {
		t.Error("expected ok=false for an empty assignment")
	}
}
