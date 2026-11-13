//go:build !darwin && !linux

package runningstate

import (
	"errors"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
)

func (s Store) locked(_ domainname.TeamName, _ func() error) error {
	return errors.New("running team file locking is supported on macOS and Linux")
}
