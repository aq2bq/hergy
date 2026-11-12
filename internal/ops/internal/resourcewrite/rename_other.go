//go:build !darwin && !linux

package resourcewrite

import "errors"

func RenameDirectory(_, _ string) error {
	return errors.New("exclusive directory rename is supported on macOS and Linux")
}
