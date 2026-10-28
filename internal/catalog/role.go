package catalog

import (
	"regexp"

	"github.com/aq2bq/hergy/internal/i18n"
)

// RoleNamePattern is the shape a role name has, and so the shape of the file
// roles/<name>.yaml (spec/01 section 3): a lowercase head with lowercase letters,
// digits, underscores and dashes after it, at most 32 characters.
var RoleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// RoleNameProblem answers why one name is not a role name: nil when it is, and
// the sentence a person reads when it is not (error.role_name_pattern), naming
// the name and the way to fix it. The shape is held here alone: hergy validate,
// up and the API's check and save all ask this, so a name is one thing in every
// entrance (d-841e, r-7ea6).
//
// An empty name is not asked: a row a person just made has no name yet, and the
// screen's save is what refuses it (the same sentence, with no name between the
// brackets).
func RoleNameProblem(name string) error {
	if name == "" || RoleNamePattern.MatchString(name) {
		return nil
	}
	return i18n.Errorf("error.role_name_pattern", name)
}
