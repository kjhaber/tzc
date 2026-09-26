package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

func isQuitCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestCtrlC_alwaysQuitsInstantly(t *testing.T) {
	m := buildTestModel("some input", 0, []*time.Location{time.UTC})
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !isQuitCmd(cmd) {
		t.Fatalf("ctrl+c should quit instantly regardless of input state")
	}
	_ = newM
}

func TestEsc_clearsInputWithoutArmingExit(t *testing.T) {
	m := buildTestModel("some input", 0, []*time.Location{time.UTC})
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	nm := newModel.(model)
	if nm.rows[0].ti.Value() != "" {
		t.Fatalf("esc should clear focused input, got %q", nm.rows[0].ti.Value())
	}
	if nm.pendingExit {
		t.Fatalf("esc should not arm pending-exit when it had input to clear")
	}
	if isQuitCmd(cmd) {
		t.Fatalf("esc should not quit when it had input to clear")
	}
}

func TestEsc_armsPendingExitWhenInputAlreadyEmpty(t *testing.T) {
	m := buildTestModel("", 0, []*time.Location{time.UTC})
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	nm := newModel.(model)
	if !nm.pendingExit {
		t.Fatalf("esc on empty input should arm pending-exit")
	}
	if cmd == nil {
		t.Fatalf("esc on empty input should schedule the exit-arm timeout")
	}
	if isQuitCmd(cmd) {
		t.Fatalf("first esc on empty input should not quit yet")
	}
}

func TestEsc_secondPressWhilePendingQuits(t *testing.T) {
	m := buildTestModel("", 0, []*time.Location{time.UTC})
	m, _ = mustModel(m.Update(tea.KeyMsg{Type: tea.KeyEsc}))
	if !m.pendingExit {
		t.Fatalf("precondition: pending-exit should be armed")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !isQuitCmd(cmd) {
		t.Fatalf("second esc while pending-exit is armed should quit")
	}
}

func TestExitArmTimeout_clearsPendingExitWhenSeqMatches(t *testing.T) {
	m := buildTestModel("", 0, []*time.Location{time.UTC})
	m, _ = mustModel(m.Update(tea.KeyMsg{Type: tea.KeyEsc}))
	newModel, _ := m.Update(exitArmTimeoutMsg{seq: m.exitArmSeq})
	nm := newModel.(model)
	if nm.pendingExit {
		t.Fatalf("matching timeout should clear pending-exit")
	}
}

func TestExitArmTimeout_ignoresStaleSeq(t *testing.T) {
	m := buildTestModel("", 0, []*time.Location{time.UTC})
	m, _ = mustModel(m.Update(tea.KeyMsg{Type: tea.KeyEsc}))
	staleSeq := m.exitArmSeq - 1
	newModel, _ := m.Update(exitArmTimeoutMsg{seq: staleSeq})
	nm := newModel.(model)
	if !nm.pendingExit {
		t.Fatalf("stale timeout must not clear a pending-exit armed after it")
	}
}

func TestAnyOtherKey_dismissesPendingExit(t *testing.T) {
	m := buildTestModel("", 0, []*time.Location{time.UTC})
	m, _ = mustModel(m.Update(tea.KeyMsg{Type: tea.KeyEsc}))
	if !m.pendingExit {
		t.Fatalf("precondition: pending-exit should be armed")
	}
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	nm := newModel.(model)
	if nm.pendingExit {
		t.Fatalf("pressing another key should dismiss pending-exit")
	}
}

func mustModel(tm tea.Model, cmd tea.Cmd) (model, tea.Cmd) {
	return tm.(model), cmd
}
