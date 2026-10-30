package darwinuser

import (
	"fmt"
	"path"
	"strings"
)

// These are captured stat64 results, not directory-entry absence or permission
// to apply ENOENT to a different filesystem operation or an unknown path.
func validateStatENOENTPaths(paths []string, directories []RuntimeDirectory) error {
	return validateAbsentPaths(paths, directories, "stat")
}

func validateAbsentPaths(paths []string, directories []RuntimeDirectory, operation string) error {
	if len(paths) > 256 {
		return fmt.Errorf("too many captured %s ENOENT paths", operation)
	}
	seen := map[string]bool{}
	for _, name := range paths {
		if !path.IsAbs(name) || path.Clean(name) != name || name == "/" || len(name) > 1023 || strings.IndexByte(name, 0) >= 0 || seen[name] {
			return fmt.Errorf("invalid/duplicate captured %s ENOENT path", operation)
		}
		seen[name] = true
		for _, directory := range directories {
			if name == directory.Path || name == directory.CanonicalPath {
				return fmt.Errorf("%s ENOENT conflicts with captured directory", operation)
			}
		}
	}
	return nil
}
