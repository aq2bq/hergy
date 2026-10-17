package topology

import (
	"math"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
)

// graphX is where the member column starts when the figure draws a graph alone.
// The document band and the role column of the older figure are not drawn: they
// are not in the graph (C7, 段 4).
const graphX = 12.0

// workerPairGap is how far apart the two dotted arrows of one worker sit.
const workerPairGap = 12.0

// LayoutGraph lays out one definition's graph for drawing (C7, 段 4): the human
// at the top, the members with the workers they may hire under them, and one
// arrow per edge of the graph.
//
// It adds nothing to the graph. A thing that is not a node or an edge of it is
// not drawn: the boxes of the rule documents, the band every member reads, and
// an arrow read from a rule's text are all absent (段 5, d-11f8).
//
// The arrows are the graph's Edges in the order the graph holds them, so
// len(f.Edges) is len(g.Edges) and the i-th arrow draws the i-th edge. A worker's
// two edges (workers: makes them, I12c) are dotted and run between the member's
// box and the worker's. The older Layout draws the figure that reads a
// definition's members and its documents; it stays until the older screen is
// gone.
func LayoutGraph(g *definition.Graph) Figure {
	rows := graphRows(g)
	top := topMargin + humanH + humanGap
	bottom := stackGraphRows(rows, top)
	f := Figure{
		Human:   Box{X: graphX, Y: rows[0].Y, W: memberW, H: humanH},
		Members: make([]MemberBox, len(g.Members)),
		// The arrows leave the member column: its right edge is fixed, so a
		// screen can draw the curves without knowing the sizes.
		MemberRight: graphX + memberW,
	}
	for i, member := range g.Members {
		f.Members[i] = placeGraphMember(member, i, rows[1+i])
	}
	placeGraphWorkers(g.Workers, f.Members)
	f.Edges = placeGraphEdges(g, rows, f.Members)
	turns := 0
	for _, edge := range g.Edges {
		if !workerEdge(edge) {
			turns++
		}
	}
	f.Width = graphX + memberW + laneStart + math.Max(0, float64(turns-1))*laneW + rightPad
	f.Height = bottom + bottomPad
	return f
}

// nodeKey is how one node is told apart from another in a figure: its kind and
// its name. Only a member and the human key a row: a worker is drawn under the
// member that hires it.
func nodeKey(id definition.NodeID) string { return string(id.Kind) + ":" + id.Name }

// graphRows is the rows of a graph's figure, in drawn order: the human, then the
// members. A member's row is taller when a worker is drawn under it.
func graphRows(g *definition.Graph) []row {
	rows := make([]row, 0, len(g.Members)+1)
	rows = append(rows, row{key: nodeKey(g.Human.ID()), H: humanH})
	for _, member := range g.Members {
		row := row{key: nodeKey(member.ID()), H: memberH, anchorH: memberH}
		if len(workersOf(g.Workers, member.Name)) > 0 {
			row.H += workerStep
		}
		rows = append(rows, row)
	}
	return rows
}

// stackGraphRows puts the rows under the human, one after another, and returns
// how far down the figure goes. No band of documents stands beside them, so the
// lowest row is the end of the figure.
func stackGraphRows(rows []row, top float64) float64 {
	rows[0].Y = topMargin
	bottom := topMargin + rows[0].H
	y := top
	for i := 1; i < len(rows); i++ {
		rows[i].Y = y
		y += rows[i].H + rowGap
		bottom = rows[i].Y + rows[i].H
	}
	return bottom
}

// placeGraphMember puts one member's box. r carries the y the box starts at.
func placeGraphMember(member definition.MemberNode, index int, r row) MemberBox {
	return MemberBox{
		Box:     Box{X: graphX, Y: r.Y, W: memberW, H: memberH},
		Index:   index,
		Name:    member.Name,
		Role:    member.Role,
		Profile: member.Profile,
	}
}

// placeGraphWorkers puts each worker's box under the member that hires it, side
// by side, in the order the graph holds them.
func placeGraphWorkers(workers []definition.WorkerNode, members []MemberBox) {
	for i, member := range members {
		hired := workersOf(workers, member.Name)
		if len(hired) == 0 {
			continue
		}
		width := math.Min(workerMaxW, (memberW-2*workerPadX-float64(len(hired)-1)*8)/float64(len(hired)))
		for j, worker := range hired {
			members[i].Workers = append(members[i].Workers, WorkerBox{
				Box: Box{
					X: member.X + workerPadX + float64(j)*(width+8),
					Y: member.Y + memberH + workerGap,
					W: width,
					H: workerH,
				},
				Purpose: worker.Purpose,
				Profile: worker.Profile,
			})
		}
	}
}

// workersOf is the workers that call one member, in the order the graph holds
// them.
func workersOf(workers []definition.WorkerNode, member string) []definition.WorkerNode {
	var out []definition.WorkerNode
	for _, worker := range workers {
		if worker.From == member {
			out = append(out, worker)
		}
	}
	return out
}

// workerEdge reports whether a graph edge is one that workers: made: an edge
// with a worker on one end (I12c). A turn written in turns: names the human or a
// member of the team.
func workerEdge(edge definition.Edge) bool {
	return edge.From.Kind == definition.KindWorker || edge.To.Kind == definition.KindWorker
}

// placeGraphEdges places every edge of the graph: a written turn in a lane to the
// right of the members, a worker's edge between the member's box and the
// worker's. The arrows come out in the order the graph holds its edges.
func placeGraphEdges(g *definition.Graph, rows []row, members []MemberBox) []Arrow {
	index := rowIndex(rows)
	// The written turns, with the rows their ends meet and the lane each bends
	// in. A turn whose end is not a row of this figure is not drawn.
	var held []heldEdge
	for _, edge := range g.Edges {
		if workerEdge(edge) {
			continue
		}
		from, ok := index[nodeKey(edge.From)]
		if !ok {
			continue
		}
		to, ok := index[nodeKey(edge.To)]
		if !ok {
			continue
		}
		held = append(held, heldEdge{edge: edge, from: from, to: to})
	}
	assignLanes(held)
	fromY, toY := anchors(held, rows)
	seat := make(map[definition.Edge]int, len(held))
	for i, h := range held {
		seat[h.edge] = i
	}
	out := make([]Arrow, 0, len(g.Edges))
	for _, edge := range g.Edges {
		if workerEdge(edge) {
			out = append(out, workerArrow(edge, members))
			continue
		}
		i, ok := seat[edge]
		if !ok {
			continue
		}
		out = append(out, Arrow{
			FromNode: edge.From,
			ToNode:   edge.To,
			FromY:    fromY[i],
			ToY:      toY[i],
			LaneX:    graphX + memberW + laneStart + float64(held[i].lane)*laneW,
		})
	}
	return out
}

// workerArrow places one of the two edges workers: makes (I12c): a short dotted
// arrow between the member's box and the worker's, drawn beside its pair. When
// the figure holds no box for that worker the arrow is left without a place to
// meet: the edge is named, and a screen draws no line it cannot place.
func workerArrow(edge definition.Edge, members []MemberBox) Arrow {
	memberName, purpose := workerEnds(edge)
	out := Arrow{FromNode: edge.From, ToNode: edge.To, Worker: true}
	member, worker, ok := workerBox(members, memberName, purpose)
	if !ok {
		return out
	}
	bottom := member.Y + memberH
	if edge.From.Kind == definition.KindWorker {
		out.FromY, out.ToY = worker.Y, bottom
		out.LaneX = worker.CX() + workerPairGap/2
		return out
	}
	out.FromY, out.ToY = bottom, worker.Y
	out.LaneX = worker.CX() - workerPairGap/2
	return out
}

// workerEnds is the member that hires a worker edge and the purpose of the
// worker: one end of the edge is the worker, whose name is "<member>/<purpose>"
// (I12c).
func workerEnds(edge definition.Edge) (member, purpose string) {
	worker := edge.From
	if worker.Kind != definition.KindWorker {
		worker = edge.To
	}
	member, purpose, _ = strings.Cut(worker.Name, "/")
	return member, purpose
}

// workerBox finds one worker's box and the box of the member that hires it.
func workerBox(members []MemberBox, member, purpose string) (MemberBox, WorkerBox, bool) {
	for _, box := range members {
		if box.Name != member {
			continue
		}
		for _, worker := range box.Workers {
			if worker.Purpose == purpose {
				return box, worker, true
			}
		}
	}
	return MemberBox{}, WorkerBox{}, false
}
