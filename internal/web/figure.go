package web

import (
	"strconv"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/topology"
)

// figureOf lays one definition's graph out for drawing (設計 4 節): the human at
// the top, the members with the workers under them, and one arrow per edge. The
// positions come from topology.LayoutGraph; the state overlay comes from the
// activity, and the links from the fields the nodes open.
func (s *Server) figure(def *definition.Definition, states map[string]string, openKey string) figureData {
	layout := topology.LayoutGraph(def.Graph)
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
	}
	for i := range layout.Members {
		member := layout.Members[i]
		box := memberFigure{
			nodeBox: nodeBox{
				X: member.X, Y: member.Y, W: member.W, H: member.H,
				Name:  member.Name,
				Sub:   dot(member.Role, member.Profile),
				Href:  s.openHref(def, definition.NodeID{Kind: definition.KindMember, Name: member.Name}, openKey),
				Class: out.Classes[i],
				Open:  nodeKeyOf(definition.NodeID{Kind: definition.KindMember, Name: member.Name}) == openKey,
				State: states[member.Name],
			},
		}
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
		out.Edges = append(out.Edges, s.arrow(def, layout, edge, openKey))
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
	Classes       []string
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
	DataFrom string
	DataTo   string
}

// CX is the x of a box's center.
func (b nodeBox) CX() float64 { return b.X + b.W/2 }

// CY is the y of a box's center.
func (b nodeBox) CY() float64 { return b.Y + b.H/2 }
