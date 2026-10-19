package profile

import "maps"

// ActorEnv is the environment variable hergy sets in every pane it makes: the
// name the agent writes into gy as the writer of what it records. A profile may
// write it, but hergy's value wins, because the name a pane answers to is
// hergy's to say (r-1c42).
const ActorEnv = "GY_ACTOR"

// PaneEnv is the environment a pane is made with: the profile's own env, with
// hergy's GY_ACTOR set to the herdr name the pane answers to ("<team>-<member>"
// or "<team>-<worker>"). A profile that writes GY_ACTOR does not win; validate
// warns about it, because the pane never sees that value (r-1c42).
func PaneEnv(env map[string]string, herdrName string) map[string]string {
	out := make(map[string]string, len(env)+1)
	maps.Copy(out, env)
	out[ActorEnv] = herdrName
	return out
}
