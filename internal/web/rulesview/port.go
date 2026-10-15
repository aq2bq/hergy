// Package rulesview draws the rules of a hergy project (spec/10 の 6 節): the
// rule files a layer's team definitions read, the rules in them, the three
// filters, and the panel a person edits one rule in. It is one of the screens
// internal/web draws and reads and writes nothing except through the Port the
// app hands it.
package rulesview

// Port is the part of hergy the rules screen reads and writes through. It is
// deliberately narrow: the screen touches the rule files of the layer it is
// showing, and nothing else. internal/app implements it and hands it in, and it
// satisfies both this and web.Port (the method names differ from web.Port's
// Teams and LayerRoot on purpose, so one struct can carry both).
//
// The prices of the promises in spec/10 の 4 節 are paid here, by the
// implementation: WriteRule is the only way this screen changes a file, so the
// checks that keep a write inside the layer (a path a member of some team
// definition reads) and the check that the file has not moved under the screen
// (the hash) live there and cannot be forgotten by a handler.
type Port interface {
	// RuleTeams is the team definitions of one layer by name, each with the
	// rule files its members read. A definition that cannot be read is left out
	// here; the rules screen shows what it can read rather than a broken row.
	RuleTeams(layer string) []Team
	// ReadRule reads one rule file: its text and the hash of what was read.
	// Reading the same bytes twice gives the same hash, so a save can tell
	// whether the file changed under the screen.
	ReadRule(path string) (Source, error)
	// WriteRule writes one rule file back, and refuses:
	//
	//   - a path that is not a rule file a member of some team definition of
	//     this layer reads, and
	//   - a file whose hash on disk is not the one that was read.
	//
	// It writes a file beside the target and renames it over, so a reader never
	// sees half a file.
	WriteRule(path, text, hash string) error
	// Preview is the edited document as a member reads it: the Markdown of the
	// section the rule stands in (section is its place in the document), and
	// the break rules.Check names for this rule, empty when it is sound. The
	// edited text is put over the file it stands for, so the rest of the
	// document is the file on disk.
	Preview(layer, path string, section int, text string) (Preview, error)
	// Changes is how many rules moved in one layer's documents since the launch
	// each of them was last read by: what the guide's last step counts (d-207f).
	Changes(layer string) int
	// DocChanges is what the launch record says moved in one document since the
	// launch that last read it: the same comparison the home screen draws
	// (d-0a25). The changes tab and the document's own line count it.
	DocChanges(layer, path string) DocChanges
	// Bump raises one document's version by one and answers what it wrote. It
	// is the write the home screen's /bump makes: the app reads the document,
	// raises the version it holds, and writes it back the way the rules screen
	// writes one, refusing a path no member of the layer reads and a document
	// the record does not hold, has not moved, or whose version was already
	// raised.
	Bump(layer, path string) (BumpAnswer, error)
}

// Team is one team definition and the rule files its members read.
type Team struct {
	Name    string
	Members []Member
}

// Member is one member and the rule files that go into its prompt, in order.
// The scope of the checks (a rule id that stands twice, a report a rule points
// at that is not there) is the files of one member.
type Member struct {
	Name  string
	Files []File
}

// File is one rule file a member reads.
type File struct {
	Path string
	// Display is the same file as a person writes it (".hergy/teams/…"), which
	// is what the screen shows. Path stays the way the file is read and written.
	Display string
	Role    bool // a role file, not a document every member reads
	// GroupLabel is what the left list calls this file's group, in the language
	// in use ("共通の文書", "チームの文書", "役割ファイル"). The app names it, so
	// the kicker and the left list cannot say different words for one group.
	GroupLabel string
}

// Source is one rule file as it was read.
type Source struct {
	Path string
	Text string
	Hash string
}

// Preview is the edited rule as it would reach a member.
type Preview struct {
	// Section is the section the rule stands in, as Markdown.
	Section string
	// Break is the break this rule is part of, as the reader words it, empty
	// when the rule is sound.
	Break string
}
