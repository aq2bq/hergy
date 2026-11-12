package catalog

import (
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/i18n"
)

// RoleNameProblem answers why one name is not a role name: nil when it is, and
// the sentence a person reads when it is not (error.role_name_pattern), naming
// the name and the way to fix it. domain/name owns the shape; hergy validate,
// up and the API's check and save all ask this, so a name is one thing in every
// entrance (d-841e, r-7ea6).
//
// An empty name is not asked: a row a person just made has no name yet, and the
// screen's save is what refuses it (the same sentence, with no name between the
// brackets).
func RoleNameProblem(name string) error {
	if name == "" {
		return nil
	}
	if _, err := domainname.ParseRoleName(name); err == nil {
		return nil
	}
	return i18n.Errorf("error.role_name_pattern", name)
}
