// Package profileview draws and edits the profiles of a hergy project (r-bb9c,
// r-daf8): the profiles of the layer being shown, the profiles a local team
// definition ends up with once the local layer is laid over the global one, and
// the form that writes the layer's own profiles.yaml back. It is one of the
// screens internal/web draws, and it touches no file except through the Port the
// app hands it.
//
// The one file this screen writes is the shown layer's profiles.yaml. A save
// goes through the same check hergy validate runs on that file (CheckProfiles),
// so a file hergy cannot read back is never written, and through WriteProfiles,
// which refuses a file that changed on disk since the screen read it.
//
// A profile's env reaches this screen with its values and is edited like any
// other field (d-c60f): env holds no API key, so nothing is hidden.
//
// The screen keeps the promises internal/web makes: its body carries no style
// and no script of its own (Content-Security-Policy: default-src 'self'), and
// it draws with the classes of the one stylesheet the frame brings.
package profileview

// Port is what the app hands this screen: the profiles of one layer, the
// profiles a local team definition uses, where a layer's files live, and the one
// file this screen writes. It is deliberately narrow, because this screen reads
// and changes nothing else.
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
	// ReadProfiles reads one layer's profiles.yaml. The text and its hash are
	// what a save hands back, so a file that changed under the screen is
	// refused rather than overwritten; a file that does not read is named as
	// broken and is never written over.
	ReadProfiles(layer string) Source
	// CheckProfiles runs over the text the screen would write the same check
	// hergy validate runs on this layer's profiles.yaml, and answers the reason
	// it is broken, or "" when it reads. The app knows the file's path, so the
	// reason names it.
	CheckProfiles(layer, text string) string
	// WriteProfiles writes one layer's profiles.yaml. It refuses a file whose
	// hash on disk is not the one that was read, a layer whose own directory is
	// not there, and it writes a file that is not there yet (the layer's
	// profiles.yaml is made on the first save). It writes a file beside the
	// target and renames it over, so a reader never sees half a file.
	WriteProfiles(layer, text, hash string) error
}

// Profile is one profile as the screen shows it: its name, the engine it
// starts, its arguments, and its environment variables. Env carries the values
// too (d-c60f): the screen shows and edits them.
type Profile struct {
	Name string
	Kind string
	Args []string
	// Env are the profile's environment variables, by name, in order.
	Env []EnvVar
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

// EnvVar is one environment variable of a profile: its name and its value, the
// way the form holds it.
type EnvVar struct {
	Name  string
	Value string
}

// Source is one layer's profiles.yaml as it was read.
type Source struct {
	// Text is the file's content, empty when the file is not there.
	Text string
	// Hash is the hash of Text, and of the empty text when the file is not
	// there. A save hands it back, so a file that changed under the screen is
	// refused rather than overwritten.
	Hash string
	// Exists says the file is there. A layer without one shows the empty table,
	// and the first save writes the file.
	Exists bool
	// Broken is why the file does not read, in the words hergy validate uses,
	// empty when it reads or is not there. A file that does not read is shown
	// with the reason, and the screen does not edit it: hergy does not write
	// over a file it cannot read.
	Broken string
}
