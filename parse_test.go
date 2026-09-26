package main

import (
	"testing"
	"time"
)

func TestParseTimestamp(t *testing.T) {
	utc := func(y int, mo time.Month, d, h, mi, s int) time.Time {
		return time.Date(y, mo, d, h, mi, s, 0, time.UTC)
	}
	base := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		in   string
		want time.Time
	}{
		{"0", utc(1970, 1, 1, 0, 0, 0)},
		{"946684800", utc(2000, 1, 1, 0, 0, 0)},
		{"1704067200000", utc(2024, 1, 1, 0, 0, 0)},
		{"2024-01-01T00:00:00Z", utc(2024, 1, 1, 0, 0, 0)},
		{"2024-01-01 00:00:00", utc(2024, 1, 1, 0, 0, 0)}, // assumed UTC for this test
		{"2026-04-08 22:08:10", utc(2026, 4, 8, 22, 8, 10)},
		{"10:08pm", utc(2026, 4, 8, 22, 8, 0)}, // assumed today's date
		{"10:12pm 4/8/2026", utc(2026, 4, 8, 22, 12, 0)},
		{"9:15pm 2026-04-10", utc(2026, 4, 10, 21, 15, 0)},
		{"21:15 2026-04-10", utc(2026, 4, 10, 21, 15, 0)},
		{"9pm", utc(2026, 4, 8, 21, 0, 0)}, // base date local day
		{"9pm 2026-04-10", utc(2026, 4, 10, 21, 0, 0)},
		{"9:25", utc(2026, 4, 8, 9, 25, 0)},
		{"9:25:30", utc(2026, 4, 8, 9, 25, 30)},
		{"9:25:30pm", utc(2026, 4, 8, 21, 25, 30)},
		{"9:25:30 2026-04-10", utc(2026, 4, 10, 9, 25, 30)},
		{"9:25:30pm 4/8/2026", utc(2026, 4, 8, 21, 25, 30)},
		{"12:23p", utc(2026, 4, 8, 12, 23, 0)},
		{"2026-4-1", utc(2026, 4, 1, 0, 0, 0)},
		{"2026-4-1 14:30:05", utc(2026, 4, 1, 14, 30, 5)},
		{"9pm 2026-4-1", utc(2026, 4, 1, 21, 0, 0)},
		// Written-out / browser-style formats.
		{"Thu, 10 Sep 2026 19:21:53 GMT", utc(2026, 9, 10, 19, 21, 53)},
		{"10 Sep 2026 19:21:53 GMT", utc(2026, 9, 10, 19, 21, 53)},
		{"Sep 10, 2026 19:21:53 GMT", utc(2026, 9, 10, 19, 21, 53)},
		{"September 10, 2026 7:21:53 PM GMT", utc(2026, 9, 10, 19, 21, 53)},
		{"Sep 10, 2026", utc(2026, 9, 10, 0, 0, 0)},
		// Zone abbreviation offsets should apply even though assumeLoc is UTC here.
		{"10 Sep 2026 7:21:53 PM IST", utc(2026, 9, 10, 13, 51, 53)}, // IST = UTC+5:30
	}

	for _, tt := range tests {
		got, err := ParseTimestamp(tt.in, base, time.UTC)
		if err != nil {
			t.Fatalf("ParseTimestamp(%q): %v", tt.in, err)
		}
		if !got.Time.UTC().Equal(tt.want) {
			t.Fatalf("ParseTimestamp(%q) = %v, want %v", tt.in, got.Time.UTC(), tt.want)
		}
	}
}

func TestCanonicalTimestampInput(t *testing.T) {
	if got := CanonicalTimestampInput("  12:23p "); got != "12:23pm" {
		t.Fatalf("CanonicalTimestampInput = %q, want 12:23pm", got)
	}
	if got := CanonicalTimestampInput("9p"); got != "9pm" {
		t.Fatalf("CanonicalTimestampInput(9p) = %q, want 9pm", got)
	}
}

func TestParseTimestamp_now(t *testing.T) {
	before := time.Now()
	r, err := ParseTimestamp("now", time.Time{}, time.UTC)
	after := time.Now()
	if err != nil {
		t.Fatalf("ParseTimestamp(%q): %v", "now", err)
	}
	if r.Time.Before(before) || r.Time.After(after) {
		t.Fatalf("ParseTimestamp(%q) = %v, want between %v and %v", "now", r.Time, before, after)
	}
	if r.HadTZ {
		t.Fatalf("ParseTimestamp(%q).HadTZ = true, want false", "now")
	}

	// Case insensitivity
	for _, input := range []string{"NOW", "Now", "  now  "} {
		before = time.Now()
		r2, err := ParseTimestamp(input, time.Time{}, time.UTC)
		after = time.Now()
		if err != nil {
			t.Fatalf("ParseTimestamp(%q): %v", input, err)
		}
		if r2.Time.Before(before) || r2.Time.After(after) {
			t.Fatalf("ParseTimestamp(%q) = %v, want recent time", input, r2.Time)
		}
	}
}

func TestParseTimestampRejectYearAsUnix(t *testing.T) {
	_, err := ParseTimestamp("2024", time.Now(), time.UTC)
	if err == nil {
		t.Fatal("expected error for bare year")
	}
}

func TestFormatMatchingInputStyle_timeOnly12h(t *testing.T) {
	pdt := time.FixedZone("PDT", -7*3600)
	tm := time.Date(2026, 4, 10, 21, 2, 0, 0, pdt)
	got := FormatMatchingInputStyle(tm, time.UTC, "9:02pm")
	want := "2026-04-11 4:02am"
	if got != want {
		t.Fatalf("FormatMatchingInputStyle(..., %q) = %q, want %q", "9:02pm", got, want)
	}
}

func TestFormatMatchingInputStyle_RFC3339NoFraction(t *testing.T) {
	tm := time.Date(2026, 1, 1, 0, 0, 0, 123456789, time.UTC)
	got := FormatMatchingInputStyle(tm, time.UTC, "2026-01-01T00:00:00.123Z")
	want := "2026-01-01T00:00:00Z"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatMatchingInputStyle_timeThenISO(t *testing.T) {
	tm := time.Date(2026, 4, 10, 21, 15, 0, 0, time.UTC)
	got := FormatMatchingInputStyle(tm, time.UTC, "9:15pm 2026-04-10")
	want := "2026-04-10 9:15pm"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatMatchingInputStyle_preserveTZAbbrev(t *testing.T) {
	pdt := time.FixedZone("PDT", -7*3600)
	tm := time.Date(2026, 4, 14, 2, 28, 0, 0, time.UTC)
	got := FormatMatchingInputStyle(tm, pdt, "2026-04-14 02:28 UTC")
	want := "2026-04-13 19:28 PDT"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatMatchingInputStyle_MDY12h(t *testing.T) {
	loc := time.FixedZone("X", -7*3600)
	tm := time.Date(2026, 4, 10, 21, 2, 0, 0, loc)
	got := FormatMatchingInputStyle(tm, time.UTC, "4/10/2026 9:02pm")
	want := "4/11/2026 4:02am"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
