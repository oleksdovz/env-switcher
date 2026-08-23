package shell

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func Quote(value string) (string, error) {
	if strings.ContainsRune(value, 0) {
		return "", fmt.Errorf("NUL is not representable in shell literals")
	}
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("invalid UTF-8 is not representable in shell literals")
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'", nil
}
