package teststate

import (
	"fmt"
	"os"
	"path/filepath"
)

// State helpers must never let runningstate.Open fall back to the person's home.
// XDG ignores relative paths too, so an explicit absolute path is required.
func requireStateHome() error {
	if !filepath.IsAbs(os.Getenv("XDG_STATE_HOME")) {
		return fmt.Errorf("teststate requires an absolute XDG_STATE_HOME pointing to a temporary directory")
	}
	return nil
}
