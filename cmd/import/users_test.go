package main

import "testing"

func TestRoleForAuthLevel(t *testing.T) {
	cases := []struct {
		authLevel int64
		want      string
	}{
		{0, "staff"},
		{1, "staff"},
		{2, "staff"},
		{3, "coordinator"},
		{4, "admin"},
		{5, "admin"}, // never observed, but shouldn't panic or downgrade
	}
	for _, c := range cases {
		if got := roleForAuthLevel(c.authLevel); got != c.want {
			t.Errorf("roleForAuthLevel(%d) = %q, want %q", c.authLevel, got, c.want)
		}
	}
}

func TestMapPriority(t *testing.T) {
	cases := []struct {
		legacy string
		want   string
	}{
		{"Undergrad/Community Volunteer w/o independent project", "undergrad_no_project"},
		{"Undergrad with independent project", "undergrad_with_project"},
		{"Lab coordinator", "lab_coordinator"},
		{"Graduate Student", "graduate_student"},
		{"Postdoc", "postdoc"},
		{"Lab Director", "lab_director"},
	}
	for _, c := range cases {
		got, err := mapPriority(c.legacy)
		if err != nil {
			t.Errorf("mapPriority(%q) error: %v", c.legacy, err)
			continue
		}
		if got != c.want {
			t.Errorf("mapPriority(%q) = %q, want %q", c.legacy, got, c.want)
		}
	}
}

func TestMapPriority_Unrecognized(t *testing.T) {
	if _, err := mapPriority("Something Else"); err == nil {
		t.Error("expected an error for an unrecognized priority value")
	}
}
