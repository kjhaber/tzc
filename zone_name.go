package main

import (
	"fmt"
	"strings"
	"time"
)

// zoneAliases maps common lowercase region names/abbreviations to an IANA identifier.
// Zone abbreviations are inherently ambiguous (e.g. IST, CST, AST all name more than
// one real-world zone); where there's a conflict we pick the most common interpretation.
var zoneAliases = map[string]string{
	// US / Canada
	"eastern":      "America/New_York",
	"et":           "America/New_York",
	"est":          "America/New_York",
	"edt":          "America/New_York",
	"central":      "America/Chicago",
	"ct":           "America/Chicago",
	"cst":          "America/Chicago",
	"cdt":          "America/Chicago",
	"mountain":     "America/Denver",
	"mt":           "America/Denver",
	"mst":          "America/Denver",
	"mdt":          "America/Denver",
	"pacific":      "America/Los_Angeles",
	"pt":           "America/Los_Angeles",
	"pst":          "America/Los_Angeles",
	"pdt":          "America/Los_Angeles",
	"alaska":       "America/Anchorage",
	"akst":         "America/Anchorage",
	"akdt":         "America/Anchorage",
	"hawaii":       "Pacific/Honolulu",
	"hst":          "Pacific/Honolulu",
	"atlantic":     "America/Halifax",
	"ast":          "America/Halifax", // also Arabia Standard Time; Atlantic is more common here
	"adt":          "America/Halifax",
	"newfoundland": "America/St_Johns",
	"nst":          "America/St_Johns",
	"ndt":          "America/St_Johns",

	// UTC / GMT
	"utc": "UTC",
	"gmt": "UTC",
	"z":   "UTC",

	// Europe
	"uk":     "Europe/London",
	"london": "Europe/London",
	"bst":    "Europe/London", // British Summer Time
	"cet":    "Europe/Paris",
	"cest":   "Europe/Paris",
	"berlin": "Europe/Berlin",
	"paris":  "Europe/Paris",
	"wet":    "Europe/Lisbon",
	"eet":    "Europe/Athens",
	"eest":   "Europe/Athens",
	"msk":    "Europe/Moscow",
	"moscow": "Europe/Moscow",

	// Asia
	"india":     "Asia/Kolkata",
	"ist":       "Asia/Kolkata", // also Irish/Israel Standard Time; India is most common
	"china":     "Asia/Shanghai",
	"beijing":   "Asia/Shanghai",
	"shanghai":  "Asia/Shanghai",
	"japan":     "Asia/Tokyo",
	"jst":       "Asia/Tokyo",
	"tokyo":     "Asia/Tokyo",
	"korea":     "Asia/Seoul",
	"kst":       "Asia/Seoul",
	"singapore": "Asia/Singapore",
	"sgt":       "Asia/Singapore",
	"hongkong":  "Asia/Hong_Kong",
	"hkt":       "Asia/Hong_Kong",
	"pakistan":  "Asia/Karachi",
	"pkt":       "Asia/Karachi",
	"dubai":     "Asia/Dubai",
	"gst":       "Asia/Dubai", // Gulf Standard Time
	"nepal":     "Asia/Kathmandu",

	// Oceania
	"sydney":   "Australia/Sydney",
	"aest":     "Australia/Sydney",
	"aedt":     "Australia/Sydney",
	"adelaide": "Australia/Adelaide",
	"acst":     "Australia/Adelaide",
	"acdt":     "Australia/Adelaide",
	"perth":    "Australia/Perth",
	"awst":     "Australia/Perth",
	"nz":       "Pacific/Auckland",
	"nzst":     "Pacific/Auckland",
	"nzdt":     "Pacific/Auckland",
	"auckland": "Pacific/Auckland",

	// Africa
	"johannesburg": "Africa/Johannesburg",
	"sast":         "Africa/Johannesburg",
	"nairobi":      "Africa/Nairobi",
	"eat":          "Africa/Nairobi",

	// South America
	"brazil":    "America/Sao_Paulo",
	"brt":       "America/Sao_Paulo",
	"argentina": "America/Argentina/Buenos_Aires",
	"art":       "America/Argentina/Buenos_Aires",
	"chile":     "America/Santiago",
	"clt":       "America/Santiago",
	"clst":      "America/Santiago",
}

// ParseZoneSpecifier resolves a user string to a tz database location.
// Accepts IANA names (e.g. America/Denver), a few common region/abbreviation
// aliases (mountain, eastern, ist, …), and IANA names typed with inconsistent
// casing or spaces instead of underscores (e.g. "america/new york").
func ParseZoneSpecifier(raw string) (*time.Location, string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, "", fmt.Errorf("empty zone")
	}
	if v, ok := zoneAliases[strings.ToLower(s)]; ok {
		if loc, err := time.LoadLocation(v); err == nil {
			return loc, loc.String(), nil
		}
	}
	// Try the canonicalized form first: on case-insensitive filesystems, LoadLocation
	// will happily accept badly-cased input directly and echo that casing back in
	// loc.String(), so we'd never get a clean canonical name if we tried s first.
	if canon := canonicalizeIANACandidate(s); canon != s {
		if loc, err := time.LoadLocation(canon); err == nil {
			return loc, loc.String(), nil
		}
	}
	if loc, err := time.LoadLocation(s); err == nil {
		return loc, loc.String(), nil
	}
	return nil, "", fmt.Errorf("unknown zone %q", s)
}

// canonicalizeIANACandidate normalizes casing and word separators for an IANA-style
// zone id (e.g. "america/new york" -> "America/New_York", "EUROPE/LONDON" ->
// "Europe/London") so LoadLocation has a chance even when the user didn't type
// exact tzdata casing.
func canonicalizeIANACandidate(s string) string {
	segs := strings.Split(s, "/")
	for i, seg := range segs {
		words := strings.FieldsFunc(seg, func(r rune) bool {
			return r == '_' || r == ' ' || r == '-'
		})
		for j, w := range words {
			if w == "" {
				continue
			}
			words[j] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
		segs[i] = strings.Join(words, "_")
	}
	return strings.Join(segs, "/")
}
