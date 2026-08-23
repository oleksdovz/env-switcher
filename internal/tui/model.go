package tui

import (
	"os"
	"sort"

	tea "charm.land/bubbletea/v2"
	"github.com/dolf/env-switcher/internal/config"
	"github.com/dolf/env-switcher/internal/editor"
)

type Services struct {
	Reload  func() (*config.Settings, error)
	Install func() error
}
type Model struct {
	Settings                             *config.Settings
	Projects                             []string
	Focus                                int
	Selected, Status                     string
	Width, Height                        int
	mode, settingsPath, digest, fullFile string
	services                             Services
}
type operationMsg struct {
	kind     string
	settings *config.Settings
	err      error
}

func New(settings *config.Settings, path string, services Services) Model {
	m := Model{Settings: settings, settingsPath: path, services: services, digest: config.FunctionDigest(settings)}
	m.setProjects()
	if !config.IsAcknowledged(m.digest) && config.HasFunctions(settings) {
		m.mode = "trust"
	}
	return m
}
func (m Model) Init() tea.Cmd { return nil }
func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	case operationMsg:
		if msg.err != nil {
			m.Status = msg.err.Error()
		} else if msg.kind == "reload" {
			focused := ""
			if len(m.Projects) > 0 && m.Focus < len(m.Projects) {
				focused = m.Projects[m.Focus]
			}
			m.Settings = msg.settings
			m.digest = config.FunctionDigest(msg.settings)
			m.setProjects()
			if focused != "" {
				for i, name := range m.Projects {
					if name == focused {
						m.Focus = i
						break
					}
				}
			}
			if config.HasFunctions(msg.settings) && !config.IsAcknowledged(m.digest) {
				m.mode = "trust"
			}
			m.Status = "projects reloaded"
		} else {
			m.Status = "operation completed"
		}
		return m, nil
	case tea.KeyPressMsg:
		a := action(msg.Keystroke())
		if m.mode != "" {
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
				}
				if m.mode != "view" {
					m.mode = ""
				}
			}
			return m, nil
		}
		switch a {
		case "up":
			if m.Focus > 0 {
				m.Focus--
			}
		case "down":
			if m.Focus+1 < len(m.Projects) {
				m.Focus++
			}
		case "select":
			if len(m.Projects) > 0 {
				m.Selected = m.Projects[m.Focus]
				return m, tea.Quit
			}
		case "quit":
			return m, tea.Quit
		case "view":
			m.mode = "view-warning"
		case "edit":
			cmd, err := editor.Command(m.settingsPath)
			if err != nil {
				m.Status = err.Error()
			} else {
				return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return operationMsg{kind: "edit", err: err} })
			}
		case "reload":
			return m, m.reloadCmd()
		case "install":
			m.mode = "install-warning"
		}
	}
	return m, nil
}
func (m *Model) setProjects() {
	m.Projects = m.Projects[:0]
	for name := range m.Settings.Envs {
		m.Projects = append(m.Projects, name)
	}
	sort.Strings(m.Projects)
	if m.Focus >= len(m.Projects) {
		m.Focus = 0
	}
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
func (m Model) View() tea.View { return tea.NewView(m.render()) }
