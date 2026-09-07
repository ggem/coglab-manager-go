package main

import "testing"

func TestMapContactMethod(t *testing.T) {
	if got := mapContactMethod("Home Phone"); got == nil || *got != "home_phone" {
		t.Errorf("mapContactMethod(Home Phone) = %v, want home_phone", got)
	}
	if got := mapContactMethod("Unknown"); got != nil {
		t.Errorf("mapContactMethod(Unknown) = %v, want nil", got)
	}
	if got := mapContactMethod(""); got != nil {
		t.Errorf("mapContactMethod(\"\") = %v, want nil", got)
	}
}

func TestMapEducation(t *testing.T) {
	cases := map[string]string{
		"Unknown":                                "unknown",
		"Without High School Diploma":            "without_high_school_diploma",
		"HS Grad, No College":                    "hs_grad_no_college",
		"HS Grad, Some College":                  "hs_grad_some_college",
		"Degree From a 4 Year College or Higher": "degree_from_4yr_college_or_higher",
		"Left Blank":                             "left_blank",
	}
	for legacy, want := range cases {
		got, err := mapEducation(legacy)
		if err != nil {
			t.Errorf("mapEducation(%q) error: %v", legacy, err)
			continue
		}
		if got != want {
			t.Errorf("mapEducation(%q) = %q, want %q", legacy, got, want)
		}
	}
	if _, err := mapEducation("Something Else"); err == nil {
		t.Error("expected an error for an unrecognized education value")
	}
}

func TestMapPhoneType(t *testing.T) {
	if got := mapPhoneType(" (Home)"); got == nil || *got != "home" {
		t.Errorf("mapPhoneType(\" (Home)\") = %v, want home", got)
	}
	if got := mapPhoneType(""); got != nil {
		t.Errorf("mapPhoneType(\"\") = %v, want nil", got)
	}
}
