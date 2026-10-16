package app

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/web"
	"github.com/aq2bq/hergy/internal/web/profileview"
)

// Profiles is one layer's own profiles.yaml, by name. A profile says whether the
// other layer holds the same name, so the screen can mark what one layer
// overrides in the other: a global profile a local one of the same name takes
// the place of, and a local profile that overrides a global one.
//
// The env of a profile reaches the screen with its values (d-c60f): the screen
// shows them and edits them, because env holds no API key.
func (p servePort) Profiles(layer string) []profileview.Profile {
	own := p.layerProfiles(layer)
	other := p.layerProfiles(otherLayer(layer))
	out := make([]profileview.Profile, 0, len(own))
	for _, name := range sortedProfileNames(own) {
		profile := own[name]
		_, both := other[name]
		out = append(out, profileview.Profile{
			Name:     profile.Name,
			Kind:     profile.Kind,
			Args:     profile.Args,
			Env:      profileEnv(profile.Env),
			Layer:    layer,
			Over:     layer == web.LayerLocal && both,
			Shadowed: layer == web.LayerGlobal && both,
		})
	}
	return out
}

// Uses is the profiles a team definition ends up with, by the one way a name is
// looked up (K1): the global layer's profiles, with the local layer's laid over
// them, whatever layer the definition sits in. Each one says which layer it came
// from and whether it overrides the other layer's profile of that name.
//
// The overlay itself is catalog.Profiles's, not repeated here: this reads which
// layer each name came from out of the two files, and nothing else.
func (p servePort) Uses(_ string) []profileview.Profile {
	merged, _, err := p.catalog().Profiles()
	if err != nil {
		// A profiles.yaml that does not read holds no profile for a team
		// either, so the screen shows none rather than stopping.
		return nil
	}
	local := p.layerProfiles(web.LayerLocal)
	global := p.layerProfiles(web.LayerGlobal)
	names := sortedProfileNames(merged)
	// The project's own profiles come first, the way the prototype does
	// (試作 profilesFor).
	slices.SortStableFunc(names, func(a, b string) int {
		fromA, _ := profileOrigin(a, local, global)
		fromB, _ := profileOrigin(b, local, global)
		return originRank(fromA) - originRank(fromB)
	})
	out := make([]profileview.Profile, 0, len(merged))
	for _, name := range names {
		profile := merged[name]
		from, over := profileOrigin(name, local, global)
		out = append(out, profileview.Profile{
			Name:  profile.Name,
			Kind:  profile.Kind,
			Args:  profile.Args,
			Env:   profileEnv(profile.Env),
			Layer: from,
			Over:  over,
		})
	}
	return out
}

// originRank orders the profiles a team definition ends up with: the project's
// own first, then the global layer's.
func originRank(from string) int {
	if from == web.LayerLocal {
		return 0
	}
	return 1
}

// profileOrigin is which layer a profile a team definition uses came from, and
// whether it overrides the other layer's profile of that name: the local profile
// of a name the local file holds wins over the global one (K1).
func profileOrigin(name string, local, global map[string]profile.Profile) (from string, over bool) {
	if _, ok := local[name]; ok {
		_, over := global[name]
		return web.LayerLocal, over
	}
	return web.LayerGlobal, false
}

// layerProfiles reads one layer's own profiles.yaml, by name. A layer with no
// file, or a file that does not read, holds no profile rather than stopping the
// screen.
func (p servePort) layerProfiles(layer string) map[string]profile.Profile {
	path := p.profilesPath(layer)
	text, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	profiles, err := profile.ParseProfiles(string(text), path)
	if err != nil {
		return nil
	}
	return profiles
}

// profilesPath is the one file the profiles screen writes: the layer's own
// profiles.yaml. The path is built from the layer, never from the form.
func (p servePort) profilesPath(layer string) string {
	return filepath.Join(p.layerDir(layer), "profiles.yaml")
}

// ReadProfiles reads one layer's profiles.yaml for the profiles screen: its
// text and the hash of that text, whether the file is there, and, when it does
// not read, the reason hergy validate would give (validateProfiles). The screen
// is drawn from this and saves against it, so a file that changed under the
// screen is refused rather than overwritten, and a file that does not read is
// shown with its reason and never written over.
func (p servePort) ReadProfiles(layer string) profileview.Source {
	path := p.profilesPath(layer)
	text, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// A layer without a profiles.yaml yet: the screen shows the empty
		// table, and the first save writes the file. The hash of the empty text
		// is what that save hands back.
		return profileview.Source{Hash: hashText("")}
	case err != nil:
		return profileview.Source{Broken: breakLine(p.env, path, err)}
	}
	source := profileview.Source{Text: string(text), Hash: hashText(string(text)), Exists: true}
	if _, err := profile.ParseProfiles(string(text), path); err != nil {
		source.Broken = breakLine(p.env, path, err)
	}
	return source
}

// CheckProfiles is the check hergy validate runs on one layer's profiles.yaml
// (validateProfiles), run over the text the screen would write: the reason it is
// broken, in the words the screen shows, or "" when it reads. It reads the file
// itself, and then builds the team definitions that read it, so a save that
// would leave a definition without a profile it names is refused rather than
// written (r-daf8).
func (p servePort) CheckProfiles(layer, text string) string {
	path := p.profilesPath(layer)
	if _, err := profile.ParseProfiles(text, path); err != nil {
		return breakLine(p.env, path, err)
	}
	if reason := p.checkTeamsReadProfiles(layer, path, text); reason != "" {
		return reason
	}
	return ""
}

// checkTeamsReadProfiles builds every team definition that reads the layer's
// profiles.yaml with the text the screen would write, and answers why one breaks,
// or "" when the file reads for all of them.
//
// The layers built are the layer being written and, for the global layer, the
// project's local layer: a global team reads the global profiles alone, but a
// local team reads the local profiles over the global ones, so a global save can
// leave a local definition without a profile it names. A definition that is
// already broken on disk says so on its own screen and is left out here: this
// check speaks about the change and nothing else.
func (p servePort) checkTeamsReadProfiles(layer, path, text string) string {
	edit := catalog.ProfilesEdit{Path: path, Text: text}
	cat := p.catalog()
	layers := []string{layer}
	if layer == web.LayerGlobal {
		layers = append(layers, web.LayerLocal)
	}
	for _, build := range layers {
		for _, row := range cat.Definitions() {
			if !inLayer(row, build) {
				continue
			}
			def, err := cat.Definition(row.Name, build)
			if err != nil {
				continue
			}
			onDisk, err := buildResolved(cat, source.Disk{}, p.env.ProjectRoot, def, def.Name)
			if err != nil {
				continue
			}
			// The names the definition will see once the file is written: the
			// layer of the definition, read with the edit in place.
			read, _, err := cat.ProfilesEdit(edit)
			if err != nil {
				continue
			}
			if name, place := firstProfileLeftOut(onDisk, read); name != "" {
				return i18n.Errorf("web.profiles.error.team_needs_profile", place, name, row.Name).Error()
			}
			editedDef, err := cat.DefinitionEdit(row.Name, build, edit)
			if err != nil {
				return breakLine(p.env, def.TeamYAML, err)
			}
			if _, err := buildResolved(cat, source.Disk{}, p.env.ProjectRoot, editedDef, editedDef.Name); err != nil {
				return breakLine(p.env, def.TeamYAML, err)
			}
		}
	}
	return ""
}

// firstProfileLeftOut names one profile a built team definition uses that the
// file being saved does not hold, with where it is used: a member, or a worker of
// the member named by its purpose. It answers the first in the order the members
// are written. A definition that names no such profile answers "".
func firstProfileLeftOut(r *resolved, profiles map[string]profile.Profile) (name, place string) {
	for i := range r.starts {
		member := &r.starts[i]
		if member.Profile != "" {
			if _, ok := profiles[member.Profile]; !ok {
				return member.Profile, member.Name
			}
		}
		for _, worker := range member.Workers {
			if _, ok := profiles[worker.Profile]; !ok {
				return worker.Profile, member.Name + " / " + worker.Purpose
			}
		}
	}
	return "", ""
}

// WriteProfiles writes one layer's profiles.yaml. It refuses a file whose
// content is not the one the screen read (the hash), and a layer whose own
// directory is not there. A file that is not there yet is written: a layer
// without a profiles.yaml gets one on the first save (hergy new writes none).
// The write goes to a file beside the target and is renamed over it, so a reader
// never sees half a file.
func (p servePort) WriteProfiles(layer, text, hash string) error {
	path := p.profilesPath(layer)
	display := p.env.Display(path)
	before, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if hash != hashText("") {
			return i18n.Errorf("error.profiles_file_changed", display)
		}
		if _, err := os.Stat(p.layerDir(layer)); err != nil {
			return i18n.Errorf("error.profiles_no_layer", p.env.Display(p.layerDir(layer)))
		}
	case err != nil:
		return err
	default:
		if hashText(string(before)) != hash {
			return i18n.Errorf("error.profiles_file_changed", display)
		}
	}
	return writeFileBeside(path, text)
}

// sortedProfileNames is the names of a read profiles.yaml, in order: the map is
// not ordered, and the screen shows the profiles by name.
func sortedProfileNames(profiles map[string]profile.Profile) []string {
	return slices.Sorted(maps.Keys(profiles))
}

// profileEnv is a profile's environment variables, by name, with their values
// (d-c60f: the screen shows and edits them, so nothing is held back here).
func profileEnv(env map[string]string) []profileview.EnvVar {
	names := slices.Sorted(maps.Keys(env))
	out := make([]profileview.EnvVar, 0, len(names))
	for _, name := range names {
		out = append(out, profileview.EnvVar{Name: name, Value: env[name]})
	}
	return out
}

// otherLayer is the other layer of the two, for a mark that compares one
// layer's profiles with the other's.
func otherLayer(layer string) string {
	if layer == web.LayerGlobal {
		return web.LayerLocal
	}
	return web.LayerGlobal
}
