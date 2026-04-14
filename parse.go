package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type ParseResult struct {
	Time time.Time

	// HadTZ is true if the input explicitly specified a timezone/offset (e.g. Z, -0700).
	HadTZ bool

	// InterpretedTZ describes how the input timezone was determined.
	// Examples: "from input: Z", "assumed: local", "assumed: UTC".
	InterpretedTZ string

	// Assumptions are additional notes (e.g. "assumed today's date").
	Assumptions []string
}

// AssumedTodayDate reports whether the parse filled in today's calendar date for a time-only input.
func AssumedTodayDate(r ParseResult) bool {
	for _, a := range r.Assumptions {
		if a == "assumed today's date" {
			return true
		}
	}
	return false
}

var (
	// Time-of-day only: "22:08", "9:25:30", "10:08pm", "9:25:30pm", or hour-only "9pm" / "12 am".
	reTimeOnly   = regexp.MustCompile(`(?i)^\s*(\d{1,2}:\d{2}(:\d{2})?(\s*[ap]m)?|\d{1,2}\s*(am|pm))\s*$`)
	reHourOnly12 = regexp.MustCompile(`(?i)^\d{1,2}\s*(am|pm)$`)
	reMDY        = regexp.MustCompile(`^\s*(\d{1,2})/(\d{1,2})/(\d{4})(?:\s+(.*))?\s*$`)
	reHasTZ      = regexp.MustCompile(`(?i)(z\b|[+-]\d{2}:?\d{2}\b|utc\b|gmt\b|[a-z]{2,5}\b)`)
	reHasHMS     = regexp.MustCompile(`(?i)\d{1,2}:\d{2}:\d{2}`)
	reISOStart   = regexp.MustCompile(`^\d{4}-\d{1,2}-\d{1,2}`)
	reISOTSep    = regexp.MustCompile(`^\d{4}-\d{1,2}-\d{1,2}T`) // T as ISO-8601 separator (not e.g. "UTC")
	// reISOHasTZ detects a TZ token (Z, offset, or 3-5-letter abbreviation) preceded by whitespace.
	// Requires 3+ letters to avoid false-positives on "am"/"pm".
	reISOHasTZ    = regexp.MustCompile(`(?i)\s+(z\b|[+-]\d{2}:?\d{2}\b|utc\b|gmt\b|[a-z]{3,5}\b)`)
	reUnixLiteral = regexp.MustCompile(`^\d+(\.\d+)?$`)
	reYMDOnly     = regexp.MustCompile(`^\d{4}-\d{1,2}-\d{1,2}$`)
)

// ParseTimestamp does best-effort parsing of common log/graph timestamps.
// If the input doesn't specify a timezone, it will be interpreted in assumeLoc.
// If the input doesn't specify a date (e.g. "10:08pm"), it assumes baseDate in assumeLoc.
func ParseTimestamp(raw string, baseDate time.Time, assumeLoc *time.Location) (ParseResult, error) {
	s := CanonicalTimestampInput(raw)
	if s == "" {
		return ParseResult{}, fmt.Errorf("empty input")
	}

	res := ParseResult{}

	if assumeLoc == nil {
		assumeLoc = time.Local
	}
	if baseDate.IsZero() {
		baseDate = time.Now().In(assumeLoc)
	}
	baseDate = baseDate.In(assumeLoc)

	if strings.EqualFold(s, "now") {
		return ParseResult{
			Time:          time.Now().In(assumeLoc),
			HadTZ:         false,
			InterpretedTZ: "assumed: " + locLabel(assumeLoc),
		}, nil
	}

	if t, err := parseUnix(s); err == nil {
		res.Time = t
		res.HadTZ = true
		res.InterpretedTZ = "from input: unix epoch (UTC)"
		return res, nil
	}

	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		res.Time = t
		res.HadTZ = true
		res.InterpretedTZ = "from input: RFC3339 offset"
		return res, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		res.Time = t
		res.HadTZ = true
		res.InterpretedTZ = "from input: RFC3339 offset"
		return res, nil
	}

	// "10:08pm" or "22:08" → assume today's date in assumeLoc
	if reTimeOnly.MatchString(s) {
		t, tzFromInput, err := parseTimeOnlyOnBaseDate(s, baseDate, assumeLoc)
		if err == nil {
			res.Time = t
			res.HadTZ = tzFromInput
			if tzFromInput {
				res.InterpretedTZ = "from input"
			} else {
				res.InterpretedTZ = "assumed: " + locLabel(assumeLoc)
			}
			res.Assumptions = append(res.Assumptions, "assumed today's date")
			return res, nil
		}
	}

	// "4/8/2026 10:12pm" (MDY) optionally with seconds
	if reMDY.MatchString(s) {
		t, hadTZ, err := parseMDY(s, assumeLoc)
		if err == nil {
			res.Time = t
			res.HadTZ = hadTZ
			if hadTZ {
				res.InterpretedTZ = "from input"
			} else {
				res.InterpretedTZ = "assumed: " + locLabel(assumeLoc)
			}
			return res, nil
		}
	}

	// "2026-04-08 22:08:10" and friends (no TZ) → assumeLoc
	// If input looks like it includes a TZ token but parsing fails, we'll still fall back.
	// appendFlexibleISODateLayouts adds 2006-1-2 variants so "2026-4-1" parses without leading zeroes.
	layouts := appendFlexibleISODateLayouts([]string{
		"2006-01-02 3:04:05pm",
		"2006-01-02 3:04pm",
		"2006-01-02 3pm",
		"2006-01-02 3 pm",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	})
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, assumeLoc); err == nil {
			res.Time = t
			res.HadTZ = false
			res.InterpretedTZ = "assumed: " + locLabel(assumeLoc)
			return res, nil
		}
	}

	// Try layouts that include numeric timezone offsets.
	layoutsTZ := appendFlexibleISODateLayouts([]string{
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04 -0700",
		"2006-01-02 15:04:05.999999999 -07:00",
		"2006-01-02 15:04:05 -07:00",
		"2006-01-02 15:04 -07:00",
		"2006-01-02T15:04:05.999999999-0700",
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04-0700",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04-07:00",
	})
	for _, layout := range layoutsTZ {
		if t, err := time.Parse(layout, s); err == nil {
			res.Time = t
			res.HadTZ = true
			res.InterpretedTZ = "from input: offset"
			return res, nil
		}
	}

	// As a last attempt, if the string appears to contain tz-ish tokens, try ParseInLocation anyway
	// (time.ParseInLocation ignores unknown abbreviations; better than giving up).
	if reHasTZ.MatchString(s) {
		for _, layout := range appendFlexibleISODateLayouts([]string{
			"2006-01-02 15:04:05 MST",
			"2006-01-02 15:04 MST",
			"2006-01-02T15:04:05 MST",
			"2006-01-02T15:04 MST",
		}) {
			if t, err := time.ParseInLocation(layout, s, assumeLoc); err == nil {
				res.Time = t
				res.HadTZ = true
				res.InterpretedTZ = "from input: abbreviation"
				return res, nil
			}
		}
	}

	return ParseResult{}, fmt.Errorf("could not parse %q", raw)
}

// appendFlexibleISODateLayouts adds a 2006-1-2 variant after each layout using a zero-padded
// ISO date prefix so inputs like "2026-4-1" parse without leading zeroes on month or day.
func appendFlexibleISODateLayouts(layouts []string) []string {
	out := make([]string, 0, len(layouts)*2)
	for _, l := range layouts {
		out = append(out, l)
		if alt := strings.Replace(l, "2006-01-02", "2006-1-2", 1); alt != l {
			out = append(out, alt)
		}
	}
	return out
}

func parseUnix(s string) (time.Time, error) {
	// Integer path
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		if len(s) == 4 {
			if y := int(i); y >= 1900 && y <= 2100 {
				return time.Time{}, strconv.ErrSyntax
			}
		}
		sec, ok := unixFromInt(i)
		if !ok {
			return time.Time{}, strconv.ErrSyntax
		}
		return time.Unix(sec, 0).UTC(), nil
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f != f { // NaN
		return time.Time{}, strconv.ErrSyntax
	}

	if len(s) == 4 {
		if y := int(f); f == float64(y) && y >= 1900 && y <= 2100 {
			return time.Time{}, strconv.ErrSyntax
		}
	}

	sec := f
	switch {
	case f >= 1e14: // nanoseconds
		sec = f / 1e9
	case f >= 1e10: // milliseconds
		sec = f / 1e3
	}
	if sec < 0 || sec >= 4_102_441_200 {
		return time.Time{}, strconv.ErrSyntax
	}
	secI := int64(sec)
	nsec := int64((sec - float64(secI)) * 1e9)
	return time.Unix(secI, nsec).UTC(), nil
}

func unixFromInt(i int64) (sec int64, ok bool) {
	switch {
	case i >= 1_000_000_000_000_000: // ns
		return i / 1_000_000_000, true
	case i >= 10_000_000_000: // ms
		return i / 1000, true
	default:
		if i >= 0 && i < 4_102_441_200 {
			return i, true
		}
	}
	return 0, false
}

// expandAbbrevAMPMToken turns a trailing "…a" / "…p" (digit immediately before) into "…am" / "…pm".
func expandAbbrevAMPMToken(tok string) string {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return tok
	}
	low := strings.ToLower(tok)
	if strings.HasSuffix(low, "pm") || strings.HasSuffix(low, "am") {
		return tok
	}
	last, sz := utf8.DecodeLastRuneInString(tok)
	if last != 'a' && last != 'p' && last != 'A' && last != 'P' {
		return tok
	}
	before := tok[:len(tok)-sz]
	if before == "" {
		return tok
	}
	r, _ := utf8.DecodeLastRuneInString(before)
	if r < '0' || r > '9' {
		return tok
	}
	return tok + "m"
}

// expandAbbrevTokens applies expandAbbrevAMPMToken to each whitespace-separated token.
func expandAbbrevTokens(s string) string {
	parts := strings.Fields(strings.TrimSpace(s))
	if len(parts) == 0 {
		return strings.TrimSpace(s)
	}
	for i := range parts {
		parts[i] = expandAbbrevAMPMToken(parts[i])
	}
	return strings.Join(parts, " ")
}

// CanonicalTimestampInput returns trim + abbreviated am/pm + two-part date/time reordering.
// Use this for display and for consistent classification in FormatMatchingInputStyle.
func CanonicalTimestampInput(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = expandAbbrevTokens(s)
	return normalizeTwoPartDateTime(s)
}

// normalizeTwoPartDateTime rewrites "9:15pm 2026-04-10" → "2026-04-10 9:15pm" and
// "10:12pm 4/8/2026" → "4/8/2026 10:12pm" so the rest of the parser can handle them.
func normalizeTwoPartDateTime(s string) string {
	s = strings.TrimSpace(s)
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return s
	}
	a, b := parts[0], parts[1]
	// First token must look like a time (H:MM or hour-only 12h like "9pm").
	if !strings.Contains(a, ":") && !reHourOnly12.MatchString(a) {
		return s
	}
	if reYMDOnly.MatchString(b) && !strings.Contains(b, "/") {
		return b + " " + a
	}
	low := strings.ToLower(a)
	if (strings.Contains(low, "am") || strings.Contains(low, "pm")) && strings.Contains(b, "/") {
		return b + " " + a
	}
	return s
}

// FormatMatchingInputStyle renders t in loc with roughly the same notation as the user's raw
// input (12h vs 24h, MDY vs ISO-style date, seconds only if present). Never includes sub-second precision.
func FormatMatchingInputStyle(t time.Time, loc *time.Location, raw string) string {
	if loc == nil {
		loc = time.Local
	}
	tt := t.In(loc)
	s0 := CanonicalTimestampInput(raw)
	sLow := strings.ToLower(s0)
	use12 := strings.Contains(sLow, "am") || strings.Contains(sLow, "pm")
	hasSec := reHasHMS.MatchString(s0)

	switch {
	case reTimeOnly.MatchString(s0):
		if use12 {
			if hasSec {
				return tt.Format("2006-01-02 3:04:05pm")
			}
			return tt.Format("2006-01-02 3:04pm")
		}
		if hasSec {
			return tt.Format("2006-01-02 15:04:05")
		}
		return tt.Format("2006-01-02 15:04")

	case reMDY.MatchString(s0):
		if use12 {
			if hasSec {
				return tt.Format("1/2/2006 3:04:05pm")
			}
			return tt.Format("1/2/2006 3:04pm")
		}
		if hasSec {
			return tt.Format("1/2/2006 15:04:05")
		}
		return tt.Format("1/2/2006 15:04")

	case reISOTSep.MatchString(s0):
		// RFC3339 / ISO8601 with a T separator — second precision, no fractional seconds.
		return trimSubsecondFromRFC(tt.Format(time.RFC3339Nano))

	case reISOStart.MatchString(s0):
		var layout string
		switch {
		case use12 && hasSec:
			layout = "2006-01-02 3:04:05pm"
		case use12:
			layout = "2006-01-02 3:04pm"
		case hasSec:
			layout = "2006-01-02 15:04:05"
		default:
			layout = "2006-01-02 15:04"
		}
		if reISOHasTZ.MatchString(s0) {
			layout += " MST"
		}
		return tt.Format(layout)

	case isLikelyUnixLiteral(s0):
		return tt.Format("2006-01-02 15:04:05 MST")

	default:
		if use12 {
			if hasSec {
				return tt.Format("2006-01-02 3:04:05pm")
			}
			return tt.Format("2006-01-02 3:04pm")
		}
		if hasSec {
			return tt.Format("2006-01-02 15:04:05 MST")
		}
		return tt.Format("2006-01-02 15:04 MST")
	}
}

func isLikelyUnixLiteral(s string) bool {
	s = strings.TrimSpace(s)
	if !reUnixLiteral.MatchString(s) {
		return false
	}
	return len(s) >= 8
}

func trimSubsecondFromRFC(s string) string {
	// RFC3339Nano is ...05.999999999Z07:00 — strip .fraction before zone.
	i := strings.LastIndex(s, ".")
	if i < 0 {
		return s
	}
	// Find end of fractional digits (ASCII digits only).
	j := i + 1
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if j == i+1 {
		return s
	}
	// Expect following char to be Z or + or - (offset).
	if j >= len(s) {
		return s
	}
	switch s[j] {
	case 'Z', '+', '-':
		return s[:i] + s[j:]
	default:
		return s
	}
}

func parseTimeOnlyOnBaseDate(raw string, baseDate time.Time, loc *time.Location) (time.Time, bool, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	layouts := []string{
		"15:04", "3:04pm", "3:04 pm",
		"15:04:05", "3:04:05pm", "3:04:05 pm",
		"3pm", "3 pm", // hour-only 12h (e.g. "9pm", "12 am")
	}
	var parsed time.Time
	var err error
	for _, layout := range layouts {
		parsed, err = time.ParseInLocation(layout, s, loc)
		if err == nil {
			break
		}
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), loc), false, nil
}

func parseMDY(s string, assumeLoc *time.Location) (time.Time, bool, error) {
	m := reMDY.FindStringSubmatch(s)
	if len(m) == 0 {
		return time.Time{}, false, errors.New("no match")
	}
	mo, _ := strconv.Atoi(m[1])
	day, _ := strconv.Atoi(m[2])
	yr, _ := strconv.Atoi(m[3])
	rest := strings.TrimSpace(m[4])
	if rest == "" {
		return time.Date(yr, time.Month(mo), day, 0, 0, 0, 0, assumeLoc), false, nil
	}
	// time part, possibly with tz offset
	for _, layout := range []string{
		"3:04pm",
		"3:04 pm",
		"3:04:05pm",
		"3:04:05 pm",
		"3pm",
		"3 pm",
		"15:04",
		"15:04:05",
		"3:04pm -0700",
		"3:04 pm -0700",
		"3:04:05pm -0700",
		"3:04:05 pm -0700",
		"15:04 -0700",
		"15:04:05 -0700",
		"3:04pm -07:00",
		"3:04 pm -07:00",
		"3:04:05pm -07:00",
		"3:04:05 pm -07:00",
		"15:04 -07:00",
		"15:04:05 -07:00",
	} {
		if strings.Contains(layout, "-07") {
			if tt, err := time.Parse(layout, rest); err == nil {
				_, off := tt.Zone()
				loc := time.FixedZone("offset", off)
				return time.Date(yr, time.Month(mo), day, tt.Hour(), tt.Minute(), tt.Second(), tt.Nanosecond(), loc), true, nil
			}
			continue
		}
		if tt, err := time.ParseInLocation(layout, rest, assumeLoc); err == nil {
			return time.Date(yr, time.Month(mo), day, tt.Hour(), tt.Minute(), tt.Second(), tt.Nanosecond(), assumeLoc), false, nil
		}
	}
	return time.Time{}, false, errors.New("bad time portion")
}

func locLabel(loc *time.Location) string {
	if loc == nil {
		return "local"
	}
	if loc == time.UTC {
		return "UTC"
	}
	// Try to return something stable-ish.
	name := loc.String()
	if name == "Local" {
		return "local"
	}
	return name
}
