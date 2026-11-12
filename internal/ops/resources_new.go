package ops

import (
	"errors"
	"os"
	"path/filepath"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/catalog"
	"github.com/aq2bq/hergy/internal/ops/internal/newteam"
	"github.com/aq2bq/hergy/internal/scaffold"
)

// StarterDefinition is the shared resource model.
type StarterDefinition struct {
	Written               newteam.Written
	HasRoles, HasProfiles bool
}

// CreateStarterDefinition owns the CLI's starter selection and file writes.
func CreateStarterDefinition(cat *catalog.Catalog, name domainname.DefinitionName, layer, lang string) (StarterDefinition, error) {
	var out StarterDefinition
	dir := filepath.Join(cat.LayerDir(layer), "teams", name.String())
	if _, err := os.Stat(dir); err == nil {
		return out, i18n.Errorf("error.new_exists", name.String(), cat.Display(dir))
	} else if !errors.Is(err, os.ErrNotExist) {
		return out, err
	}
	files, err := scaffold.Team(lang)
	if err != nil {
		return out, err
	}
	out.HasRoles = newteam.HasRoles(cat)
	out.HasProfiles, err = newteam.HasProfiles(cat)
	if err != nil {
		return out, err
	}
	out.Written, err = newteam.Write(newteam.In{Catalog: cat, Layer: layer, Name: name.String(), Lang: lang, Files: files, Starter: true, StarterRoles: !out.HasRoles, StarterProfile: !out.HasProfiles})
	return out, err
}
