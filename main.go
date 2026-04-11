package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	version = "dev"

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	hintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Faint(true)

	// Focus: left bar + bold label (no full-row background) so textinput placeholder/cursor
	// lipgloss does not fight an outer Background().
	focusBarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true)
	focusLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))

	rowStyle = lipgloss.NewStyle()

	// Non-focused rows after a failed sync (Enter).
	dimRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Faint(true)

	timeInputPlaceholder = "e.g. 9pm · 2026-04-10 21:15 · 9:15pm 2026-04-10"
)

type zoneRow struct {
	label string
	loc   *time.Location
	ti    textinput.Model
	fixed bool // UTC and Local only
}

type model struct {
	rows    []zoneRow
	focus   int
	invalid bool
	err     string
	termW   int

	addingZone bool
	addZoneTI  textinput.Model
	addZoneErr string
}

func localDisplayLabel() string {
	name, _ := time.Now().In(time.Local).Zone()
	if name == "" {
		name = time.Now().In(time.Local).Format("MST")
	}
	if name == "" {
		return "Local"
	}
	return "Local (" + name + ")"
}

func fixedZoneDefinitions() []struct {
	label string
	loc   *time.Location
} {
	return []struct {
		label string
		loc   *time.Location
	}{
		{"UTC", time.UTC},
		{localDisplayLabel(), time.Local},
	}
}

func userZoneNames(rows []zoneRow) []string {
	var n []string
	for _, r := range rows {
		if !r.fixed {
			n = append(n, r.loc.String())
		}
	}
	return n
}

func newRowTextInput(width int, focused bool) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = timeInputPlaceholder
	ti.CharLimit = 512
	ti.Width = width
	if focused {
		ti.Focus()
	} else {
		ti.Blur()
	}
	return ti
}

func buildZoneRows(extraNames []string, termW int) []zoneRow {
	w := inputWidthForRows(nil, termW) // first pass without extras for width estimate
	defs := fixedZoneDefinitions()
	rows := make([]zoneRow, 0, len(defs)+len(extraNames))
	for i := range defs {
		ti := newRowTextInput(w, i == 0)
		rows = append(rows, zoneRow{
			label: defs[i].label,
			loc:   defs[i].loc,
			ti:    ti,
			fixed: true,
		})
	}
	for _, name := range extraNames {
		loc, err := time.LoadLocation(name)
		if err != nil {
			continue
		}
		ti := newRowTextInput(w, false)
		ti.Blur()
		rows = append(rows, zoneRow{
			label: name,
			loc:   loc,
			ti:    ti,
			fixed: false,
		})
	}
	w = inputWidthForRows(rows, termW)
	for i := range rows {
		rows[i].ti.Width = w
	}
	return rows
}

func maxLabelWidth(rows []zoneRow) int {
	n := 0
	for _, r := range rows {
		if w := lipgloss.Width(r.label); w > n {
			n = w
		}
	}
	if n < 3 {
		n = 3
	}
	return n
}

// focusGutterWidth is the visual width of the focused gutter (▌ + space).
func focusGutterWidth() int {
	return lipgloss.Width(focusBarStyle.Render("▌") + " ")
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

func newAddZoneTextInput(termW int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "e.g. America/Denver · eastern · mountain"
	ti.CharLimit = 256
	ti.Width = addZoneFieldWidth(termW)
	ti.Blur()
	styleTextInput(&ti, false)
	return ti
}

func addZoneFieldWidth(termW int) int {
	w := termW - 8
	if w < 24 {
		w = 24
	}
	return w
}

func inputWidthForRows(rows []zoneRow, termW int) int {
	lw := maxLabelWidth(rows)
	// term margin, focus gutter, label column, gap before textinput
	w := termW - 4 - focusGutterWidth() - lw - 2
	if w < 24 {
		w = 24
	}
	return w
}

func newModel() model {
	extra, err := loadExtraZoneNames()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tzc: loading saved zones: %v\n", err)
		extra = nil
	}
	termW := 80
	rows := buildZoneRows(extra, termW)
	syncRowInputStyles(rows, 0)
	return model{
		rows:       rows,
		focus:      0,
		termW:      termW,
		addZoneTI:  newAddZoneTextInput(termW),
		addingZone: false,
	}
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

func (m model) inputWidth() int {
	return inputWidthForRows(m.rows, m.termW)
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

func (m model) openAddOverlay() model {
	m.addingZone = true
	m.addZoneErr = ""
	m.addZoneTI.SetValue("")
	m.addZoneTI.Width = addZoneFieldWidth(m.termW)
	m.addZoneTI.Focus()
	styleTextInput(&m.addZoneTI, true)
	for j := range m.rows {
		m.rows[j].ti.Blur()
		styleTextInput(&m.rows[j].ti, false)
	}
	return m
}

func (m model) closeAddOverlay() model {
	m.addingZone = false
	m.addZoneTI.Blur()
	styleTextInput(&m.addZoneTI, false)
	m.addZoneErr = ""
	return m.setFocus(m.focus)
}

func (m model) removeFocusedUserZone() model {
	if m.rows[m.focus].fixed {
		return m
	}
	i := m.focus
	next := make([]zoneRow, 0, len(m.rows)-1)
	next = append(next, m.rows[:i]...)
	next = append(next, m.rows[i+1:]...)
	names := userZoneNames(next)
	if err := saveExtraZoneNames(names); err != nil {
		m.err = err.Error()
		return m
	}
	m.err = ""
	m.rows = next
	if m.focus >= len(m.rows) {
		m.focus = len(m.rows) - 1
	}
	w := m.inputWidth()
	for j := range m.rows {
		m.rows[j].ti.Width = w
	}
	return m.setFocus(m.focus)
}

func (m model) confirmAddZone() (model, tea.Cmd) {
	s := strings.TrimSpace(m.addZoneTI.Value())
	if s == "" {
		m.addZoneErr = "enter a zone name"
		return m, textinput.Blink
	}
	loc, canon, err := ParseZoneSpecifier(s)
	if err != nil {
		m.addZoneErr = err.Error()
		return m, textinput.Blink
	}
	for _, r := range m.rows {
		if r.loc.String() == canon {
			m.addZoneErr = "already in list"
			return m, textinput.Blink
		}
	}
	w := m.inputWidth()
	ti := newRowTextInput(w, false)
	ti.Blur()
	newRow := zoneRow{label: canon, loc: loc, ti: ti, fixed: false}
	next := append(append([]zoneRow{}, m.rows...), newRow)
	names := userZoneNames(next)
	if err := saveExtraZoneNames(names); err != nil {
		m.addZoneErr = err.Error()
		return m, textinput.Blink
	}
	m.rows = next
	m.focus = len(m.rows) - 1
	m = m.closeAddOverlay()
	return m, textinput.Blink
}

func (m model) updateAddZoneOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m = m.closeAddOverlay()
		return m, textinput.Blink
	case "enter":
		return m.confirmAddZone()
	}
	var cmd tea.Cmd
	m.addZoneTI, cmd = m.addZoneTI.Update(msg)
	m.addZoneErr = ""
	return m, cmd
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termW = msg.Width
		w := m.inputWidth()
		for i := range m.rows {
			m.rows[i].ti.Width = w
		}
		m.addZoneTI.Width = addZoneFieldWidth(m.termW)
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		if m.addingZone {
			return m.updateAddZoneOverlay(msg)
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

		case "+":
			m = m.openAddOverlay()
			return m, textinput.Blink

		case "-":
			if !m.rows[m.focus].fixed && strings.TrimSpace(m.rows[m.focus].ti.Value()) == "" {
				return m.removeFocusedUserZone(), textinput.Blink
			}
		case "ctrl+d":
			if !m.rows[m.focus].fixed {
				return m.removeFocusedUserZone(), textinput.Blink
			}
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
	b.WriteString(hintStyle.Render("↑/↓ · Tab · Shift+Tab field   Enter sync   Esc clear   +/- add/remove zone   Ctrl+C quit") + "\n\n")

	lw := maxLabelWidth(m.rows)
	gw := focusGutterWidth()
	for i := range m.rows {
		z := m.rows[i]
		labelVis := lipgloss.Width(z.label)
		pad := lw - labelVis
		if pad < 0 {
			pad = 0
		}
		labelCol := z.label + strings.Repeat(" ", pad)

		gutter := strings.Repeat(" ", gw)
		if i == m.focus && !m.addingZone {
			gutter = focusBarStyle.Render("▌") + " "
		}

		body := labelCol + "  " + z.ti.View()
		var line string
		switch {
		case m.addingZone:
			line = gutter + dimRowStyle.Render(body)
		case i == m.focus:
			line = gutter + focusLabelStyle.Render(labelCol) + "  " + z.ti.View()
		case m.invalid:
			line = gutter + dimRowStyle.Render(body)
		default:
			line = gutter + rowStyle.Render(body)
		}
		b.WriteString(line + "\n")
	}

	if m.invalid && m.err != "" {
		b.WriteString("\n" + errStyle.Render(m.err) + "\n")
	} else if m.err != "" {
		b.WriteString("\n" + errStyle.Render(m.err) + "\n")
	}

	if m.addingZone {
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("Add timezone   Enter save   Esc cancel") + "\n")
		b.WriteString(focusBarStyle.Render("▌") + "  " + m.addZoneTI.View() + "\n")
		if m.addZoneErr != "" {
			b.WriteString(errStyle.Render(m.addZoneErr) + "\n")
		}
	}
	return b.String()
}

func main() {
	for _, a := range os.Args[1:] {
		switch a {
		case "-v", "-version", "--version":
			fmt.Println(version)
			return
		}
	}
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
	}
}
