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

// Plan is a balanced binary tree: halve the roster, alternate right/down.
// A three-member roster becomes left one / right two stacked.
func Plan(members []string, depth int) []Step {
	if len(members) == 1 {
		return []Step{Rename{Member: members[0]}}
	}
	midpoint := len(members) / 2
	first := members[:midpoint]
	second := members[midpoint:]
	direction := "down"
	if depth%2 == 0 {
		direction = "right"
	}
	steps := []Step{Split{Source: first[0], Target: second[0], Direction: direction}}
	steps = append(steps, Plan(first, depth+1)...)
	steps = append(steps, Plan(second, depth+1)...)
	return steps
}
