package main

import (
	"reflect"
	"testing"
)

func TestSplitGreeter(t *testing.T) {
	hasGreeter, other := splitGreeter([]int64{legacyGreeterRoleID, 5})
	if !hasGreeter || !reflect.DeepEqual(other, []int64{5}) {
		t.Errorf("splitGreeter = %v, %v", hasGreeter, other)
	}
	hasGreeter, other = splitGreeter([]int64{5})
	if hasGreeter || !reflect.DeepEqual(other, []int64{5}) {
		t.Errorf("splitGreeter = %v, %v", hasGreeter, other)
	}
	hasGreeter, other = splitGreeter([]int64{legacyGreeterRoleID})
	if !hasGreeter || other != nil {
		t.Errorf("splitGreeter = %v, %v", hasGreeter, other)
	}
	// Legacy sometimes records the exact same (appointment, member,
	// role) triple twice -- not a real conflict, just a duplicate row.
	hasGreeter, other = splitGreeter([]int64{8, 8})
	if hasGreeter || !reflect.DeepEqual(other, []int64{8}) {
		t.Errorf("splitGreeter(duplicate role) = %v, %v, want false, [8]", hasGreeter, other)
	}
}

func TestResolveExperimenterRole(t *testing.T) {
	sitterByLab := map[int64]int64{1: 801}
	greeterByLab := map[int64]int64{1: 802}

	// A real role passes through untouched.
	got, err := resolveExperimenterRole(10, []int64{5}, 1, true, sitterByLab, greeterByLab)
	if err != nil || got != 5 {
		t.Errorf("real role: got %d, %v; want 5, nil", got, err)
	}

	// The Sitter sentinel remaps to the lab's real Sitter role.
	got, err = resolveExperimenterRole(10, []int64{legacySitterRoleID}, 1, true, sitterByLab, greeterByLab)
	if err != nil || got != 801 {
		t.Errorf("sitter: got %d, %v; want 801, nil", got, err)
	}

	// Greeter-only (no other role at all) resolves to the lab's Greeter role.
	got, err = resolveExperimenterRole(10, nil, 1, true, sitterByLab, greeterByLab)
	if err != nil || got != 802 {
		t.Errorf("greeter-only: got %d, %v; want 802, nil", got, err)
	}

	// More than one non-greeter, non-sitter role is an unresolvable conflict.
	if _, err := resolveExperimenterRole(10, []int64{5, 6}, 1, true, sitterByLab, greeterByLab); err == nil {
		t.Error("expected an error for more than one non-greeter role")
	}

	// No known lab for a greeter-only/sitter resolution is reported, not guessed.
	if _, err := resolveExperimenterRole(10, nil, 0, false, sitterByLab, greeterByLab); err == nil {
		t.Error("expected an error when the appointment's lab isn't known")
	}

	// A lab with no Sitter/Greeter role created is reported, not guessed.
	if _, err := resolveExperimenterRole(10, []int64{legacySitterRoleID}, 99, true, sitterByLab, greeterByLab); err == nil {
		t.Error("expected an error for a lab with no Sitter role created")
	}
}
