package tui

import "strings"

// boxMinWidth is a floor so small boxes (a one-line confirmation, a two-project list) still read
// as a deliberate panel rather than a barely-there sliver.
const boxMinWidth = 44

const (
	ansiBold  = "\x1b[1m"
	ansiReset = "\x1b[0m"
)

// render dispatches to the current mode's view. The confirmation dialogs (trust/view/install
// warnings) share a dialog/whiptail-style look: a single bordered box, title centered in the top
// edge, and a divider before the button row at the bottom of the same box. Environment selection
// itself (the default case) is rendered by the embedded huh.Form instead — see renderSelect.
func (m Model) render() string {
	if m.quitting {
		// This inline (non-alt-screen) program leaves whatever View() returns here as the last
		// frame in the terminal's scrollback once it exits — see the `quitting` field's doc
		// comment on Model. Nothing here (the form, the shortcut footer, a stale Status line) is
		// useful once the program has already ended, so render nothing instead of leaving it
		// behind. `switched to <project>` (or whatever else the CLI has to say) is printed
		// separately, after this program has already returned.
		return ""
	}
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
	case "upgrade-warning":
		return drawDialog("Confirm", []string{
			"Downloads and verifies the latest compatible release,",
			"then replaces the installed env-switcher binary.",
		}, "Upgrade env-switcher now? [y/N]", m.boxWidth())
	case "view":
		// Raw file content, deliberately not boxed — it's arbitrary-width user data, not a
		// short fixed message, and boxing it would mean either wrapping or truncating lines
		// the user is specifically trying to inspect.
		return m.fullFile + "\nPress n or Esc to return.\n"
	default:
		return m.renderSelect()
	}
}

// shortcutHelp is the application-level key hint row. huh's own help line (shown beneath the
// select field) documents navigation — up/down/enter — since that's the form's concern; this
// documents the outer model's shortcuts, which huh has no way to know about.
const shortcutHelp = "F2/v View  F3/e Edit  F4/r Reload  F5/i Install  F6 Upgrade  F10/q Exit"

func (m Model) renderSelect() string {
	var b strings.Builder
	b.WriteString(m.form.View())
	b.WriteString("\n")
	b.WriteString(shortcutHelp)
	b.WriteString("\n")
	if m.Status != "" {
		b.WriteString(m.Status + "\n")
	}
	return b.String()
}

// boxWidth is the terminal width once known (0 before the first WindowSizeMsg, in which case
// drawBox auto-sizes to its content instead of a fixed width).
func (m Model) boxWidth() int { return m.Width }

type boxRow struct {
	text string
}

// drawDialog is drawBox for the common case of plain message lines plus a single button-row
// line, e.g. a [y/N] confirmation.
func drawDialog(title string, message []string, buttons string, maxWidth int) string {
	rows := make([]boxRow, len(message))
	for i, l := range message {
		rows[i] = boxRow{text: l}
	}
	return drawBox(title, rows, buttons, maxWidth)
}

// drawBox renders a dialog/whiptail-style panel: a bordered box with the title centered in the
// top edge, the given message rows, a divider, and a button/key-hint row — all inside the same
// box. It does not wrap long lines — callers keep each line short enough to read as one row —
// but does truncate to fit when maxWidth constrains it below what the content would otherwise
// need.
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
		b.WriteString(boxLine(r.text, width))
		b.WriteString("\n")
	}
	b.WriteString(boxDivider(width))
	b.WriteString("\n")
	b.WriteString(boxLine(buttons, width))
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

func boxLine(s string, width int) string {
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
	return "│ " + text + " │"
}
