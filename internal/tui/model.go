package tui

import (
	"os"
	"sort"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/dolf/env-switcher/internal/config"
	"github.com/dolf/env-switcher/internal/editor"
	"github.com/dolf/env-switcher/internal/upgrade"
)

type Services struct {
	Reload  func() (*config.Settings, error)
	Install func() error
	// Upgrade is the same upgrade.Upgrader.Upgrade call the "upgrade"/"--upgrade" CLI command
	// uses (see internal/app/upgrade.go and tuiCommand's wiring): F6 only ever triggers it, it
	// never reimplements any part of the upgrade itself.
	Upgrade func() (upgrade.Result, error)
}

// Model is the outer Bubble Tea model. It owns nothing about how an environment is picked —
// that's the embedded huh.Form's job — and is responsible only for application-level keyboard
// events (view/edit/reload/install/quit) and the small set of yes/no confirmation dialogs those
// can raise. See render.go for how the two are composed on screen.
type Model struct {
	Settings                             *config.Settings
	Selected, Status                     string
	Width, Height                        int
	mode, settingsPath, digest, fullFile string
	// quitting is set on every path that returns tea.Quit. Bubble Tea, in this inline
	// (non-alt-screen) mode, renders whatever View() returns for the model at the moment Update
	// decides to quit — that's the last frame left behind in the terminal's scrollback once the
	// program exits, since exiting only erases what's *below* it, not the frame itself. Without
	// this, the form and the "F2/v View  F3/e Edit  ..." shortcut footer (both irrelevant once
	// the program has already ended) would linger there after every run. render() checks this
	// first and renders nothing once it's set.
	quitting bool
	services Services
	form     *huh.Form
	field    *huh.Select[string]
}

type operationMsg struct {
	kind     string
	settings *config.Settings
	err      error
	// message overrides the default "operation completed" status text on success, for
	// operations (upgrade) that have something more specific to report.
	message string
}

func New(settings *config.Settings, path string, services Services) Model {
	field := newSelectField(settings, "")
	m := Model{
		Settings:     settings,
		settingsPath: path,
		services:     services,
		digest:       config.FunctionDigest(settings),
		field:        field,
		form:         newForm(field),
	}
	if !config.IsAcknowledged(m.digest) && config.HasFunctions(settings) {
		m.mode = "trust"
	}
	return m
}

func (m Model) Init() tea.Cmd { return m.form.Init() }

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := message.(tea.WindowSizeMsg); ok {
		m.Width, m.Height = msg.Width, msg.Height
	}
	switch msg := message.(type) {
	case operationMsg:
		return m.handleOperation(msg)
	case tea.KeyPressMsg:
		if m.mode != "" {
			return m.handleDialogKey(msg)
		}
		if next, cmd, handled := m.handleShortcut(msg); handled {
			return next, cmd
		}
	}
	return m.forwardToForm(message)
}

// handleShortcut handles the application-level keys that exist outside of (and take priority
// over) the embedded select field: view/edit/reload/install/quit. Anything it doesn't recognize
// falls through to the form, which is where navigation (up/down/enter) is actually handled.
func (m Model) handleShortcut(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	switch action(msg.Keystroke()) {
	case "view":
		m.mode = "view-warning"
		return m, nil, true
	case "edit":
		cmd, err := editor.Command(m.settingsPath)
		if err != nil {
			m.Status = err.Error()
			return m, nil, true
		}
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return operationMsg{kind: "edit", err: err} }), true
	case "reload":
		return m, m.reloadCmd(), true
	case "install":
		m.mode = "install-warning"
		return m, nil, true
	case "upgrade":
		m.mode = "upgrade-warning"
		return m, nil, true
	case "quit":
		m.quitting = true
		return m, tea.Quit, true
	case "select":
		if len(m.Settings.Envs) == 0 {
			// Nothing to select; swallow Enter instead of letting an empty select field
			// complete the form with a zero-value selection.
			return m, nil, true
		}
	}
	return m, nil, false
}

// handleDialogKey handles the y/n (and quit) keys for the trust/view/install confirmation
// dialogs. It is unchanged from the pre-huh implementation: these are plain text prompts, not
// something huh provides a field for, so they stay hand-rolled (see render.go).
func (m Model) handleDialogKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	a := action(msg.Keystroke())
	if a == "n" || a == "quit" {
		m.mode = ""
		m.fullFile = ""
		return m, nil
	}
	if a == "y" {
		switch m.mode {
		case "trust":
			if err := config.Acknowledge(m.digest); err != nil {
				m.Status = err.Error()
			} else {
				m.Status = "trusted function warning acknowledged"
			}
		case "view-warning":
			b, err := os.ReadFile(m.settingsPath)
			if err != nil {
				m.Status = err.Error()
			} else {
				m.fullFile = string(b)
				m.mode = "view"
			}
		case "install-warning":
			m.mode = ""
			return m, m.installCmd()
		case "upgrade-warning":
			m.mode = ""
			return m, m.upgradeCmd()
		}
		if m.mode != "view" {
			m.mode = ""
		}
	}
	return m, nil
}

// forwardToForm passes any message the outer model doesn't intercept (navigation keys, window
// resizes, exec/tick results, ...) down to the embedded huh.Form, then checks whether that
// completed or aborted the form.
func (m Model) forwardToForm(message tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.form.Update(message)
	if f, ok := next.(*huh.Form); ok {
		m.form = f
	}
	switch m.form.State {
	case huh.StateCompleted:
		if v, ok := m.field.GetValue().(string); ok {
			m.Selected = v
		}
		m.quitting = true
		return m, tea.Quit
	case huh.StateAborted:
		// Cancellation (e.g. ctrl+c) is a normal exit, same as F10/q: leave Selected empty.
		m.quitting = true
		return m, tea.Quit
	}
	return m, cmd
}

func (m Model) handleOperation(msg operationMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.Status = msg.err.Error()
		return m, nil
	}
	if msg.kind == "reload" {
		focused, _ := m.field.GetValue().(string)
		m.Settings = msg.settings
		m.digest = config.FunctionDigest(msg.settings)
		m.field = newSelectField(msg.settings, focused)
		cmd := m.setForm(newForm(m.field))
		if config.HasFunctions(msg.settings) && !config.IsAcknowledged(m.digest) {
			m.mode = "trust"
		}
		m.Status = "projects reloaded"
		return m, cmd
	}
	if msg.message != "" {
		m.Status = msg.message
	} else {
		m.Status = "operation completed"
	}
	return m, nil
}

// setForm installs a freshly built form (used on reload, since the environment list itself may
// have changed shape) and immediately seeds it with the last known terminal size, since a
// replacement form otherwise won't see another tea.WindowSizeMsg until the next real resize.
//
// It also runs the new form's own Init(), and returns the resulting tea.Cmd for the caller to
// return onward to Bubble Tea. Skipping this looked harmless (the form still renders) but wasn't:
// huh only marks a form's first group "active" and focuses its selected field from inside Init()
// — a freshly built *huh.Form has neither yet, so without this, a reload silently left the
// rebuilt select field unfocused. It kept rendering and kept accepting the outer model's own
// shortcuts (F2/F3/...), which made it look normal, but arrow keys and Enter never reached the
// field itself, so nothing could be selected until the next reload happened to fix it again.
func (m *Model) setForm(form *huh.Form) tea.Cmd {
	m.form = form
	initCmd := m.form.Init()
	if m.Width == 0 && m.Height == 0 {
		return initCmd
	}
	next, sizeCmd := m.form.Update(tea.WindowSizeMsg{Width: m.Width, Height: m.Height})
	if f, ok := next.(*huh.Form); ok {
		m.form = f
	}
	return tea.Batch(initCmd, sizeCmd)
}

func (m Model) reloadCmd() tea.Cmd {
	return func() tea.Msg {
		s, err := m.services.Reload()
		return operationMsg{kind: "reload", settings: s, err: err}
	}
}

func (m Model) installCmd() tea.Cmd {
	return func() tea.Msg {
		if m.services.Install == nil {
			return operationMsg{kind: "install", err: os.ErrInvalid}
		}
		return operationMsg{kind: "install", err: m.services.Install()}
	}
}

func (m Model) upgradeCmd() tea.Cmd {
	return func() tea.Msg {
		if m.services.Upgrade == nil {
			return operationMsg{kind: "upgrade", err: os.ErrInvalid}
		}
		result, err := m.services.Upgrade()
		if err != nil {
			return operationMsg{kind: "upgrade", err: err}
		}
		if result.AlreadyCurrent {
			return operationMsg{kind: "upgrade", message: "already up to date (" + result.NewVersion + ")"}
		}
		return operationMsg{kind: "upgrade", message: "upgraded " + result.OldVersion + " -> " + result.NewVersion}
	}
}

func (m Model) View() tea.View { return tea.NewView(m.render()) }

// newSelectField builds the huh.Select used for environment selection: one option per configured
// project, sorted, with the project's directory shown as a description alongside its name. If
// focused names a project still present in settings, that option starts pre-selected so a reload
// preserves the user's place in the list, matching the pre-huh model.
func newSelectField(settings *config.Settings, focused string) *huh.Select[string] {
	names := sortedEnvNames(settings)
	opts := make([]huh.Option[string], 0, len(names))
	for _, name := range names {
		opt := huh.NewOption(optionLabel(settings, name), name)
		if name == focused {
			opt = opt.Selected(true)
		}
		opts = append(opts, opt)
	}
	return huh.NewSelect[string]().
		Key("env").
		Title("Select an environment").
		Filtering(false).
		Options(opts...)
}

// optionLabel shows the environment name plus its configured directory as a lightweight
// description, when one is set — huh.Select displays options as plain strings, so this is folded
// into the option's own label rather than a separate widget.
func optionLabel(settings *config.Settings, name string) string {
	if dir := settings.Envs[name].Project; dir != "" {
		return name + "  " + dir
	}
	return name
}

func sortedEnvNames(settings *config.Settings) []string {
	names := make([]string, 0, len(settings.Envs))
	for name := range settings.Envs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func newForm(field *huh.Select[string]) *huh.Form {
	return huh.NewForm(huh.NewGroup(field).Title("env-switcher")).WithTheme(Theme).WithKeyMap(noFilterKeyMap())
}

// noFilterKeyMap disables huh's "/" search-filter for the select field. Filtering is a real huh
// feature, not something recreated here, but it's turned off rather than left on: while it's
// active, typed letters go to the filter's own text box, including v/e/r/i/q — this app's own
// single-letter shortcuts. Leaving both live at once would mean those shortcuts silently stop
// working the moment a user starts typing a search, and the pre-huh picker never had a filter
// mode to begin with, so disabling it keeps the field a plain, single-mode navigation list.
func noFilterKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Select.Filter.SetEnabled(false)
	return km
}
