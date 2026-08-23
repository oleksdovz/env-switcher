package environment

type Variable struct {
	Name  string
	Value string
}

type Function struct {
	Name string
	Body string
}

type Effective struct {
	Project   string
	Shell     string
	Variables []Variable
	Functions []Function
	// ShellCmds are anonymous activation hooks, in run order: the shared shell-cmd (if any)
	// first, then the project's own (if any).
	ShellCmds []string
}
