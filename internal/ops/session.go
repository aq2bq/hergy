package ops

import (
	"os"

	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/team"
)

// Port is the transport supplied by an entry point or a deterministic test.
type Port = herdr.Port

func portOrDefault(port Port) Port {
	if port != nil {
		return port
	}
	return herdrcli.New(os.Getenv("HERGY_HERDR"))
}

// Session is one herdr reading used by operation preconditions.
type Session struct {
	Tabs       []team.Tab
	Agents     []protocol.Agent
	Workspaces []herdr.Workspace
	AgentsRead bool
	Err        error
}

// ReadSession reads current state, without a screen cache or project filter.
func ReadSession(port Port) Session {
	port = portOrDefault(port)
	if p, ok := port.(interface {
		SessionSnapshot() (herdr.SessionSnapshot, error)
	}); ok {
		snap, err := p.SessionSnapshot()
		return Session{Tabs: TeamTabs(snap.Tabs, snap.Agents, snap.Panes), Agents: snap.Agents, Workspaces: snap.Workspaces, AgentsRead: true, Err: err}
	}
	h := herdr.New(port)
	tabs, err := h.Tabs()
	if err != nil {
		return Session{Err: err}
	}
	panes, _ := h.Panes()
	agents, agentErr := h.LiveAgents()
	return Session{Tabs: TeamTabs(tabs, agents, panes), Agents: agents, AgentsRead: agentErr == nil}
}
