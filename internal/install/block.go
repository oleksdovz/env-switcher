package install

import (
	"bytes"
	"fmt"
	"strings"
)

const Begin = "# >>> env-switcher managed block v1 >>>"
const End = "# <<< env-switcher managed block v1 <<<"

func Block(wrapper string) []byte {
	return []byte(Begin + "\n" + strings.TrimSuffix(wrapper, "\n") + "\n" + End + "\n")
}

func Reconcile(original, block []byte) ([]byte, bool, error) {
	start := bytes.Count(original, []byte(Begin))
	end := bytes.Count(original, []byte(End))
	if start > 1 || end > 1 || start != end {
		return nil, false, fmt.Errorf("profile contains malformed managed markers")
	}
	if start == 0 {
		out := append([]byte(nil), original...)
		if len(out) > 0 && out[len(out)-1] != '\n' {
			out = append(out, '\n')
		}
		out = append(out, block...)
		return out, true, nil
	}
	a := bytes.Index(original, []byte(Begin))
	b := bytes.Index(original, []byte(End))
	if b < a {
		return nil, false, fmt.Errorf("profile contains reversed managed markers")
	}
	b += len(End)
	if b < len(original) && original[b] == '\n' {
		b++
	}
	out := append([]byte(nil), original[:a]...)
	out = append(out, block...)
	out = append(out, original[b:]...)
	return out, !bytes.Equal(out, original), nil
}

func RemoveBlock(original []byte) ([]byte, bool, error) {
	start := bytes.Count(original, []byte(Begin))
	end := bytes.Count(original, []byte(End))
	if start == 0 && end == 0 {
		return original, false, nil
	}
	if start != 1 || end != 1 {
		return nil, false, fmt.Errorf("profile contains malformed managed markers")
	}
	a := bytes.Index(original, []byte(Begin))
	b := bytes.Index(original, []byte(End))
	if b < a {
		return nil, false, fmt.Errorf("profile contains reversed managed markers")
	}
	b += len(End)
	if b < len(original) && original[b] == '\n' {
		b++
	}
	return append(append([]byte(nil), original[:a]...), original[b:]...), true, nil
}
