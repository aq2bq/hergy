package app

import (
	"github.com/aq2bq/hergy/internal/herdr"
)

type applyCallbackPort struct {
	herdr.Port
	onPrompt func()
	called   bool
}

func (p *applyCallbackPort) JSON(args ...string) (map[string]any, error) {
	if len(args) > 1 && args[0] == "agent" && args[1] == "prompt" && !p.called {
		p.called = true
		p.onPrompt()
	}
	return p.Port.JSON(args...)
}
