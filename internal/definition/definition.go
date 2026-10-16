// Package definition is a team's type: what a team booted from it is made of,
// and who hands the turn to whom (I1, I2). A Definition is a set of files; a
// team is a run of one definition (internal/team).
//
// The nodes and the edges are the one graph the whole of hergy projects from
// (d-11f8, refactor-design.md の 1a 節): the figure, the activity, the two
// lists, a member's prompt and validate all read this graph and add nothing to
// it. The kinds of node are not a closed enumeration — another kind may be added
// — so everything here looks at Node and Edge and never at a kind's own fields
// (I12e).
package definition

import (
	"github.com/aq2bq/hergy/internal/source"
)

// Definition is one team definition, read: the directory teams/<name>/ and the
// files in it.
type Definition struct {
	// Name is the directory name under teams/.
	Name string
	// Mission is one line: what the team is for. 段 6 makes it required.
	Mission string
	// Graph is the nodes and the edges.
	Graph *Graph
	// Version is the format version team.yaml wrote.
	Version int
	// Dir is the definition's own directory, and TeamYAML the file it was read
	// from.
	Dir      string
	TeamYAML string
	// Layer is where it was found (local or global). The layer stays inside
	// hergy: it is not a word a person reads (I11).
	Layer string

	// Docs is what every member reads, and RoleDirs where a member's role
	// document is looked for, both as team.yaml writes them. 段 4 moves the role
	// documents and the shared agreement into the catalog; until then the
	// definition carries them.
	Docs     []source.Doc
	RoleDirs []source.Doc
	// Prompt is the definition's own prompt template, empty when it writes none
	// (the member prompt in the locales is used).
	Prompt string
}

// MemberNode is one member of a definition: the frame team.yaml writes. The
// engine it runs (its profile) and the prompt it is handed are projections of
// the definition and the catalog, not fields of the frame.
type MemberNode struct {
	Name    string // the short name team.yaml writes
	Role    string
	Profile string
	Cwd     string
	// Prompt is the member's own prompt, with HasPrompt saying whether it wrote
	// one. 段 5 keeps this; the default comes from the locales.
	Prompt    string
	HasPrompt bool
}

// ID is the node's identity in the graph: a member is its name.
func (m MemberNode) ID() NodeID { return NodeID{Kind: KindMember, Name: m.Name} }

// WorkerNode is one worker a member may hire: the purpose it is hired for and
// the profile it runs. Its identity is the member that hires it and the purpose
// (I12c).
type WorkerNode struct {
	From    string // the member that hires it
	Purpose string
	Profile string
}

// ID is the node's identity: the hiring member and the purpose.
func (w WorkerNode) ID() NodeID { return NodeID{Kind: KindWorker, Name: w.From + "/" + w.Purpose} }

// Human is the human: one person, outside every team, who decides. In the graph
// it is the team's boundary (I12b, d-11f8). Its name in a settings file is
// fixed.
type Human struct{}

// ID is the human's identity, the one node whose name is fixed.
func (Human) ID() NodeID { return NodeID{Kind: KindHuman, Name: "human"} }

// NodeKind is the kind of a node. It is not a closed enumeration: the kinds
// bit is what a name is read by and what the edges of that kind mean, and
// another kind can be added without changing the edges or the invariants
// (I12e).
type NodeKind string

// The kinds of node hergy has now.
const (
	KindHuman  NodeKind = "human"
	KindMember NodeKind = "member"
	KindWorker NodeKind = "worker"
)

// NodeID is a node's kind and its name. A settings file writes a member by its
// name, the human as "human", and another kind as "<kind>:<name>"
// (d-11f8).
type NodeID struct {
	Kind NodeKind
	Name string
}

// Node is one node of the graph: something that can receive the turn and hand it
// on. Everything that reads the graph reads Node and Edge only, so a kind added
// later needs no change there.
type Node interface {
	ID() NodeID
}

// Edge is one turn: one node hands the turn to another.
type Edge struct {
	From NodeID
	To   NodeID
}
