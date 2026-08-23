package install

import (
	"fmt"
	"os"
)

func acquireLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("another install operation is active")
	}
	f.Close()
	return func() { _ = os.Remove(path) }, nil
}
