package catalog

import "regexp"

// RoleNamePattern is the shape a role name has, and so the shape of the file
// roles/<name>.yaml (spec/01 section 3): a lowercase head with lowercase letters,
// digits, underscores and dashes after it, at most 32 characters. A screen that
// makes a role document or renames one holds a name to this, and the sentence
// that refuses one is error.role_name_pattern.
var RoleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
