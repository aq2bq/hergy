package topology

import (
	"math"
	"sort"
)

// The figure's fixed sizes, in pixels. The columns run left to right: the
// documents every member reads, the role files, the members, and then the
// arrows. The rows run top to bottom: the human, then the members. These are the
// sizes the prototype settled on; the screen draws at these positions.
const (
	sharedX    = 12.0
	sharedW    = 176.0
	roleX      = 212.0
	roleW      = 128.0
	memberX    = 364.0
	memberW    = 380.0
	memberH    = 62.0
	humanH     = 46.0
	roleH      = 42.0
	workerH    = 36.0
	workerGap  = 14.0
	workerMaxW = 132.0
	workerPadX = 18.0
	workerStep = 52.0 // how much a member's row grows by its workers
	rowGap     = 26.0
	topMargin  = 14.0
	humanGap   = 40.0
	docX       = 8.0
	docY       = 34.0
	docH       = 38.0
	docStep    = 46.0
	bandTopPad = 44.0
	laneStart  = 40.0
	laneW      = 30.0
	rightPad   = 40.0
	anchorPad  = 22.0
	bottomPad  = 18.0
)

// Box is a rectangle in the figure's coordinate space, in pixels.
type Box struct {
	X, Y, W, H float64
}

// CX is the x of the box's center.
func (b Box) CX() float64 { return b.X + b.W/2 }

// CY is the y of the box's center.
func (b Box) CY() float64 { return b.Y + b.H/2 }

// DocBox is one document in the band of documents every member reads.
type DocBox struct {
	Box
	Key   string
	Label string
	Rules int
}

// WorkerBox is one worker, drawn under the member that may hire it.
type WorkerBox struct {
	Box
	Purpose string
	Profile string
}

// MemberBox is one member's row: its box, the role file it reads when it has
// one, and the workers it may hire.
type MemberBox struct {
	Box
	Index       int
	Name        string
	Role        string
	RoleFile    *Box
	RoleFileKey string
	// RoleSlot is where a role file box sits on this member's row, whether or
	// not the member reads one: a screen draws an empty box there to say the
	// member reads no role file.
	RoleSlot Box
	Workers  []WorkerBox
}

// StrangerBox is a party no team name matched, drawn with the written word.
type StrangerBox struct {
	Box
	Word string
}

// Turn is one edge of the graph the figure draws: one node hands the turn to
// another. The ends are members of this definition, or the human. The figure
// draws the graph and adds nothing to it (段 5): a turn that is not in turns:
// is not in the figure.
type Turn struct {
	From Side
	To   Side
}

// Arrow is one turn placed in the figure: where it meets each row, and the lane
// it bends in.
type Arrow struct {
	Turn
	FromY float64
	ToY   float64
	LaneX float64
}

// Figure is one team definition laid out for drawing.
type Figure struct {
	Width, Height float64
	Shared        Box
	Docs          []DocBox
	Human         Box
	Members       []MemberBox
	Edges         []Arrow
	// MemberRight is the x where the member column ends: the edge the turn
	// arrows leave from and come back to. The screen draws the curves; this is
	// where they start.
	MemberRight float64
}

// Layout lays out one team definition: the boxes, and the arrows the graph's
// turns make. It reads no rule text: the turns are the definition's own (段 5).
func Layout(members []Member, docs []Doc, turns []Turn) Figure {
	rows := buildRows(members)
	top := topMargin + humanH + humanGap
	bottom := stackRows(rows, top, len(sharedDocs(docs)))
	f := Figure{
		Shared:  Box{X: sharedX, Y: top, W: sharedW, H: bottom - top},
		Docs:    placeDocs(docs, top),
		Human:   Box{X: memberX, Y: rows[0].Y, W: memberW, H: humanH},
		Members: make([]MemberBox, len(members)),
		// The arrows leave the member column: its right edge is fixed, so the
		// screen can draw the curves without knowing the sizes.
		MemberRight: memberX + memberW,
	}
	for i := range members {
		f.Members[i] = placeMember(members[i], i, rows[1+i], docs)
	}
	index := rowIndex(rows)
	f.Edges = placeEdges(turns, rows, index)
	f.Width = memberX + memberW + laneStart + math.Max(0, float64(len(f.Edges)-1))*laneW + rightPad
	f.Height = bottom + bottomPad
	return f
}

// row is one horizontal band of the figure: a party's key, and where it sits.
type row struct {
	key  string
	Y, H float64
}

// buildRows is the rows in drawn order: the human, then the members.
func buildRows(members []Member) []row {
	rows := make([]row, 0, len(members)+1)
	rows = append(rows, row{key: Side{Kind: SideHuman}.Key(), H: humanH})
	for i, member := range members {
		h := memberH
		if len(member.Workers) > 0 {
			h += workerStep
		}
		rows = append(rows, row{key: Side{Kind: SideMember, Member: i}.Key(), H: h})
	}
	return rows
}

// stackRows puts the rows under the human, one after another, and returns how
// far down the figure goes: the last row, or the end of the band of documents,
// whichever is lower.
func stackRows(rows []row, top float64, docs int) float64 {
	rows[0].Y = topMargin
	y := top
	for i := 1; i < len(rows); i++ {
		rows[i].Y = y
		y += rows[i].H + rowGap
	}
	band := top + bandTopPad + float64(docs)*docStep
	return math.Max(y-rowGap, band)
}

// sharedDocs is the documents every member reads: the ones with no role.
func sharedDocs(docs []Doc) []Doc {
	var out []Doc
	for _, doc := range docs {
		if doc.Role == "" {
			out = append(out, doc)
		}
	}
	return out
}

// placeDocs puts the shared documents in the band, top to bottom.
func placeDocs(docs []Doc, top float64) []DocBox {
	shared := sharedDocs(docs)
	out := make([]DocBox, len(shared))
	for i, doc := range shared {
		out[i] = DocBox{
			Box:   Box{X: sharedX + docX, Y: top + docY + float64(i)*docStep, W: sharedW - 2*docX, H: docH},
			Key:   doc.Key,
			Label: doc.Label,
			Rules: len(doc.Rules),
		}
	}
	return out
}

// placeMember puts one member's box, its role file and its workers. row carries
// the y the member's box starts at.
func placeMember(member Member, index int, row row, docs []Doc) MemberBox {
	box := Box{X: memberX, Y: row.Y, W: memberW, H: memberH}
	slot := Box{X: roleX, Y: box.CY() - roleH/2, W: roleW, H: roleH}
	out := MemberBox{Box: box, Index: index, Name: member.Name, Role: member.Role, RoleSlot: slot}
	if doc, ok := roleFile(member.Role, docs); ok {
		file := slot
		out.RoleFile = &file
		out.RoleFileKey = doc.Key
	}
	if n := len(member.Workers); n > 0 {
		w := math.Min(workerMaxW, (memberW-2*workerPadX-float64(n-1)*8)/float64(n))
		for j, worker := range member.Workers {
			out.Workers = append(out.Workers, WorkerBox{
				Box: Box{
					X: memberX + workerPadX + float64(j)*(w+8),
					Y: box.Y + memberH + workerGap,
					W: w,
					H: workerH,
				},
				Purpose: worker.Purpose,
				Profile: worker.Profile,
			})
		}
	}
	return out
}

// roleFile finds the role file a role reads: the document whose role is that
// role. Files are never merged, so the first one wins.
func roleFile(role string, docs []Doc) (Doc, bool) {
	for _, doc := range docs {
		if doc.Role != "" && doc.Role == role {
			return doc, true
		}
	}
	return Doc{}, false
}

// rowIndex maps each party's key to its row.
func rowIndex(rows []row) map[string]int {
	index := make(map[string]int, len(rows))
	for i, r := range rows {
		index[r.key] = i
	}
	return index
}

// placeEdges puts every arrow: its lane to the right of the members, and the y
// where it meets each row.
func placeEdges(turns []Turn, rows []row, index map[string]int) []Arrow {
	held := make([]heldEdge, 0, len(turns))
	for _, turn := range turns {
		from, ok := index[turn.From.Key()]
		if !ok {
			continue
		}
		to, ok := index[turn.To.Key()]
		if !ok {
			continue
		}
		held = append(held, heldEdge{turn: turn, from: from, to: to})
	}
	sort.SliceStable(held, func(i, j int) bool {
		si, sj := held[i].span(), held[j].span()
		if si != sj {
			return si < sj
		}
		return held[i].first() < held[j].first()
	})
	for i := range held {
		held[i].lane = i
	}
	fromY, toY := anchors(held, rows)
	return assemble(held, fromY, toY)
}

// heldEdge is one arrow with the rows its ends meet.
type heldEdge struct {
	turn     Turn
	from, to int
	lane     int
}

func (h heldEdge) span() int  { return abs(h.from - h.to) }
func (h heldEdge) first() int { return min(h.from, h.to) }

// anchors spreads the arrows that meet a row along that row, so two arrows on
// the same row do not land on one point. It returns each arrow's y on its from
// row and on its to row.
func anchors(held []heldEdge, rows []row) (fromY, toY []float64) {
	fromY, toY = make([]float64, len(held)), make([]float64, len(held))
	ends := make([][]end, len(rows))
	for i, h := range held {
		ends[h.from] = append(ends[h.from], end{at: i, side: 's', other: h.to})
		ends[h.to] = append(ends[h.to], end{at: i, side: 't', other: h.from})
	}
	for r, list := range ends {
		if len(list) == 0 {
			continue
		}
		placeEnds(list, held, rows[r], r, fromY, toY)
	}
	return fromY, toY
}

// end is one arrow meeting one row.
type end struct {
	at    int
	side  byte
	other int
}

// placeEnds distributes one row's arrow ends along the row's height.
func placeEnds(list []end, held []heldEdge, r row, index int, fromY, toY []float64) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].other != list[j].other {
			return list[i].other < list[j].other
		}
		// The same far row: take the farther lane first from the near side, so
		// the curves nest.
		if list[i].other < index {
			return held[list[i].at].lane > held[list[j].at].lane
		}
		return held[list[i].at].lane < held[list[j].at].lane
	})
	limit := r.H - anchorPad
	center := r.Y + r.H/2
	for j, e := range list {
		offset := 0.0
		if len(list) > 1 {
			offset = -limit/2 + limit*float64(j)/float64(len(list)-1)
		}
		if e.side == 's' {
			fromY[e.at] = center + offset
		} else {
			toY[e.at] = center + offset
		}
	}
}

// assemble writes each arrow out with its lane and label position, shifting a
// label down when it would sit on another label in a nearby lane.
func assemble(held []heldEdge, fromY, toY []float64) []Arrow {
	x0 := memberX + memberW
	out := make([]Arrow, 0, len(held))
	for i, h := range held {
		laneX := x0 + laneStart + float64(h.lane)*laneW
		out = append(out, Arrow{Turn: h.turn, FromY: fromY[i], ToY: toY[i], LaneX: laneX})
	}
	return out
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
