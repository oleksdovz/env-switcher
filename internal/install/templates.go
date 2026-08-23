package install

import _ "embed"

//go:embed templates/bash.sh.tmpl
var bashWrapper string

//go:embed templates/zsh.sh.tmpl
var zshWrapper string

func Wrapper(shellName string) (string, bool) {
	if shellName == "bash" {
		return bashWrapper, true
	}
	if shellName == "zsh" {
		return zshWrapper, true
	}
	return "", false
}
