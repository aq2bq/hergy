package profile

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"go.yaml.in/yaml/v3"
)

// The reading every settings YAML shares. A settings file carries a format
// version first, and every key it writes has to be one hergy knows: the version
// is checked before an unknown key is reported, so a file from another version
// says so instead of listing the keys it does not know.

var (
	typeNameRE = regexp.MustCompile(` in type \S+|\s*(?:\[\])?config\.raw\w+|\s*into\s+(?:map\[string\]interface \{\}|(?:\[\])?interface \{\}|\[\](?:string|int|bool|any|float64))`)
	dangleRE   = regexp.MustCompile(`(?m)\s+into\s*$`)
)

// ReadVersion decodes only the version, so a missing or wrong version is
// reported before any unknown-key complaint. what names the file in the message
// ("team.yaml"), and path is where it was.
func ReadVersion(text, what, path string) (int, error) {
	var only struct {
		Version *int `yaml:"version"`
	}
	dec := yaml.NewDecoder(strings.NewReader(text))
	if err := dec.Decode(&only); err != nil {
		if errors.Is(err, io.EOF) {
			return 0, i18n.Errorf("error.file_empty", what, path)
		}
		return 0, fmt.Errorf("%s: %s", path, CleanError(err))
	}
	if err := CheckVersion(only.Version, what, path); err != nil {
		return 0, err
	}
	return *only.Version, nil
}

// StrictDecode reads a settings file into out, refusing a key out does not have.
func StrictDecode(text string, out any) error {
	dec := yaml.NewDecoder(strings.NewReader(text))
	dec.KnownFields(true)
	return dec.Decode(out)
}

// CheckVersion holds a settings file to the one version this build reads.
func CheckVersion(version *int, what, path string) error {
	if version == nil {
		return i18n.Errorf("error.version_missing", what, path)
	}
	if *version != 1 {
		return i18n.Errorf("error.version_wrong", what, *version, path)
	}
	return nil
}

// CleanError is a YAML error as a person reads it: the Go type names yaml.v3
// puts in it are dropped, and the "profiles" field it names when a team.yaml
// writes may_spawn in the old form is explained.
func CleanError(err error) string {
	msg := typeNameRE.ReplaceAllString(err.Error(), "")
	msg = dangleRE.ReplaceAllString(msg, "")
	msg = strings.ReplaceAll(msg, "field profiles not found", "field profiles not found"+i18n.T("error.profiles_moved"))
	return strings.TrimSpace(msg)
}
