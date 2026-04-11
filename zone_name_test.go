package main

import (
	"testing"
)

func TestParseZoneSpecifier(t *testing.T) {
	tests := []struct {
		in       string
		wantName string
	}{
		{"America/Denver", "America/Denver"},
		{"  America/New_York ", "America/New_York"},
		{"mountain", "America/Denver"},
		{"Mountain", "America/Denver"},
		{"eastern", "America/New_York"},
		{"pacific", "America/Los_Angeles"},
		{"Europe/London", "Europe/London"},
	}
	for _, tc := range tests {
		loc, canon, err := ParseZoneSpecifier(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if canon != tc.wantName {
			t.Errorf("%q: got canonical %q want %q", tc.in, canon, tc.wantName)
		}
		if loc.String() != tc.wantName {
			t.Errorf("%q: loc.String() %q want %q", tc.in, loc.String(), tc.wantName)
		}
	}
}

func TestParseZoneSpecifierErrors(t *testing.T) {
	for _, in := range []string{"", "   ", "Not/A/Real/Zone_12345"} {
		_, _, err := ParseZoneSpecifier(in)
		if err == nil {
			t.Fatalf("expected error for %q", in)
		}
	}
}
