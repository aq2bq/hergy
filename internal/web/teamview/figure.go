package teamview

import (
	"net/http"
	"strconv"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/status"
	"github.com/aq2bq/hergy/internal/topology"
	"github.com/aq2bq/hergy/internal/web"
)

// figureScreen draws one team definition as a single figure: the documents
// every member reads, the role files, the members and the workers they may
// hire, the human, and the turn directions read out of the rules' text.
//
// The positions come from internal/topology; this screen writes them as SVG,
// and every line of text in the figure is put together here. The figure carries
// no style of its own (default-src 'self' refuses a style attribute), so every
// colour and size is a class in this package's stylesheet, and every box and
// arrow points at a panel of this package.
type figureScreen struct{ base }

// Name is what the navigation calls this screen.
func (s *figureScreen) Name() string { return i18n.T("web.teamview.figure.title") }

// Routes is the figure page. A box or an arrow opens a panel, which the
// definition screen answers (/panel/...).
func (s *figureScreen) Routes() []web.Route {
	return []web.Route{{Path: "/figure", Handler: s.show}}
}

func (s *figureScreen) show(r *http.Request) (web.View, error) {
	layer := web.LayerOf(r)
	name, teams := s.choose(layer, r.URL.Query().Get("name"))
	data := figureData{
		Layer:  layer,
		Name:   name,
		Teams:  pickTeams(teams, name),
		Legend: figureLegend(),
	}
	// Nothing to draw when the layer has no team definition: the page says so.
	if name != "" {
		def, err := s.port.Definition(layer, name)
		if err != nil {
			return web.View{}, err
		}
		data.Path = def.Path
		data.Broken = def.Broken
		if def.Broken == "" {
			states, live := s.liveStates(layer, def.Name)
			layout := topology.Layout(topologyMembers(def), topologyDocs(def))
			data.View = figureOf(layout, def, live, states)
			data.NoArrows = len(layout.Edges) == 0
			if live != nil {
				data.Banner = &banner{Team: live.Team, Verdict: status.Judge(live.Rows).Text()}
			}
		}
	}
	view, err := s.body("figure.html", data)
	view.Title = s.Name()
	return view, err
}

// figureData is what the figure page is drawn from.
type figureData struct {
	Layer    string
	Name     string
	Path     string
	Broken   string
	Teams    []pickTeam
	View     figure
	Banner   *banner
	NoArrows bool
	Legend   []string
}

// banner is the line above the figure when Herdr says the definition runs.
type banner struct {
	Team    string
	Verdict string
}

// figureLegend is what the line under the figure says each shape is.
func figureLegend() []string {
	return []string{
		i18n.T("web.teamview.figure.legend.doc"),
		i18n.T("web.teamview.figure.legend.role"),
		i18n.T("web.teamview.figure.legend.member"),
		i18n.T("web.teamview.figure.legend.worker"),
		i18n.T("web.teamview.figure.legend.arrow"),
		i18n.T("web.teamview.figure.legend.dotted"),
		i18n.T("web.teamview.figure.legend.pill"),
	}
}

// figure is one team definition ready to write as SVG: every box with the lines
// it writes, and every arrow as path data. Nothing in the template lays this
// out; the numbers come from internal/topology and from here.
type figure struct {
	Width, Height      float64
	Band               topology.Box
	CaptionX, CaptionY float64
	FeedX1             float64 // where the lines to the members leave the band
	Docs               []docBox
	Human              labelBox
	Members            []memberBox
	Strangers          []labelBox
	Arrows             []arrow
	Unread             []unread
}

// labelBox is one box of the figure and the two lines it writes: the name, and
// the line under it as a person reads it.
type labelBox struct {
	Box          topology.Box
	Name         string
	Sub          string
	NameX, NameY float64
	SubX, SubY   float64
}

// docBox is one document in the band of documents every member reads.
type docBox struct {
	labelBox
	Key   string
	Rules int
}

// memberBox is one member's row: its box, its role file, the workers it may
// hire, and the state Herdr reports for it.
type memberBox struct {
	labelBox
	Class    string // the colour of the role
	FeedY    float64
	State    string // empty when Herdr does not report the definition running
	Pill     pill
	RoleFile roleBox
	Workers  []workerBox
}

// roleBox is the role file of one member's row: the file when the member reads
// one, and the empty place it would sit in when it does not.
type roleBox struct {
	labelBox
	Has  bool
	Key  string
	Role string
}

// workerBox is one worker under its member, with the line that ties it there.
type workerBox struct {
	labelBox
	HireX, HireTop, HireBottom float64
}

// arrow is one turn direction ready to write as SVG: the curve, and the id
// label on it.
type arrow struct {
	DocKey    string
	RuleID    string
	Path      string
	Both      bool
	Dotted    bool
	Title     string
	LabelX    float64 // the label's left edge
	LabelTop  float64
	LabelMid  float64 // the middle of the label, where its text sits
	LabelBase float64
	LabelW    float64
}

// unread is one turn direction the figure could not draw, as the list under it
// writes it.
type unread struct {
	DocKey  string
	RuleID  string
	Text    string
	Missing []string
}

// figureOf turns the layout into what the template writes.
func figureOf(layout topology.Figure, def Definition, live *LiveTeam, states map[string]status.Row) figure {
	out := figure{
		Width:    layout.Width,
		Height:   layout.Height,
		Band:     layout.Shared,
		CaptionX: layout.Shared.X + 12,
		CaptionY: layout.Shared.Y + 22,
		FeedX1:   layout.Shared.X + layout.Shared.W,
		Human: labelBox{
			Box:   layout.Human,
			Name:  i18n.T("web.teamview.figure.human"),
			Sub:   i18n.T("web.teamview.figure.human_note"),
			NameX: layout.Human.X + 22, NameY: layout.Human.CY() + 5,
			SubX: layout.Human.X + 66, SubY: layout.Human.CY() + 5,
		},
	}
	for _, doc := range layout.Docs {
		sub := i18n.Tf("web.teamview.doc.rules", doc.Rules)
		if source, ok := def.Doc(doc.Key); ok && source.Broken != "" {
			sub = i18n.T("web.teamview.figure.doc_unread")
		}
		out.Docs = append(out.Docs, docBox{
			Key: doc.Key, Rules: doc.Rules,
			labelBox: labelBox{
				Box: doc.Box, Name: doc.Label, Sub: sub,
				NameX: doc.X + 10, NameY: doc.Y + 16,
				SubX: doc.X + 10, SubY: doc.Y + 32,
			},
		})
	}
	classes := roleClass(def)
	for i, member := range layout.Members {
		box := memberBox{
			labelBox: labelBox{
				Box: member.Box, Name: member.Name, Sub: memberSub(def, member, live, states),
				NameX: member.X + 18, NameY: member.Y + 27,
				SubX: member.X + 18, SubY: member.Y + 47,
			},
			Class: classes[i],
			FeedY: member.CY(),
			RoleFile: roleBox{
				labelBox: labelBox{
					Box: member.RoleSlot, Name: member.Role, Sub: i18n.T("label.role_file"),
					NameX: member.RoleSlot.X + 10, NameY: member.RoleSlot.CY() - 3,
					SubX: member.RoleSlot.X + 10, SubY: member.RoleSlot.CY() + 13,
				},
				Has:  member.RoleFile != nil,
				Key:  member.RoleFileKey,
				Role: member.Role,
			},
		}
		if box.RoleFile.Has && box.RoleFile.Name == "" {
			box.RoleFile.Name = i18n.T("label.role_file")
		}
		if !box.RoleFile.Has {
			box.RoleFile.Name = i18n.T("web.teamview.figure.no_role_file")
			box.RoleFile.Sub = ""
		}
		for _, worker := range member.Workers {
			box.Workers = append(box.Workers, workerBox{
				labelBox: labelBox{
					Box: worker.Box, Name: worker.Purpose, Sub: dot(i18n.T("label.worker"), worker.Profile),
					NameX: worker.X + 9, NameY: worker.Y + 15,
					SubX: worker.X + 9, SubY: worker.Y + 30,
				},
				HireX:      worker.X + worker.W/2,
				HireTop:    member.Y + member.H,
				HireBottom: worker.Y,
			})
		}
		if live != nil {
			row := states[herdrName(live, def.Name, member.Name)]
			box.State = stateWord(row)
			box.Pill = newPill(row, member.X, member.Y, member.W)
		}
		out.Members = append(out.Members, box)
	}
	for _, stranger := range layout.Strangers {
		out.Strangers = append(out.Strangers, labelBox{
			Box: stranger.Box, Name: stranger.Word, Sub: i18n.T("web.teamview.figure.stranger"),
			NameX: stranger.X + 18, NameY: stranger.Y + 27,
			SubX: stranger.X + 18, SubY: stranger.Y + 47,
		})
	}
	out.Arrows = arrowsOf(layout, def)
	for _, dropped := range layout.Unread {
		out.Unread = append(out.Unread, unread{
			DocKey: dropped.DocKey, RuleID: dropped.RuleID,
			Text: dropped.Text, Missing: dropped.Missing,
		})
	}
	return out
}

// memberSub is the line under a member's name in the figure: the role it reads,
// the profile or engine it runs, and the pane it sits in when Herdr has it.
func memberSub(def Definition, box topology.MemberBox, live *LiveTeam, states map[string]status.Row) string {
	parts := []string{box.Role}
	if member, ok := def.Member(box.Name); ok {
		if member.Profile != "" {
			parts = append(parts, member.Profile)
		} else {
			parts = append(parts, member.Kind)
		}
	}
	if live != nil {
		if row, ok := states[herdrName(live, def.Name, box.Name)]; ok && row.PaneID != "" {
			parts = append(parts, i18n.Tf("web.teamview.running.pane", row.PaneID))
		}
	}
	return dot(parts...)
}

// arrowsOf puts each turn direction's curve in the figure's own coordinates: it
// leaves the member column's right edge, bends at its own lane, and comes back
// to the row it points at. SVG cannot bend a line, so the curve is written as
// data here.
func arrowsOf(layout topology.Figure, def Definition) []arrow {
	out := make([]arrow, 0, len(layout.Edges))
	for _, edge := range layout.Edges {
		x0 := layout.MemberRight
		bend := (edge.LaneX - 0.25*x0) / 0.75
		// The label is as wide as internal/topology measured it when it told
		// the labels apart, so the box that is drawn matches the gap that was
		// left for it.
		width := float64(len(edge.RuleID))*7.4 + 14
		out = append(out, arrow{
			DocKey: edge.DocKey, RuleID: edge.RuleID,
			Both: edge.Both, Dotted: edge.Dotted,
			Path: "M" + num(x0) + " " + num(edge.FromY) +
				" C" + num(bend) + " " + num(edge.FromY) +
				" " + num(bend) + " " + num(edge.ToY) +
				" " + num(x0) + " " + num(edge.ToY),
			Title: i18n.Tf("web.teamview.figure.arrow",
				edge.RuleID, sideLabel(def, edge.From), arrowWord(edge.Both), sideLabel(def, edge.To)),
			LabelX:    edge.LaneX - width/2,
			LabelW:    width,
			LabelTop:  edge.LabelY - 10,
			LabelMid:  edge.LaneX,
			LabelBase: edge.LabelY + 4,
		})
	}
	return out
}

// sideLabel is one end of an arrow as a person reads it: the member's name, the
// word a rule wrote for a party that is not in the team, or the word for the
// human.
func sideLabel(def Definition, side topology.Side) string {
	switch side.Kind {
	case topology.SideMember:
		if side.Member >= 0 && side.Member < len(def.Members) {
			return def.Members[side.Member].Name
		}
	case topology.SideStranger:
		return side.Word
	}
	return i18n.T("web.teamview.figure.human")
}

func arrowWord(both bool) string {
	if both {
		return "⇄"
	}
	return "→"
}

// roleClass is the colour class of each member's box: the roles in the order
// they first appear, four colours around.
func roleClass(def Definition) []string {
	seen := map[string]int{}
	out := make([]string, len(def.Members))
	for i, member := range def.Members {
		at, ok := seen[member.Role]
		if !ok {
			at = len(seen) % 4
			seen[member.Role] = at
		}
		out[i] = "c" + strconv.Itoa(at)
	}
	return out
}

// topologyMembers turns a definition into the members the figure draws: each
// with the short name the turn directions are written with, its role, and the
// workers it may hire.
func topologyMembers(def Definition) []topology.Member {
	out := make([]topology.Member, len(def.Members))
	for i, member := range def.Members {
		workers := make([]topology.Worker, len(member.Workers))
		for j, worker := range member.Workers {
			workers[j] = topology.Worker{Purpose: worker.Purpose, Profile: worker.Profile}
		}
		out[i] = topology.Member{Name: member.Name, Role: member.Role, Workers: workers}
	}
	return out
}

// topologyDocs is every rule file the members read: the documents every member
// reads, then each member's role file. The turn directions are read from the
// text of all of them (spec/12).
func topologyDocs(def Definition) []topology.Doc {
	var out []topology.Doc
	seen := map[string]bool{}
	add := func(doc Doc, role string) {
		if seen[doc.Key] {
			return
		}
		seen[doc.Key] = true
		rules := make([]topology.Rule, len(doc.Rules))
		for i, rule := range doc.Rules {
			rules[i] = topology.Rule{ID: rule.ID, Text: rule.Text}
		}
		out = append(out, topology.Doc{Key: doc.Key, Label: doc.Label, Role: role, Rules: rules})
	}
	for _, doc := range def.Docs {
		add(doc, "")
	}
	for _, member := range def.Members {
		if member.RoleFile != nil {
			add(*member.RoleFile, member.Role)
		}
	}
	return out
}
