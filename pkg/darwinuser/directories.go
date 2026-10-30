package darwinuser

import (
	"fmt"
	"path"
	"strings"
)

// RuntimeDirectory is a selected directory node, not a host mount. The capture
// verifies both aliases name the same directory and the canonical alias can be
// opened without following symlinks. Symlink describes traversal of Path.
type RuntimeDirectory struct {
	Stat          *DirectoryStat `json:"stat,omitempty"`
	Path          string         `json:"path"`
	CanonicalPath string         `json:"canonical_path"`
	Symlink       bool           `json:"symlink"`
}

func validateDirectories(entries []RuntimeDirectory) error {
	if len(entries) > 64 {
		return fmt.Errorf("too many captured runtime directories")
	}
	aliases := map[string]bool{"/": true}
	for _, entry := range entries {
		if entry.Stat != nil {
			if e := entry.Stat.validate(); e != nil {
				return e
			}
		}
		if entry.Symlink && entry.Path == entry.CanonicalPath {
			return fmt.Errorf("symlink directory alias equals canonical path")
		}
		for i, name := range []string{entry.Path, entry.CanonicalPath} {
			if i == 1 && name == entry.Path {
				continue
			}
			if name == "/" || !path.IsAbs(name) || path.Clean(name) != name || len(name) >= 1024 || strings.IndexByte(name, 0) >= 0 || aliases[name] {
				return fmt.Errorf("invalid/conflicting runtime directory alias")
			}
			aliases[name] = true
		}
	}
	return nil
}
