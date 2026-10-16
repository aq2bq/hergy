package app

import (
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/web"
	"github.com/aq2bq/hergy/internal/web/profileview"
)

// Profiles is one layer's own profiles.yaml, by name. A profile says whether the
// other layer holds the same name, so the screen can mark what one layer
// overrides in the other: a global profile a local one of the same name takes
// the place of, and a local profile that overrides a global one.
//
// The env of a profile reaches the screen as names alone (profileEnvNames): a
// value may hold an API key, so it goes no further than here.
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
			EnvNames: profileEnvNames(profile.Env),
			Layer:    layer,
			Over:     layer == web.LayerLocal && both,
			Shadowed: layer == web.LayerGlobal && both,
		})
	}
	return out
}

// Uses is the profiles a team definition of one layer ends up with, by the rule
// team.Env.LoadProfiles reads: the global layer's profiles, with the local
// layer's laid over them, and only for a local team definition. Each one says
// which layer it came from and whether it overrides the other layer's profile
// of that name.
//
// The overlay itself is Env.LoadProfiles's, not repeated here: this reads which
// layer each name came from out of the two files, and nothing else.
func (p servePort) Uses(layer string) []profileview.Profile {
	merged, _, err := p.env.LoadProfiles(layer)
	if err != nil {
		// A profiles.yaml that does not read holds no profile for a team
		// either, so the screen shows none rather than stopping.
		return nil
	}
	local := p.layerProfiles(web.LayerLocal)
	global := p.layerProfiles(web.LayerGlobal)
	names := sortedProfileNames(merged)
	// A local team definition reads its project's profiles over the global ones,
	// and the screen says which is which by listing the project's own first, the
	// way the prototype does (試作 profilesFor). A global definition reads the
	// global layer alone, so name order is all there is.
	if layer == web.LayerLocal {
		slices.SortStableFunc(names, func(a, b string) int {
			fromA, _ := profileOrigin(layer, a, local, global)
			fromB, _ := profileOrigin(layer, b, local, global)
			return originRank(fromA) - originRank(fromB)
		})
	}
	out := make([]profileview.Profile, 0, len(merged))
	for _, name := range names {
		profile := merged[name]
		from, over := profileOrigin(layer, name, local, global)
		out = append(out, profileview.Profile{
			Name:     profile.Name,
			Kind:     profile.Kind,
			Args:     profile.Args,
			EnvNames: profileEnvNames(profile.Env),
			Layer:    from,
			Over:     over,
		})
	}
	return out
}

// originRank orders the profiles a local team definition ends up with: its own
// project's first, then the global layer's.
func originRank(from string) int {
	if from == web.LayerLocal {
		return 0
	}
	return 1
}

// profileOrigin is which layer a profile a team definition uses came from, and
// whether it overrides the other layer's profile of that name. A global team
// definition reads the global layer alone, so every profile is the global
// layer's; a local one takes the local profile of a name the local file holds,
// which overrides the global one when both hold it.
func profileOrigin(layer, name string, local, global map[string]config.Profile) (from string, over bool) {
	if layer != web.LayerLocal {
		return web.LayerGlobal, false
	}
	if _, ok := local[name]; ok {
		_, over := global[name]
		return web.LayerLocal, over
	}
	return web.LayerGlobal, false
}

// layerProfiles reads one layer's own profiles.yaml, by name. A layer with no
// file, or a file that does not read, holds no profile rather than stopping the
// screen.
func (p servePort) layerProfiles(layer string) map[string]config.Profile {
	path := filepath.Join(p.layerDir(layer), "profiles.yaml")
	text, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	profiles, err := config.ParseProfiles(string(text), path)
	if err != nil {
		return nil
	}
	return profiles
}

// sortedProfileNames is the names of a read profiles.yaml, in order: the map is
// not ordered, and the screen shows the profiles by name.
func sortedProfileNames(profiles map[string]config.Profile) []string {
	return slices.Sorted(maps.Keys(profiles))
}

// profileEnvNames is the names of a profile's environment variables, in order,
// and never their values. The values stay in config.Profile.Env and go no
// further than the app.
func profileEnvNames(env map[string]string) []string {
	return slices.Sorted(maps.Keys(env))
}

// otherLayer is the other layer of the two, for a mark that compares one
// layer's profiles with the other's.
func otherLayer(layer string) string {
	if layer == web.LayerGlobal {
		return web.LayerLocal
	}
	return web.LayerGlobal
}
