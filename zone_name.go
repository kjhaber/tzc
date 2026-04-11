package main

import (
	"fmt"
	"strings"
	"time"
)

var zoneAliases = map[string]string{
	"eastern":  "America/New_York",
	"et":       "America/New_York",
	"est":      "America/New_York",
	"edt":      "America/New_York",
	"central":  "America/Chicago",
	"ct":       "America/Chicago",
	"cst":      "America/Chicago",
	"cdt":      "America/Chicago",
	"mountain": "America/Denver",
	"mt":       "America/Denver",
	"mst":      "America/Denver",
	"mdt":      "America/Denver",
	"pacific":  "America/Los_Angeles",
	"pt":       "America/Los_Angeles",
	"pst":      "America/Los_Angeles",
	"pdt":      "America/Los_Angeles",
}

// ParseZoneSpecifier resolves a user string to a tz database location.
// Accepts IANA names (e.g. America/Denver) and a few common aliases (mountain, eastern, …).
func ParseZoneSpecifier(raw string) (*time.Location, string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, "", fmt.Errorf("empty zone")
	}
	iana := s
	if v, ok := zoneAliases[strings.ToLower(s)]; ok {
		iana = v
	}
	loc, err := time.LoadLocation(iana)
	if err != nil {
		return nil, "", fmt.Errorf("unknown zone %q", s)
	}
	return loc, loc.String(), nil
}
