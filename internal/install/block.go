package install

import (
	"bytes"
	"fmt"
	"strings"
)

const Begin = "# >>> env-switcher managed block v1 >>>"
const End = "# <<< env-switcher managed block v1 <<<"

// tailLines is how many trailing lines of a profile a fresh install leaves after the managed
// block, instead of always appending strictly at the end of the file. A profile's last lines are
// sometimes deliberately last — a final plugin/tool loader, a prompt finalization — and should
// keep running after this block, not be pushed to run before it.
const tailLines = 5

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
		return insertBeforeTail(original, block), true, nil
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

// insertBeforeTail inserts block into original so that at most tailLines of original's own
// trailing lines remain after it — the file's very last lines stay very last, running after this
// block rather than before it. If original has fewer than tailLines lines in the first place, the
// block goes at the very beginning, ahead of everything (there's nothing meaningfully "before the
// tail" to insert after).
func insertBeforeTail(original, block []byte) []byte {
	out := append([]byte(nil), original...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	split := splitPointNLinesFromEnd(out, tailLines)
	result := append([]byte(nil), out[:split]...)
	result = append(result, block...)
	result = append(result, out[split:]...)
	return result
}

// splitPointNLinesFromEnd returns the byte offset in data (which must either be empty or end with
// '\n') such that data[offset:] is exactly the last n newline-terminated lines — or 0 if data has
// n or fewer lines in total.
func splitPointNLinesFromEnd(data []byte, n int) int {
	newlines := 0
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] != '\n' {
			continue
		}
		newlines++
		if newlines == n+1 {
			return i + 1
		}
	}
	return 0
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
