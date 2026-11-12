// Package projectinput reads configuration and HTTP project-path input. It
// preserves the accepted input grammar and translated errors outside the model.
package projectinput

import (
	"os"
	"path/filepath"
	"strings"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/i18n"
)

// Read reads one projects: entry as the directory it names: ~ and
// ~/ are the home directory, a relative path is refused, and the path is made
// absolute and resolved through symbolic links when it is there.
func Read(input domainplace.Input) (domainplace.Project, error) {
	raw := string(input)
	text := strings.TrimSpace(raw)
	if text == "" {
		return domainplace.Project{}, i18n.Errorf("error.project_path_empty")
	}
	if text == "~" || strings.HasPrefix(text, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return domainplace.Project{}, err
		}
		text = filepath.Join(home, strings.TrimPrefix(text, "~"))
	}
	if !filepath.IsAbs(text) {
		return domainplace.Project{}, i18n.Errorf("error.project_path_relative", raw)
	}
	project, err := domainplace.NewProject(text)
	return project, err
}
