package turns

import (
	"maps"
	"slices"
	"time"
)

// ForgetAbsent removes parties outside the roster supplied by the caller.
// A missing holder cannot be asked, so its turn goes to the human.
func (st *State) ForgetAbsent(roster func(string) bool, at time.Time) bool {
	keep := func(name string) bool { return name == Human || roster(name) }
	changed := false
	retainParties(st.Opened, keep, &changed)
	retainParties(st.Started, keep, &changed)
	retainParties(st.Told, keep, &changed)
	retainParties(st.Asked, keep, &changed)
	retainParties(st.Rested, keep, &changed)
	retainParties(st.Pending, keep, &changed)
	retainParties(st.Handed, keep, &changed)
	retainParties(st.Waiting, keep, &changed)
	retainParties(st.Signaled, keep, &changed)
	for name, told := range st.Told {
		if !keep(told.To) {
			delete(st.Told, name)
			changed = true
		}
	}
	for name, openings := range st.Opened {
		kept := slices.DeleteFunc(openings, func(o Opening) bool { return !keep(o.By) })
		if len(kept) != len(openings) {
			changed = true
			if len(kept) == 0 {
				delete(st.Opened, name)
			} else {
				st.Opened[name] = kept
			}
		}
	}
	for name, question := range st.Asked {
		candidates := slices.DeleteFunc(question.Candidates, func(name string) bool { return !keep(name) })
		waiting := slices.DeleteFunc(question.Waiting, func(name string) bool { return !keep(name) })
		if len(candidates) != len(question.Candidates) || len(waiting) != len(question.Waiting) {
			question.Candidates, question.Waiting = candidates, waiting
			st.Asked[name], changed = question, true
		}
	}
	for name, messages := range st.Pending {
		kept := slices.DeleteFunc(messages, func(m Message) bool { return !keep(m.From) })
		if len(kept) != len(messages) {
			changed = true
			if len(kept) == 0 {
				delete(st.Pending, name)
			} else {
				st.Pending[name] = kept
			}
		}
	}
	if st.Holder != "" && !keep(st.Holder) {
		st.Unanswered(st.Holder, at)
		changed = true
	}
	return changed
}

func retainParties[V any](entries map[string]V, keep func(string) bool, changed *bool) {
	maps.DeleteFunc(entries, func(name string, _ V) bool {
		if keep(name) {
			return false
		}
		*changed = true
		return true
	})
}
