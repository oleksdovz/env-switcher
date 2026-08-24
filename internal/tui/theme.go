package tui

import (
	huh "charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// Theme is a restrained terminal theme for the environment picker: a bold title, a plain ">"
// focus indicator, dim gray for descriptions and help text, and a plain red for errors — no
// background fills or brand colors, so it stays legible on both dark and light terminals and
// matches the plain, dialog-style look of the rest of the TUI (see render.go).
var Theme huh.Theme = huh.ThemeFunc(buildTheme)

func buildTheme(isDark bool) *huh.Styles {
	t := *huh.ThemeBase(isDark)

	dim := lipgloss.Color("8")
	accent := lipgloss.Color("4")
	errColor := lipgloss.Color("1")

	// Focus: bold title, a plain accent-colored ">" selector on the highlighted option, dim
	// description/help text, and a plain (unstyled) label for every other option so the
	// highlighted row is the only thing that visually stands out.
	t.Focused.Title = lipgloss.NewStyle().Bold(true)
	t.Focused.Description = lipgloss.NewStyle().Foreground(dim)
	t.Focused.SelectSelector = lipgloss.NewStyle().Bold(true).Foreground(accent).SetString("> ")
	t.Focused.Option = lipgloss.NewStyle()
	t.Focused.ErrorIndicator = lipgloss.NewStyle().Foreground(errColor).SetString(" *")
	t.Focused.ErrorMessage = lipgloss.NewStyle().Foreground(errColor)

	t.Blurred = t.Focused
	t.Blurred.SelectSelector = lipgloss.NewStyle().SetString("  ")

	t.Group.Title = lipgloss.NewStyle().Bold(true)

	t.Help.ShortKey = lipgloss.NewStyle().Foreground(dim)
	t.Help.ShortDesc = lipgloss.NewStyle().Foreground(dim)
	t.Help.ShortSeparator = lipgloss.NewStyle().Foreground(dim)
	t.Help.FullKey = lipgloss.NewStyle().Foreground(dim)
	t.Help.FullDesc = lipgloss.NewStyle().Foreground(dim)
	t.Help.FullSeparator = lipgloss.NewStyle().Foreground(dim)

	return &t
}
