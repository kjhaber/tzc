package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	hintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Faint(true)

	labelW = 8

	// Focus: left bar + bold label (no full-row background) so textinput placeholder/cursor
	// lipgloss does not fight an outer Background().
	focusBarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true)
	focusLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))

	rowStyle = lipgloss.NewStyle().Padding(0, 1)

	// Non-focused rows after a failed sync (Enter).
	dimRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Faint(true).
			Padding(0, 1)
)

type zoneRow struct {
	label string
	loc   *time.Location
	ti    textinput.Model
}

type model struct {
	rows    []zoneRow
	focus   int
	invalid bool
	err     string
}

func zoneDefinitions() []struct {
	label string
	loc   *time.Location
} {
	return []struct {
		label string
		loc   *time.Location
	}{
		{"UTC", time.UTC},
		{"Local", time.Local},
	}
}

func styleTextInput(ti *textinput.Model, focused bool) {
	if focused {
		ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true)
		ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true)
	} else {
		ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		ti.TextStyle = lipgloss.NewStyle()
		ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		ti.Cursor.Style = lipgloss.NewStyle()
	}
}

func syncRowInputStyles(rows []zoneRow, focus int) {
	for j := range rows {
		styleTextInput(&rows[j].ti, j == focus)
	}
}

func newModel() model {
	defs := zoneDefinitions()
	rows := make([]zoneRow, len(defs))
	placeholder := "e.g. 9pm · 2026-04-10 21:15 · 9:15pm 2026-04-10"
	for i := range defs {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.CharLimit = 512
		if i == 0 {
			ti.Focus()
		} else {
			ti.Blur()
		}
		rows[i] = zoneRow{label: defs[i].label, loc: defs[i].loc, ti: ti}
	}
	syncRowInputStyles(rows, 0)
	return model{rows: rows}
}

func (m model) setFocus(i int) model {
	n := len(m.rows)
	if n == 0 {
		return m
	}
	i = ((i % n) + n) % n
	for j := range m.rows {
		if j == i {
			m.rows[j].ti.Focus()
		} else {
			m.rows[j].ti.Blur()
		}
	}
	m.focus = i
	syncRowInputStyles(m.rows, m.focus)
	return m
}

func (m model) stepFocus(delta int) model {
	return m.setFocus(m.focus + delta)
}

func (m model) inputWidth(termW int) int {
	w := termW - 4 - labelW - 2
	if w < 24 {
		w = 24
	}
	return w
}

func (m model) commit() model {
	i := m.focus
	s := strings.TrimSpace(m.rows[i].ti.Value())
	if s == "" {
		for j := range m.rows {
			m.rows[j].ti.SetValue("")
		}
		m.invalid = false
		m.err = ""
		return m
	}
	loc := m.rows[i].loc
	r, err := ParseTimestamp(s, time.Now().In(loc), loc)
	if err != nil {
		m.invalid = true
		m.err = err.Error()
		return m
	}
	m.invalid = false
	m.err = ""
	t := r.Time
	can := CanonicalTimestampInput(s)
	focused := &m.rows[i].ti
	focusChanged := false
	if AssumedTodayDate(r) {
		focused.SetValue(FormatMatchingInputStyle(t, loc, can))
		focusChanged = true
	} else if len(strings.Fields(s)) == 1 && s != can {
		// e.g. "12:23p" → "12:23pm" without rewriting multi-token inputs.
		focused.SetValue(can)
		focusChanged = true
	}
	if focusChanged {
		focused.CursorEnd()
	}
	for j := range m.rows {
		if j == i {
			continue
		}
		m.rows[j].ti.SetValue(FormatMatchingInputStyle(t, m.rows[j].loc, can))
	}
	return m
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w := m.inputWidth(msg.Width)
		for i := range m.rows {
			m.rows[i].ti.Width = w
		}
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		switch msg.String() {
		case "esc":
			for i := range m.rows {
				m.rows[i].ti.SetValue("")
			}
			m.invalid = false
			m.err = ""
			return m, textinput.Blink

		case "enter":
			m = m.commit()
			return m, textinput.Blink

		case "tab":
			m = m.stepFocus(1)
			return m, textinput.Blink

		case "shift+tab":
			m = m.stepFocus(-1)
			return m, textinput.Blink

		case "up":
			m = m.stepFocus(-1)
			return m, textinput.Blink

		case "down":
			m = m.stepFocus(1)
			return m, textinput.Blink
		}

		m.invalid = false
		m.err = ""
		var cmd tea.Cmd
		m.rows[m.focus].ti, cmd = m.rows[m.focus].ti.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("tzc") + " " + hintStyle.Render("Time zone converter") + "\n")
	b.WriteString(hintStyle.Render("↑/↓ · Tab · Shift+Tab field   Enter sync   Esc clear all   Ctrl+C quit") + "\n\n")

	for i := range m.rows {
		z := m.rows[i]
		pad := labelW - len(z.label)
		if pad < 0 {
			pad = 0
		}
		labelCol := z.label + strings.Repeat(" ", pad)

		var line string
		switch {
		case i == m.focus:
			line = focusBarStyle.Render("▌") + " " + focusLabelStyle.Render(labelCol) + "  " + z.ti.View()
		case m.invalid:
			line = dimRowStyle.Render(fmt.Sprintf("%s  %s", labelCol, z.ti.View()))
		default:
			line = rowStyle.Render(fmt.Sprintf("%s  %s", labelCol, z.ti.View()))
		}
		b.WriteString(line + "\n")
	}

	if m.invalid && m.err != "" {
		b.WriteString("\n" + errStyle.Render(m.err) + "\n")
	}
	return b.String()
}

func main() {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
	}
}
