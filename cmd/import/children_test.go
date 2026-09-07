package main

import (
	"reflect"
	"testing"
)

func TestMapRaceEthnicity(t *testing.T) {
	cases := []struct {
		name                       string
		ethnicity, race, otherRace string
		want                       []string
	}{
		{
			name:      "single race, not hispanic",
			ethnicity: "Not Hispanic or Latino", race: "White", otherRace: "No other race",
			want: []string{"white"},
		},
		{
			name:      "hispanic with unreported race",
			ethnicity: "Hispanic or Latino", race: "Unknown or Not Reported", otherRace: "Unknown or Not Reported",
			want: []string{"hispanic_or_latino"},
		},
		{
			name:      "two races via other_race, plus hispanic",
			ethnicity: "Hispanic or Latino", race: "White", otherRace: "Asian",
			want: []string{"hispanic_or_latino", "white", "asian"},
		},
		{
			name:      "everything unknown -> empty, not nil-vs-empty ambiguous",
			ethnicity: "Unknown", race: "Unknown or Not Reported", otherRace: "Unknown or Not Reported",
			want: []string{},
		},
		{
			name:      "race and other_race the same value doesn't duplicate",
			ethnicity: "Unknown", race: "White", otherRace: "White",
			want: []string{"white"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mapRaceEthnicity(c.ethnicity, c.race, c.otherRace)
			if len(got) == 0 {
				got = []string{}
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("mapRaceEthnicity(%q,%q,%q) = %v, want %v", c.ethnicity, c.race, c.otherRace, got, c.want)
			}
		})
	}
}

func TestMapSex(t *testing.T) {
	cases := map[string]string{"Male": "male", "Female": "female", "Unknown": "unknown", "": "unknown"}
	for legacy, want := range cases {
		if got := mapSex(legacy); got != want {
			t.Errorf("mapSex(%q) = %q, want %q", legacy, got, want)
		}
	}
}

func TestTriStateBool(t *testing.T) {
	if got := triStateBool("Yes"); got == nil || *got != true {
		t.Errorf("triStateBool(Yes) = %v, want true", got)
	}
	if got := triStateBool("No"); got == nil || *got != false {
		t.Errorf("triStateBool(No) = %v, want false", got)
	}
	if got := triStateBool("Unknown"); got != nil {
		t.Errorf("triStateBool(Unknown) = %v, want nil", got)
	}
}

func TestTriStatePremie(t *testing.T) {
	if got := triStatePremie("Premie"); got == nil || *got != true {
		t.Errorf("triStatePremie(Premie) = %v, want true", got)
	}
	if got := triStatePremie("Full Term"); got == nil || *got != false {
		t.Errorf("triStatePremie(Full Term) = %v, want false", got)
	}
	if got := triStatePremie("Unknown"); got != nil {
		t.Errorf("triStatePremie(Unknown) = %v, want nil", got)
	}
}

func TestMapLanguages(t *testing.T) {
	if got := mapLanguages("English,Spanish"); !reflect.DeepEqual(got, []string{"english", "spanish"}) {
		t.Errorf("mapLanguages(English,Spanish) = %v", got)
	}
	if got := mapLanguages(""); len(got) != 0 {
		t.Errorf("mapLanguages(\"\") = %v, want empty", got)
	}
}

func TestMapResponse(t *testing.T) {
	got, err := mapResponse("Snail Mail")
	if err != nil || got != "snail_mail" {
		t.Errorf("mapResponse(Snail Mail) = %q, %v", got, err)
	}
	if _, err := mapResponse("Carrier Pigeon"); err == nil {
		t.Error("expected an error for an unrecognized response value")
	}
}
