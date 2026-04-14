package main

import (
	"strings"
	"testing"
	"time"
)

func buildTestModel(focusedVal string, focusIdx int, locs []*time.Location) model {
	w := 60
	rows := make([]zoneRow, len(locs))
	for i, loc := range locs {
		ti := newRowTextInput(w, i == focusIdx)
		rows[i] = zoneRow{label: loc.String(), loc: loc, ti: ti, fixed: true}
	}
	rows[focusIdx].ti.SetValue(focusedVal)
	return model{rows: rows, focus: focusIdx, termW: 80}
}

func TestCommit_nowRewritesFocusedField(t *testing.T) {
	pdt := time.FixedZone("PDT", -7*3600)
	for _, input := range []string{"now", "NOW", "Now"} {
		m := buildTestModel(input, 0, []*time.Location{pdt, time.UTC})
		m = m.commit()

		got := m.rows[0].ti.Value()
		if strings.EqualFold(got, "now") {
			t.Fatalf("input %q: focused field was not rewritten (still %q)", input, got)
		}
		r, err := ParseTimestamp(got, time.Now(), pdt)
		if err != nil {
			t.Fatalf("input %q: rewritten value %q not parseable: %v", input, got, err)
		}
		// Output truncates to minute precision; allow up to 2 minutes of drift.
		diff := time.Since(r.Time)
		if diff < -10*time.Second || diff > 2*time.Minute {
			t.Fatalf("input %q: rewritten time %v not near now (%v ago)", input, r.Time, diff)
		}
	}
}
