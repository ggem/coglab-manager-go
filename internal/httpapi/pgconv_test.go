package httpapi

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestDateConversion_RoundTrip(t *testing.T) {
	s := "2024-03-15"
	d, err := ptrToDate(&s)
	if err != nil {
		t.Fatalf("ptrToDate: %v", err)
	}
	if !d.Valid {
		t.Fatal("expected a valid date")
	}
	got := dateToPtr(d)
	if got == nil || *got != s {
		t.Errorf("dateToPtr = %v, want %q", got, s)
	}
}

func TestDateConversion_Nil(t *testing.T) {
	d, err := ptrToDate(nil)
	if err != nil {
		t.Fatalf("ptrToDate(nil): %v", err)
	}
	if d.Valid {
		t.Error("expected an invalid (null) date for nil input")
	}
	if got := dateToPtr(pgtype.Date{}); got != nil {
		t.Errorf("dateToPtr(zero value) = %v, want nil", got)
	}
}

func TestDateConversion_InvalidFormat(t *testing.T) {
	s := "not-a-date"
	if _, err := ptrToDate(&s); err == nil {
		t.Error("expected an error for an invalid date format")
	}
}

func TestNumericConversion_RoundTrip(t *testing.T) {
	f := 39.5
	n, err := ptrToNumeric(&f)
	if err != nil {
		t.Fatalf("ptrToNumeric: %v", err)
	}
	if !n.Valid {
		t.Fatal("expected a valid numeric")
	}
	got := numericToPtr(n)
	if got == nil || *got != f {
		t.Errorf("numericToPtr = %v, want %v", got, f)
	}
}

func TestNumericConversion_Nil(t *testing.T) {
	n, err := ptrToNumeric(nil)
	if err != nil {
		t.Fatalf("ptrToNumeric(nil): %v", err)
	}
	if n.Valid {
		t.Error("expected an invalid (null) numeric for nil input")
	}
	if got := numericToPtr(pgtype.Numeric{}); got != nil {
		t.Errorf("numericToPtr(zero value) = %v, want nil", got)
	}
}

func TestClockTimeConversion_RoundTrip(t *testing.T) {
	s := "09:05"
	pt, err := stringToClockTime(s)
	if err != nil {
		t.Fatalf("stringToClockTime: %v", err)
	}
	if !pt.Valid {
		t.Fatal("expected a valid time")
	}
	if got := clockTimeToString(pt); got != s {
		t.Errorf("clockTimeToString = %q, want %q", got, s)
	}
}

func TestClockTimeConversion_InvalidFormat(t *testing.T) {
	if _, err := stringToClockTime("not-a-time"); err == nil {
		t.Error("expected an error for an invalid time format")
	}
}

func TestPgTimeToDuration(t *testing.T) {
	pt, err := stringToClockTime("09:05")
	if err != nil {
		t.Fatalf("stringToClockTime: %v", err)
	}
	want := 9*time.Hour + 5*time.Minute
	if got := pgTimeToDuration(pt); got != want {
		t.Errorf("pgTimeToDuration = %v, want %v", got, want)
	}
}
