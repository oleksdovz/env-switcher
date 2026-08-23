package install

type Target struct{ Shell, Profile, Executable string }
type Result struct {
	Changed bool
	Backup  string
	Profile string
}
