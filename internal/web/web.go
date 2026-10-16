// Package web serves hergy's screens over HTTP on this machine: a small
// server, the frame the screens are drawn in, and the one place where a rule's
// text and a prompt are turned into HTML.
//
// What it may read comes from the app through Port (team definitions, the
// layers, and, as the screens grow, herdr's state and the launch records). What
// it shows comes from the screens through Screen, and the app hands both in.
// Neither direction reaches past this package: internal/web imports no package
// of hergy's top layer, and only internal/app imports internal/web.
//
// The promises it keeps (spec/10 の 4 節):
//
//   - It listens on 127.0.0.1 only, so nothing off this machine reaches it.
//   - A request whose Host is not this server's own name is refused.
//   - A write is a POST that must carry the token of this run, and an Origin
//     that comes with it must be this server's own.
//   - Every answer carries "Content-Security-Policy: default-src 'self'", so a
//     page cannot pull in script or style from anywhere but this server.
package web

// The layers, as the screens pass them around. They are the words the commands
// and the locales use, repeated here because this package imports nothing of
// hergy's top layer.
const (
	LayerLocal  = "local"
	LayerGlobal = "global"
)

// Layers is every layer, local first: the order the layer toggle shows them in.
var Layers = []string{LayerLocal, LayerGlobal}

// Team is one row of the team list: what the app knows about one team
// definition, without the screen knowing where it was read from. A row that
// cannot be read carries Broken and no counts, so the screen can show why
// without understanding the reason.
type Team struct {
	Name    string // the team definition name
	Members int    // the members team.yaml lists
	Rules   int    // the rules those members read
	NoCheck int    // of those rules, the ones with no check
	// Launched is when the definition was last launched from this machine, as a
	// person reads it; empty when it has not been launched here.
	Launched string
	// Hash is the head of the hash that launch wrote, and Same says whether the
	// files on disk build that same hash now.
	Hash string
	Same bool
	// Href is the team definition screen at this definition: the row itself is
	// the way in (d-207f).
	Href   string
	Broken string // why the definition cannot be read, empty when it reads
	// Running says Herdr reports a team booted from this definition right now.
	// A Herdr that cannot be read leaves it false and the row renders as before.
	Running bool
}

// Index is the left list of one layer (d-207f): the team definitions of the
// layer, and the documents and role files the definitions read. Every row
// points at the screen that shows it, so the list is the way into the screens
// that are not in the navigation (the team definition).
type Index struct {
	// Teams is one row per team definition of the layer.
	Teams []IndexTeam
	// Groups are the files of the layer, grouped by what they are: the
	// documents every definition of the layer reads, the documents of one
	// team, and the role files (in that order).
	Groups []IndexGroup
	// Profiles is how many profiles the layer's own profiles.yaml holds.
	Profiles int
	// ProfilesHref is the profiles screen at this layer, which is where the
	// row leads (r-bb9c). It is the screen's own path plus the layer, so the
	// row keeps the layer being shown.
	ProfilesHref string
}

// IndexTeam is one team definition in the left list.
type IndexTeam struct {
	Name string
	// Members is how many members team.yaml lists.
	Members int
	// Href is the team definition's screen at this definition.
	Href string
	// Shadowed marks a global definition a local one of the same name takes
	// the place of: the local one is the one hergy up boots.
	Shadowed bool
	// Broken is why the definition cannot be read; empty when it reads.
	Broken string
}

// IndexGroup is one heading of the left list, with the files under it.
type IndexGroup struct {
	// Label is what the group is called, in the language in use.
	Label string
	// Rows are the files, in the order the layer reads them.
	Rows []IndexRow
}

// IndexRow is one file in the left list.
type IndexRow struct {
	// Name is what the row shows: the document's title, or its file name for
	// a role file, or the definition name for a team's own document.
	Name string
	// Mono says the name is a file or definition name and reads as code.
	Mono bool
	// Path is the document as the rules screen names it, and what tells this
	// row from the one being shown.
	Path string
	// Rules is how many rules the document holds, and Origin how many of them
	// carry an origin.
	Rules  int
	Origin int
	// Changed is how many rules moved in the document since the newest launch
	// of the layer that read it; 0 when nothing moved or nothing is recorded.
	// Whole is set when the document moved but its rules could not be lined up
	// with the ones the launch read, so the row says it changed rather than how
	// many.
	Changed int
	Whole   bool
	// Href is the rules screen at this document.
	Href string
	// Broken says the document cannot be read (the row keeps its place).
	Broken bool
}

// Meters is what the header counts over every rule of the layer (d-207f). A
// document two definitions read counts once.
type Meters struct {
	// Rules is every rule of the layer, and Origin, Reason and Check are the
	// rules that carry an origin, a reason and a check.
	Rules  int
	Origin int
	Reason int
	Check  int
}

// Port is what the app hands the screens: the settings and the state, read the
// way the commands read them. A screen asks the Port and never reaches around
// it, which is what keeps the screens free of the CLI's edge and this package
// free of hergy's top layer.
type Port interface {
	// Teams lists one layer's team definitions by name. A definition that
	// cannot be read is a row with Broken set.
	Teams(layer string) []Team
	// LayerRoot is where one layer's files live, as a person writes it
	// (".hergy", "~/.config/hergy").
	LayerRoot(layer string) string
	// Index is the left list of one layer: its team definitions and the files
	// they read.
	Index(layer string) Index
	// Meters counts the layer's rules that carry an origin, a reason and a
	// check.
	Meters(layer string) Meters
}
