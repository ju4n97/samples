package xfs

import (
	"os"
	"path/filepath"
	"strings"
)

// ExpandTilde replaces a leading tilde (~) with the user's home directory.
func ExpandTilde(path string) string {
	if strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}

	return path
}
