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

	// Agreement is the team's own agreement: agreement.yaml in the definition's
	// own directory, with HasAgreement saying whether the team wrote one. It is
	// optional. The agreement every team of a layer shares is not the
	// definition's to carry: the catalog holds it (catalog.SharedAgreement),
	// because a name is looked up in one place (K1). A member's role document is
	// looked up the same way (catalog.Role).
	Agreement    source.Doc
	HasAgreement bool
	// Prompt is the definition's own prompt template, empty when it writes none
	// (the member prompt in the locales is used).
	Prompt string
	// Ledger is the ledger: rules: what each write a member of this team makes
	// must be, and which member may make it (spec-ledger). Empty when the
	// definition writes none.
	Ledger []Ledger
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

// nodeKind is one kind of node: how a name of that kind is read from a settings
// file, and what the turn invariants say about a node of it. Adding a kind is
// adding one entry here; nothing that reads Node and Edge changes (I12e, 1 節).
type nodeKind struct {
	// byName says a settings file writes a node of this kind by its name alone:
	// the human and a member. Another kind is written "<kind>:<name>".
	byName bool
	// fixedName is the one name a node of this kind is written with, or "" when
	// any name the graph holds may be used (a member is written by its name).
	fixedName string
	// noTurnKey is the locale key of the message a node of this kind with no
	// outgoing turn gets (I12a), or "" when a node of this kind cannot be
	// without one: a worker's edge back to the member that hired it is made by
	// workers:, not written.
	noTurnKey string
	// noTurnArgs is how many times that message names the node: the human's
	// sentence needs no name, a member's names the member and the turns: line
	// to add.
	noTurnArgs int
}

// nodeKinds is every kind hergy has now. A settings file writes the human as
// "human" and a member by its name; a name written as "<kind>:<name>" is read
// and stopped (d-11f8 keeps the shape open for the kinds to come).
var nodeKinds = map[NodeKind]nodeKind{
	KindHuman:  {byName: true, fixedName: "human", noTurnKey: "error.graph_human_no_turn"},
	KindMember: {byName: true, noTurnKey: "error.graph_member_no_turn", noTurnArgs: 2},
	KindWorker: {},
}

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
