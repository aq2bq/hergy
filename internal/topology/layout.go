package topology

import (
	"sort"

	"github.com/aq2bq/hergy/internal/definition"
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
	// Profile is the engine the member runs, by profile name (I7): a figure of
	// the graph names it under the role. It is the zero value for the older
	// figure, which draws the profile from the screen's own reading.
	Profile string
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
	// FromNode and ToNode are the two ends of a graph edge (C7): the node
	// identity a screen links from and writes as data-from and data-to. They are
	// the zero value for an arrow of the older figure, whose turns were read
	// from a rule's text and whose ends are the Turn.
	FromNode definition.NodeID
	ToNode   definition.NodeID
	// Worker says the arrow is one of the two edges workers: makes (I12c), which
	// a figure draws dotted.
	Worker bool
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

// row is one horizontal band of the figure: a party's key, and where it sits.
type row struct {
	key  string
	Y, H float64
	// anchorH is the height the arrow ends spread over, when it is not the
	// row's own height: a member's row is taller than its box when a worker is
	// drawn under it, and the arrows meet the box. Zero means the whole row.
	anchorH float64
}

// rowIndex maps each party's key to its row.
func rowIndex(rows []row) map[string]int {
	index := make(map[string]int, len(rows))
	for i, r := range rows {
		index[r.key] = i
	}
	return index
}

// assignLanes sorts the arrows and gives each its lane: the short ones bend
// nearest the members, and the long ones outside them.
func assignLanes(held []heldEdge) {
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
}

// heldEdge is one arrow with the rows its ends meet: the graph edge it draws
// (C7).
type heldEdge struct {
	edge     definition.Edge
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
	lanes := make([]int, len(held))
	for i, h := range held {
		lanes[i] = h.lane
	}
	ends := make([][]end, len(rows))
	for i, h := range held {
		ends[h.from] = append(ends[h.from], end{at: i, side: 's', other: h.to})
		ends[h.to] = append(ends[h.to], end{at: i, side: 't', other: h.from})
	}
	for r, list := range ends {
		if len(list) == 0 {
			continue
		}
		placeEnds(list, lanes, rows[r], r, fromY, toY)
	}
	return fromY, toY
}

// end is one arrow meeting one row.
type end struct {
	at    int
	side  byte
	other int
}

// placeEnds distributes one row's arrow ends along the row's height. lanes is
// each arrow's lane, in the order the arrows are held.
func placeEnds(list []end, lanes []int, r row, index int, fromY, toY []float64) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].other != list[j].other {
			return list[i].other < list[j].other
		}
		// The same far row: take the farther lane first from the near side, so
		// the curves nest.
		if list[i].other < index {
			return lanes[list[i].at] > lanes[list[j].at]
		}
		return lanes[list[i].at] < lanes[list[j].at]
	})
	limit := r.H - anchorPad
	center := r.Y + r.H/2
	if r.anchorH > 0 {
		limit = r.anchorH - anchorPad
		center = r.Y + r.anchorH/2
	}
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

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
