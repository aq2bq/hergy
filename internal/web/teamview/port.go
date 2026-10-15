// Package teamview draws the screens of a team definition: what it holds, the
// teams Herdr reports as running right now, and the figure of one definition.
//
// Every screen is read-only but one button, which moves Herdr's focus to a
// member's pane. What the screens show they ask the app for through Port, so
// this package reads no file and calls no Herdr: it draws what the app hands
// in, and the layout of the figure is internal/topology's.
//
// The screens keep the promises internal/web makes: their bodies carry no style
// and no script of their own (Content-Security-Policy: default-src 'self'), the
// one write is a POST carrying the token of this run, and the panel swaps are
// fragments of this package's own rendering.
package teamview

import (
	"time"

	"github.com/aq2bq/hergy/internal/status"
	"github.com/aq2bq/hergy/internal/web"
)

// Port is what the app hands these screens: the layers and team definitions the
// team list reads, one definition in full, Herdr's state, and the one thing
// these screens change, Herdr's focus. The app hands one value that satisfies
// this and web.Port, so neither the frame nor the screens know of the other.
type Port interface {
	// Teams lists one layer's team definitions, as the team list shows them.
	Teams(layer string) []web.Team
	// Definition reads one team definition of one layer in full. A definition
	// that cannot be read comes back with Broken set and nothing else, so the
	// screen shows the same one line hergy validate writes.
	Definition(layer, name string) (Definition, error)
	// Running reads Herdr the way hergy status reads it: the teams it reports,
	// and each member's and worker's state. An error here is Herdr being
	// missing or unreadable, which is one line on the screen and no more.
	Running(layer string) (Herdr, error)
	// Focus moves Herdr's focus to a pane. It changes nothing else.
	Focus(pane string) error
}

// Definition is one team definition, read for the screens: the documents every
// member reads, and the members themselves.
type Definition struct {
	Name    string // the team definition name
	Layer   string
	Path    string // team.yaml, as a person writes it
	Broken  string // why it cannot be read; everything below is empty then
	Docs    []Doc
	Members []Member
}

// Doc is one rule file the members read: a document every member reads (Role
// empty) or one member's role file. Key is stable across the screens, so a link
// can name one file; Layer is where that file lives (the local layer, or the
// global one a local definition may also read), which is what a link to the
// rules screen needs.
type Doc struct {
	Key    string
	Layer  string
	Label  string // the title the file gives itself, else its name
	Path   string // as a person writes it
	Role   string // the role whose file it is; empty for a shared document
	Broken string // why it cannot be read; Rules is empty then
	Rules  []Rule
}

// Rule is one rule of a rule file: its id and the text a member reads. The
// figure reads the turn directions out of the text alone.
type Rule struct {
	ID   string
	Text string
}

// Member is one member of a team definition. Name is the short name team.yaml
// writes, which is also the name the turn directions use; Herdr is the name
// Herdr knows it by, "<team>-<member>".
type Member struct {
	Name     string
	Herdr    string
	Role     string
	Kind     string // the engine the settings name: claude, opencode
	Profile  string
	Prompt   string
	Workers  []Worker
	RoleFile *Doc // the role file this member reads, nil when it has none
}

// Worker is one worker a member may hire.
type Worker struct {
	Purpose string
	Profile string
}

// Herdr is what Herdr says right now, read the way hergy status reads it: one
// entry per running team, in the order Herdr gave them.
type Herdr struct {
	Teams []LiveTeam
	At    time.Time
}

// LiveTeam is one team Herdr reports as running: the name it runs under, the
// team definition it was booted from, and its members' and workers' rows.
//
// Known is false when this layer has no definition of that name (the team runs
// in another project), and Rows is empty then. Broken carries why a definition
// this layer does have cannot be read, and Rows is empty then too. The tab is
// where the team's members sit in Herdr.
type LiveTeam struct {
	Team       string
	Definition string
	TabID      string
	Known      bool
	Broken     string
	Rows       []status.Row
}

// Row is the state of one name of a running team, looked up by the name Herdr
// knows it by.
func (t LiveTeam) Row(name string) (status.Row, bool) {
	for _, row := range t.Rows {
		if row.Name == name {
			return row, true
		}
	}
	return status.Row{}, false
}

// Member finds one member of a definition by the short name team.yaml writes.
func (d Definition) Member(name string) (Member, bool) {
	for _, member := range d.Members {
		if member.Name == name {
			return member, true
		}
	}
	return Member{}, false
}

// Doc finds one rule file of a definition by its key: a document every member
// reads, or one member's role file.
func (d Definition) Doc(key string) (Doc, bool) {
	for _, doc := range d.Docs {
		if doc.Key == key {
			return doc, true
		}
	}
	for _, member := range d.Members {
		if member.RoleFile != nil && member.RoleFile.Key == key {
			return *member.RoleFile, true
		}
	}
	return Doc{}, false
}

// Rule finds one rule of a file by the id the rules are written with.
func (d Doc) Rule(id string) (Rule, bool) {
	for _, rule := range d.Rules {
		if rule.ID == id {
			return rule, true
		}
	}
	return Rule{}, false
}

// liveFor finds the running team booted from one definition. A definition may
// run under another team name (--as), which is why the screens match on the
// definition and take the herdr names from the team.
func liveFor(herdr Herdr, definition string) *LiveTeam {
	for i := range herdr.Teams {
		if herdr.Teams[i].Known && herdr.Teams[i].Definition == definition {
			return &herdr.Teams[i]
		}
	}
	return nil
}

// herdrName is the name Herdr knows a member by, under the name the team runs
// as. Without a running team, the name the definition gives it is used.
func herdrName(live *LiveTeam, definition, member string) string {
	if live != nil {
		return live.Team + "-" + member
	}
	return definition + "-" + member
}
