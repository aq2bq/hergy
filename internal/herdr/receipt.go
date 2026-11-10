package herdr

// ConfirmPrompt owns confirmed sending (d-b4b5, r-990d): herdr must observe
// working within five seconds. Retry the identical text once on any error,
// returning the last answer. Callers decide what to do with that answer; the
// watcher gives the turn to the human, while a launch distinguishes an absent
// receipt from a failure to send (d-b804).
func (h *Herdr) ConfirmPrompt(target, text string) error {
	var err error
	for range 2 {
		if err = h.PromptWait(target, text, 5000); err == nil {
			return nil
		}
	}
	return err
}
