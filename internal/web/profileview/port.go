// Package profileview draws the profiles of a hergy project (r-bb9c): the
// profiles of the layer being shown, and the profiles a local team definition
// ends up with once the local layer is laid over the global one. It is one of
// the screens internal/web draws, and it is read-only: it changes no file and
// writes nothing, and it reads nothing except through the Port the app hands it.
//
// A profile's env reaches this package as names alone. A value may hold an API
// key, so Profile has no field to carry one (spec/11: the launch record writes
// no env value either), and the screen says so under the table.
//
// The screen keeps the promises internal/web makes: its body carries no style
// and no script of its own (Content-Security-Policy: default-src 'self'), and
// it draws with the classes of the one stylesheet the frame brings.
package profileview

// Port is what the app hands this screen: the profiles of one layer, the
// profiles a local team definition uses, and where a layer's files live. It is
// deliberately narrow, because this screen reads and changes nothing else.
type Port interface {
	// Profiles is the profiles of one layer's own profiles.yaml, by name. A
	// profile says whether the other layer holds the same name, so the screen
	// can mark what one layer overrides in the other.
	Profiles(layer string) []Profile
	// Uses is the profiles a team definition of one layer ends up with, by the
	// same rule team.Env.LoadProfiles reads: the global layer's profiles, with
	// the local layer's laid over them, and only for a local team definition.
	// Each one says which layer it came from and whether it overrides the
	// other layer's profile of that name.
	Uses(layer string) []Profile
	// LayerRoot is where one layer's files live, as a person writes it (the
	// path the heading shows). The app answers the frame's layer toggle the
	// same way.
	LayerRoot(layer string) string
}

// Profile is one profile as the screen shows it: its name, the engine it
// starts, its arguments, and the names of its environment variables. Env holds
// no value.
type Profile struct {
	Name string
	Kind string
	Args []string
	// EnvNames are the names of the profile's environment variables, in order.
	// The values are not here and go no further than the app.
	EnvNames []string
	// Layer is where this profile came from, for the "uses" list ("local" or
	// "global"). The table of one layer leaves it as that layer.
	Layer string
	// Over says this local profile overrides a global one of the same name, so
	// a local team definition uses it.
	Over bool
	// Shadowed says a local profile of the same name overrides this global one,
	// so a local team definition does not use it. It is what the table marks.
	Shadowed bool
}
