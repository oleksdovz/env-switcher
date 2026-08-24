package tui

func action(key string) string {
	switch key {
	case "up", "k":
		return "up"
	case "down", "j":
		return "down"
	case "enter":
		return "select"
	case "f2", "v":
		return "view"
	case "f3", "e":
		return "edit"
	case "f4", "r":
		return "reload"
	case "f5", "i":
		return "install"
	case "f6":
		return "upgrade"
	case "f10", "q", "esc":
		return "quit"
	case "y", "n":
		return key
	}
	return ""
}
