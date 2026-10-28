// Package layout plans the panes of a new tab.
package layout

// Step is one pane operation, in the order Herdr must run them.
type Step interface{ isStep() }

// Split splits the pane `Source` holds and gives the new pane to `Target`.
type Split struct {
	Source    string
	Target    string
	Direction string
}

func (Split) isStep() {}

// Rename names a pane after its member.
type Rename struct{ Member string }

func (Rename) isStep() {}

// Direction is the way one level of the plan splits: right at even depths, down
// at odd ones. A member added to a tab that is already laid out is split in at
// the same alternation, so a pane made later is not always cut from the same side.
func Direction(depth int) string {
	if depth%2 == 0 {
		return "right"
	}
	return "down"
}

// Plan is a balanced binary tree: halve the roster, alternate right/down.
// A three-member roster becomes left one / right two stacked.
func Plan(members []string, depth int) []Step {
	if len(members) == 1 {
		return []Step{Rename{Member: members[0]}}
	}
	midpoint := len(members) / 2
	first := members[:midpoint]
	second := members[midpoint:]
	steps := []Step{Split{Source: first[0], Target: second[0], Direction: Direction(depth)}}
	steps = append(steps, Plan(first, depth+1)...)
	steps = append(steps, Plan(second, depth+1)...)
	return steps
}
