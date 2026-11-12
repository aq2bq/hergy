package ops

import (
	"os"

	"github.com/aq2bq/hergy/internal/activity"

	"github.com/aq2bq/hergy/internal/ops/internal/herdr"
	"github.com/aq2bq/hergy/internal/ops/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/ops/internal/protocol"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
)

// Port is the transport supplied by an entry point or a deterministic test.
type Port interface {
	Verify(string) error
	JSON(...string) (map[string]any, error)
	Text(...string) (string, error)
}

func portOrDefault(port Port) Port {
	if port != nil {
		return port
	}
	return herdrcli.New(os.Getenv("HERGY_HERDR"))
}

// session is one herdr reading used by operation preconditions.
type session struct {
	Seen       map[string]activity.Watch
	Tabs       []team.Tab
	Agents     []protocol.Agent
	Workspaces []herdr.Workspace
	AgentsRead bool
	Err        error
}

// readSession reads current state, without a screen cache or project filter.
func readSession(port Port) session {
	port = portOrDefault(port)
	if p, ok := port.(interface {
		SessionSnapshot() (herdr.SessionSnapshot, error)
	}); ok {
		snap, err := p.SessionSnapshot()
		return session{Tabs: teamTabs(snap.Tabs, snap.Agents, snap.Panes), Agents: snap.Agents, Workspaces: snap.Workspaces, AgentsRead: true, Err: err}
	}
	h := herdr.New(port)
	tabs, err := h.Tabs()
	if err != nil {
		return session{Err: err}
	}
	panes, _ := h.Panes()
	agents, agentErr := h.LiveAgents()
	return session{Tabs: teamTabs(tabs, agents, panes), Agents: agents, AgentsRead: agentErr == nil}
}
