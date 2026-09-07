package main

import "testing"

func TestMapStatus(t *testing.T) {
	cases := map[string]string{
		"To be scheduled": "to_be_scheduled",
		"Pending":         "pending",
		"Arrived":         "arrived",
		"No Show":         "no_show",
		"Canceled":        "canceled",
		"Problem":         "problem",
		"Released":        "released",
	}
	for legacy, want := range cases {
		got, err := mapStatus(legacy)
		if err != nil {
			t.Errorf("mapStatus(%q) error: %v", legacy, err)
			continue
		}
		if got != want {
			t.Errorf("mapStatus(%q) = %q, want %q", legacy, got, want)
		}
	}
	if _, err := mapStatus("Something Else"); err == nil {
		t.Error("expected an error for an unrecognized status")
	}
}

func TestMapDataStatus(t *testing.T) {
	got, err := mapDataStatus("Experimental Error")
	if err != nil || got != "experimental_error" {
		t.Errorf("mapDataStatus(Experimental Error) = %q, %v", got, err)
	}
	if _, err := mapDataStatus("Bogus"); err == nil {
		t.Error("expected an error for an unrecognized data_status")
	}
}

func TestMapSiblingComing(t *testing.T) {
	cases := map[string]string{
		"Unknown":    "unknown",
		"Coming":     "coming",
		"Not Coming": "not_coming",
		"None":       "not_coming", // legacy's 4th state folds into not_coming
	}
	for legacy, want := range cases {
		got, err := mapSiblingComing(legacy)
		if err != nil {
			t.Errorf("mapSiblingComing(%q) error: %v", legacy, err)
			continue
		}
		if got != want {
			t.Errorf("mapSiblingComing(%q) = %q, want %q", legacy, got, want)
		}
	}
	if _, err := mapSiblingComing("Maybe"); err == nil {
		t.Error("expected an error for an unrecognized sibling_coming value")
	}
}
