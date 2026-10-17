package web

import (
	"strconv"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/topology"
)

// turnPending is a turn a press chose to add or take away and nothing has
// written yet: the two ends, and which way the change goes (設計 4 節). The
// figure draws an added edge dashed and an edge being taken away faint.
type turnPending struct {
	From definition.NodeID
	To   definition.NodeID
	Add  bool
}

// figure lays one definition's graph out for drawing (設計 4 節): the human at
// the top, the members with the workers under them, and one arrow per edge. The
// positions come from topology.LayoutGraph; the state overlay comes from the
// activity, the sentences a node's field carries from the check, and the links
// from the fields the nodes open. When a party's field is open, the parties it
// does not hand the turn to yet get a "＋" handle, and a chosen turn is drawn
// before it is written.
func (s *Server) figure(def *definition.Definition, states, problems map[string]string, openKey string, pending *turnPending) figureData {
	// An edge a press chose to add is laid out with the graph's own arrows, so
	// the dashed line stands in a lane instead of crossing the boxes (walk-3 P2).
	var chosen []definition.Edge
	if pending != nil && pending.Add {
		chosen = append(chosen, definition.Edge{From: pending.From, To: pending.To})
	}
	layout := topology.LayoutGraph(def.Graph, chosen...)
	out := figureData{
		Width:   layout.Width,
		Height:  layout.Height,
		Classes: roleClasses(def),
	}
	out.Human = nodeBox{
		X: layout.Human.X, Y: layout.Human.Y, W: layout.Human.W, H: layout.Human.H,
		Name:  t("label.human"),
		Href:  s.openHref(def, definition.Human{}.ID(), openKey),
		Open:  nodeKeyOf(definition.Human{}.ID()) == openKey,
		NameX: layout.Human.X + 14, NameY: layout.Human.Y + layout.Human.H/2 + 5,
		Problem: problems["human"],
	}
	boxes := map[definition.NodeID]nodeBox{(definition.Human{}).ID(): out.Human}
	for i := range layout.Members {
		member := layout.Members[i]
		id := definition.NodeID{Kind: definition.KindMember, Name: member.Name}
		box := memberFigure{
			nodeBox: nodeBox{
				X: member.X, Y: member.Y, W: member.W, H: member.H,
				Name:    member.Name,
				Sub:     memberSub(member.Role, member.Profile),
				Href:    s.openHref(def, id, openKey),
				Class:   out.Classes[i],
				Open:    nodeKeyOf(id) == openKey,
				State:   states[member.Name],
				Problem: problems[member.Name],
			},
		}
		boxes[id] = box.nodeBox
		box.NameX, box.NameY = member.X+14, member.Y+24
		box.SubX, box.SubY = member.X+14, member.Y+42
		for _, worker := range member.Workers {
			id := definition.NodeID{Kind: definition.KindWorker, Name: member.Name + "/" + worker.Purpose}
			box.Workers = append(box.Workers, nodeBox{
				X: worker.X, Y: worker.Y, W: worker.W, H: worker.H,
				Name:  worker.Purpose,
				Sub:   worker.Profile,
				Href:  s.openHref(def, id, openKey),
				Open:  nodeKeyOf(id) == openKey,
				Small: true,
				NameX: worker.X + 8, NameY: worker.Y + 15,
				SubX: worker.X + 8, SubY: worker.Y + 29,
			})
		}
		out.Members = append(out.Members, box)
	}
	for _, edge := range layout.Edges {
		arrow := s.arrow(def, layout, edge, openKey)
		if pending != nil && !pending.Add && edge.FromNode == pending.From && edge.ToNode == pending.To {
			// The edge a press chose to take away is drawn faint until the
			// write (4 節).
			arrow.Fade = true
		}
		out.Edges = append(out.Edges, arrow)
	}
	if pending != nil && pending.Add {
		for _, edge := range layout.Pending {
			arrow := s.arrow(def, layout, edge, openKey)
			// The chosen edge is not written yet, so it opens nothing.
			arrow.Href, arrow.Open, arrow.Add = "", false, true
			out.Pending = append(out.Pending, arrow)
		}
	}
	if from, ok := openSource(openKey); ok {
		out.Handles = s.turnHandles(def, boxes, from, pending)
	}
	return out
}

// openSource is the node whose field is open, when it is a party that hands the
// turn on (a member or the human): only then does the figure offer the "＋"
// handles (4 節). A worker's field, an edge's field or no field at all has none.
func openSource(openKey string) (definition.NodeID, bool) {
	if openKey == nodeKeyOf((definition.Human{}).ID()) {
		return (definition.Human{}).ID(), true
	}
	if name, ok := strings.CutPrefix(openKey, "member:"); ok {
		return definition.NodeID{Kind: definition.KindMember, Name: name}, true
	}
	return definition.NodeID{}, false
}

// turnHandles is the "＋" handle on every party the open node does not hand the
// turn to yet: the members and the human, never a worker (a worker's edges are
// made by workers:), and never the node itself. A handle whose party is already
// the chosen one goes back to the field without the choice, so a second press
// undoes the pick (4 節).
func (s *Server) turnHandles(def *definition.Definition, boxes map[definition.NodeID]nodeBox, from definition.NodeID, pending *turnPending) []handleFigure {
	taken := map[definition.NodeID]bool{}
	for _, edge := range def.Graph.Out(from) {
		if isTurn(edge) {
			taken[edge.To] = true
		}
	}
	var out []handleFigure
	add := func(id definition.NodeID) {
		box, ok := boxes[id]
		if !ok || id == from {
			return
		}
		handle := handleFigure{
			X:     box.X + box.W - 10,
			Y:     box.Y + 10,
			Label: t("web.figure.add"),
			Title: nodeLabel(from) + " → " + nodeLabel(id),
		}
		if pending != nil && pending.Add && pending.To == id {
			handle.Href = s.nodeHref(def, from)
		} else {
			handle.Href = previewHref(s.nodeHref(def, from), opTurnAdd, formValue{Name: "to", Value: id.String()})
		}
		out = append(out, handle)
	}
	for i := range def.Graph.Members {
		id := def.Graph.Members[i].ID()
		if taken[id] {
			continue
		}
		add(id)
	}
	human := (definition.Human{}).ID()
	if !taken[human] {
		add(human)
	}
	return out
}

// arrow is one edge drawn: the path, the field it opens and what it is called.
func (s *Server) arrow(def *definition.Definition, layout topology.Figure, edge topology.Arrow, openKey string) arrowFigure {
	out := arrowFigure{
		Worker: edge.Worker,
		Label:  nodeLabel(edge.FromNode) + " → " + nodeLabel(edge.ToNode),
	}
	if edge.Worker {
		// workers: makes these two edges; they open the worker's own field, not
		// a turn's (設計 4 節).
		worker := edge.FromNode
		if worker.Kind != definition.KindWorker {
			worker = edge.ToNode
		}
		from, purpose := cutWorker(worker.Name)
		out.Href = workerPath(def.Name, from, purpose)
		if openKey == nodeKeyOf(worker) {
			out.Href, out.Open = definitionPath(def.Name), true
		}
		out.Path = "M" + num(edge.LaneX) + " " + num(edge.FromY) + " V" + num(edge.ToY)
		return out
	}
	from, to := edge.FromNode.String(), edge.ToNode.String()
	out.Href = s.openEdgeHref(def, from, to, openKey)
	out.Open = turnKey(from, to) == openKey
	// The two ends' identity, for the edge's own screen and stage 3. A worker's
	// two edges carry none: their ends are the worker, whose identity a person
	// must not read anywhere on the page (S1, 設計 3.4).
	out.DataFrom, out.DataTo = from, to
	x0 := layout.MemberRight
	bend := (edge.LaneX - 0.25*x0) / 0.75
	out.Path = "M" + num(x0) + " " + num(edge.FromY) +
		" C" + num(bend) + " " + num(edge.FromY) +
		" " + num(bend) + " " + num(edge.ToY) +
		" " + num(x0) + " " + num(edge.ToY)
	return out
}

// openHref is where one node's link goes: its field, or the page itself when the
// field is already open, so a second press closes it (設計 2 節).
func (s *Server) openHref(def *definition.Definition, id definition.NodeID, openKey string) string {
	if nodeKeyOf(id) == openKey {
		return definitionPath(def.Name)
	}
	return s.nodeHref(def, id)
}

// openEdgeHref is where one edge's link goes, with the same second-press rule.
func (s *Server) openEdgeHref(def *definition.Definition, from, to, openKey string) string {
	if turnKey(from, to) == openKey {
		return definitionPath(def.Name)
	}
	return turnHref(def.Name, from, to)
}

// nodeHref is the field one node opens.
func (s *Server) nodeHref(def *definition.Definition, id definition.NodeID) string {
	switch id.Kind {
	case definition.KindHuman:
		return humanPath(def.Name)
	case definition.KindWorker:
		from, purpose := cutWorker(id.Name)
		return workerPath(def.Name, from, purpose)
	default:
		return memberPath(def.Name, id.Name)
	}
}

// nodeKeyOf is one node's key: what tells the open field from the others.
func nodeKeyOf(id definition.NodeID) string {
	switch id.Kind {
	case definition.KindHuman:
		return "human"
	case definition.KindWorker:
		return "worker:" + id.Name
	default:
		return "member:" + id.Name
	}
}

// turnKey is one turn's key.
func turnKey(from, to string) string { return "turn:" + from + ">" + to }

// cutWorker splits a worker node's name ("<from>/<purpose>") into its parts.
func cutWorker(name string) (from, purpose string) {
	for i := 0; i < len(name); i++ {
		if name[i] == '/' {
			return name[:i], name[i+1:]
		}
	}
	return name, ""
}

// roleClasses is the colour class of each member's box: the roles in the order
// they first appear, four colours around.
func roleClasses(def *definition.Definition) []string {
	seen := map[string]int{}
	out := make([]string, len(def.Graph.Members))
	for i, member := range def.Graph.Members {
		at, ok := seen[member.Role]
		if !ok {
			at = len(seen) % 4
			seen[member.Role] = at
		}
		out[i] = "c" + strconv.Itoa(at)
	}
	return out
}

// memberSub is the second line of a member's box: the role and the profile, each
// under its own name, so a person can tell which is which (walk-1b P3). The
// design writes the same line the member's field does (設計 3.4).
func memberSub(role, profile string) string {
	switch {
	case role == "":
		return profile
	case profile == "":
		return role
	default:
		return tf("web.figure.role_profile", role, profile)
	}
}

// dot joins the parts of a node's second line with the list separator.
func dot(parts ...string) string {
	out := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if out != "" {
			out += t("list.separator")
		}
		out += part
	}
	return out
}

// num writes one coordinate for SVG: no exponent, at most two decimals.
func num(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// figureData is one graph laid out for the template.
type figureData struct {
	Width, Height float64
	Human         nodeBox
	Members       []memberFigure
	Edges         []arrowFigure
	// Pending is the edge a press chose to add, dashed until it is written;
	// Handles are the "＋" marks the open node's field offers (4 節).
	Pending []arrowFigure
	Handles []handleFigure
	Classes []string
}

// handleFigure is one "＋" mark on a node of the figure: where it stands, the
// field it leads to, and what it does (4 節).
type handleFigure struct {
	X, Y  float64
	Href  string
	Label string
	Title string
}

// nodeBox is one box of the figure and the lines it writes.
type nodeBox struct {
	X, Y, W, H   float64
	NameX, NameY float64
	SubX, SubY   float64
	Name         string
	Sub          string
	Href         string
	Class        string
	Open         bool
	Small        bool
	// State is the live state of the node on the activity screen ("active",
	// "blocked", "idle", "handed", empty when none).
	State string
	// Problem is a sentence the check writes about this node (node:<name>), shown
	// on the node and in its field (R1). Empty when there is none.
	Problem string
}

// memberFigure is one member's box with the workers under it.
type memberFigure struct {
	nodeBox
	Workers []nodeBox
}

// arrowFigure is one edge drawn as SVG. DataFrom and DataTo are the two ends'
// identity, for a screen to tell one arrow from another (設計 4 節).
type arrowFigure struct {
	Path     string
	Href     string
	Label    string
	Worker   bool
	Open     bool
	Fade     bool // the edge a press chose to take away, until it is written
	Add      bool // the edge a press chose to add, before it is written
	DataFrom string
	DataTo   string
}

// CX is the x of a box's center.
func (b nodeBox) CX() float64 { return b.X + b.W/2 }

// CY is the y of a box's center.
func (b nodeBox) CY() float64 { return b.Y + b.H/2 }
