package ops

import (
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
)

func memberReceipts(state runningstate.State) map[string]runningstate.Receipt {
	out := map[string]runningstate.Receipt{}
	for _, receipt := range state.Members() {
		out[receipt.Name] = receipt
	}
	return out
}

// lastPromptReceipt reads only this run's exact definition and member.
func lastPromptReceipt(state runningstate.State, project, target domainplace.Project, definition, member string) (runningstate.Receipt, bool) {
	launch := state.Launch()
	if launch.Definition.String() != definition || launch.Project.Compare(project) != domainplace.Same || launch.TeamYAML.Compare(target) != domainplace.Same {
		return runningstate.Receipt{}, false
	}
	for _, receipt := range state.Members() {
		if receipt.Short == member {
			return receipt, true
		}
	}
	return runningstate.Receipt{}, false
}
