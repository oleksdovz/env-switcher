package config

const (
	SchemaVersion   = 1
	MaxSettingsSize = 1 << 20
	// MaxExpandedSettingsSize bounds how much a document may grow once YAML anchors/merge
	// keys are resolved, independent of MaxSettingsSize's cap on the raw file. It closes off
	// the "many small aliases pointing at one large anchor" quadratic-blowup shape that
	// remains possible even with anchor-of-anchor chaining forbidden (see parser.go).
	MaxExpandedSettingsSize = 8 * MaxSettingsSize
	MaxProjects             = 100
	MaxDefinitions          = 100
	MaxValueSize            = 64 << 10
	MaxFunctionSize         = 256 << 10
)

type Settings struct {
	Version int                           `yaml:"version"`
	Shared  Environment                   `yaml:"shared,omitempty"`
	Envs    map[string]ProjectEnvironment `yaml:"envs"`
}

type SourcePosition struct {
	Line   int
	Column int
}

type Environment struct {
	EnvVars        map[string]string `yaml:"env-vars,omitempty"`
	ShellFunctions map[string]string `yaml:"shell-functions,omitempty"`
	// ShellCmd is an anonymous activation hook: unlike ShellFunctions it has no name, isn't
	// callable on demand, and simply runs (shared, then project — see environment.Resolve)
	// as the last step of every switch. It is exactly as trusted as a named function.
	ShellCmd *string `yaml:"shell-cmd,omitempty"`
}

type ProjectEnvironment struct {
	Project        string            `yaml:"project"`
	EnvVars        map[string]string `yaml:"env-vars,omitempty"`
	ShellFunctions map[string]string `yaml:"shell-functions,omitempty"`
	ShellCmd       *string           `yaml:"shell-cmd,omitempty"`
}
