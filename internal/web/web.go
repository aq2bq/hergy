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
	Broken  string // why the definition cannot be read, empty when it reads
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
}
