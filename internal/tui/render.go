package tui

import "strings"

// boxMinWidth is a floor so small boxes (a one-line confirmation, a two-project list) still read
// as a deliberate panel rather than a barely-there sliver.
const boxMinWidth = 44

const (
	ansiReverse = "\x1b[7m"
	ansiBold    = "\x1b[1m"
	ansiReset   = "\x1b[0m"
)

// render dispatches to the current mode's view. Every mode shares the same dialog/whiptail-style
// look: a single bordered box, title centered in the top edge, a divider before the button row
// at the bottom of the same box (not a separate line below it), and a reverse-video highlight on
// the focused row instead of a plain marker character.
func (m Model) render() string {
	switch m.mode {
	case "trust":
		return drawDialog("Trust Warning", []string{
			"Configured shell functions are trusted executable code.",
			"They run only after environment selection.",
		}, "Continue? [y/N]", m.boxWidth())
	case "view-warning":
		return drawDialog("Sensitive Data", []string{
			"settings.yaml contains sensitive values.",
		}, "Show complete unmasked file? [y/N]", m.boxWidth())
	case "install-warning":
		return drawDialog("Confirm", nil, "Install or update shell integration? [y/N]", m.boxWidth())
	case "view":
		// Raw file content, deliberately not boxed — it's arbitrary-width user data, not a
		// short fixed message, and boxing it would mean either wrapping or truncating lines
		// the user is specifically trying to inspect.
		return m.fullFile + "\nPress n or Esc to return.\n"
	default:
		return m.renderList()
	}
}

func (m Model) renderList() string {
	const reservedRows = 6 // top/bottom border, divider, button row, one spare line of slack
	var rows []boxRow
	start, end := 0, len(m.Projects)
	if m.Height > 0 {
		visible := m.Height - reservedRows
		if visible < 1 {
			visible = 1
		}
		if end > visible {
			start = m.Focus - visible + 1
			if start < 0 {
				start = 0
			}
			end = start + visible
			if end > len(m.Projects) {
				end = len(m.Projects)
			}
		}
	}
	for i := start; i < end; i++ {
		rows = append(rows, boxRow{text: "  " + m.Projects[i], highlight: i == m.Focus})
	}
	if end < len(m.Projects) {
		rows = append(rows, boxRow{text: "  …"})
	}
	if len(m.Projects) == 0 {
		rows = append(rows, boxRow{text: "No configured projects."})
	}
	buttons := "F2/v View  F3/e Edit  F4/r Reload  F5/i Install  F10/q Exit"
	var b strings.Builder
	b.WriteString(drawBox("env-switcher", rows, buttons, m.boxWidth()))
	if m.Status != "" {
		b.WriteString(m.Status + "\n")
	}
	return b.String()
}

// boxWidth is the terminal width once known (0 before the first WindowSizeMsg, in which case
// drawBox auto-sizes to its content instead of a fixed width).
func (m Model) boxWidth() int { return m.Width }

type boxRow struct {
	text      string
	highlight bool
}

// drawDialog is drawBox for the common case of plain (non-highlighted) message lines plus a
// single button-row line, e.g. a [y/N] confirmation.
func drawDialog(title string, message []string, buttons string, maxWidth int) string {
	rows := make([]boxRow, len(message))
	for i, l := range message {
		rows[i] = boxRow{text: l}
	}
	return drawBox(title, rows, buttons, maxWidth)
}

// drawBox renders a dialog/whiptail-style panel: a bordered box with the title centered in the
// top edge, the given rows (any marked highlight rendered in reverse video), a divider, and a
// button/key-hint row — all inside the same box. It does not wrap long lines — callers keep each
// line short enough to read as one row — but does truncate to fit when maxWidth constrains it
// below what the content would otherwise need.
func drawBox(title string, rows []boxRow, buttons string, maxWidth int) string {
	width := boxMinWidth
	if w := len([]rune(title)) + 4; w > width {
		width = w
	}
	for _, r := range rows {
		if w := len([]rune(r.text)) + 4; w > width {
			width = w
		}
	}
	buttonsMin := len([]rune(buttons)) + 4
	if buttonsMin > width {
		width = buttonsMin
	}
	// Grow 30% past the tightest fit so the panel reads as a deliberate window rather than
	// shrink-wrapped to its text.
	if grown := width * 13 / 10; grown > width {
		width = grown
	}
	// Only shrink to maxWidth if the button/key-hint row (Exit included) still fits — a
	// terminal too narrow for that is rare enough that a slightly overflowing box beats
	// silently truncating "F10/q Exit" off the edge.
	if maxWidth > 0 && maxWidth >= buttonsMin && width > maxWidth {
		width = maxWidth
	}
	var b strings.Builder
	b.WriteString(boxTopBorder(title, width))
	b.WriteString("\n")
	for _, r := range rows {
		b.WriteString(boxLine(r.text, width, r.highlight))
		b.WriteString("\n")
	}
	b.WriteString(boxDivider(width))
	b.WriteString("\n")
	b.WriteString(boxLine(buttons, width, false))
	b.WriteString("\n")
	b.WriteString(boxBottomBorder(width))
	b.WriteString("\n")
	return b.String()
}

func boxTopBorder(title string, width int) string {
	label := []rune(" " + title + " ")
	inner := width - 2 // width minus the two corner characters
	if len(label) > inner {
		label = label[:inner]
	}
	dashes := inner - len(label)
	left := dashes / 2
	right := dashes - left
	return "┌" + strings.Repeat("─", left) + ansiBold + string(label) + ansiReset + strings.Repeat("─", right) + "┐"
}

func boxDivider(width int) string {
	return "├" + strings.Repeat("─", width-2) + "┤"
}

func boxBottomBorder(width int) string {
	return "└" + strings.Repeat("─", width-2) + "┘"
}

func boxLine(s string, width int, highlight bool) string {
	inner := width - 4 // "│ " + text + " │"
	if inner < 0 {
		inner = 0
	}
	runes := []rune(s)
	if len(runes) > inner {
		runes = runes[:inner]
	}
	pad := inner - len(runes)
	text := string(runes) + strings.Repeat(" ", pad)
	if highlight {
		text = ansiReverse + text + ansiReset
	}
	return "│ " + text + " │"
}
