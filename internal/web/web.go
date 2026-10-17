// Package web serves hergy's screens over HTTP on this machine: the frame the
// screens are drawn in, the four bands of the navigation, and the projection of
// the domain model onto them.
//
// What it shows comes from the model, through one place: the catalog the app
// hands in reads every team definition, profile and rule document, and check,
// effect, activity and topology turn them into the sentences, the changes and
// the figure a screen draws. This package decides no rule of the model itself;
// it lays out what those functions returned (P1).
//
// The app hands two things in: the catalog, and a Port for the live side
// (herdr's teams, agents and pane tails, and the focus hergy may move). The top
// layer of hergy — team, herdr, the launcher — is reached only through that
// Port, so this package never imports it.
//
// The promises it keeps (設計 6 節「残す仕組み」):
//
//   - It listens on 127.0.0.1 only, so nothing off this machine reaches it.
//   - A request whose Host is not this server's own name is refused.
//   - A write is a POST that must carry the token of this run, and an Origin
//     that comes with it must be this server's own.
//   - Every answer carries "Content-Security-Policy: default-src 'self'", so a
//     page cannot pull in script or style from anywhere but this server.
//   - A write reads the file, checks the change, and writes only when the file
//     is still the one it read (source.WriteIfUnchanged).
package web

import (
	"net/url"
	"time"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/effect"
)

// Port is the live half of what the screens read: herdr's running teams, their
// agents and pane tails, gy's open questions, the launch record, and the one
// thing hergy serve changes outside its own files — where herdr's focus stands.
// The app reads team, herdr, launchlog and gy and writes them into these types,
// so this package stays out of the top layer.
type Port interface {
	// Running lists the teams running on this machine, each with the team
	// definition it was booted from (effect.Running).
	Running() ([]effect.Running, error)
	// Agents lists one team's live agents, by the names herdr knows them by.
	Agents(team string) ([]activity.Agent, error)
	// Tail is the last lines of one agent's pane, for the node that waits.
	Tail(herdrName string, lines int) (string, error)
	// Focus moves herdr's focus to one agent's pane.
	Focus(herdrName string) error
	// Questions lists gy's open questions for this project, empty when there is
	// no gy here (段 6, I17).
	Questions() ([]activity.Question, error)
	// Launched is one running team's launch record: when it was launched, the
	// hash of each member's prompt at that time, and the facts those prompts
	// were built with (段 6, C9). A team this machine has no record of answers
	// false.
	Launched(team string) (Launch, bool)
	// LaunchedDefinition reads the definition a team was booted from, from the
	// project the launch record names: it is how a team started from another
	// project is judged like any other (段 6, W16).
	LaunchedDefinition(team string) (*definition.Definition, error)
	// LaunchedView is one running team as effect reads it to compare a prompt
	// with the one the launch handed out: the definition and the facts it was
	// built with, read from the record's project, with that project's catalog
	// (C9, T1). A record from before those facts were kept answers false.
	LaunchedView(team string) (effect.View, bool)
}

// Launch is one running team's launch record as a screen reads it: the time it
// was launched, the hash of each member's prompt at that time (by the member's
// short name), and the facts those prompts were built with — the hergy binary,
// the project root and the language — so a prompt can be rebuilt the way the
// launch built it (C9). A record from before the facts were kept carries none of
// them.
type Launch struct {
	At         time.Time
	Prompts    map[string]string
	Executable string
	Project    string
	Lang       string
}

// partCount is how many pane lines the activity shows for a node that waits
// (設計 5.1).
const partCount = 5

// The bands of the navigation (設計 1 節、bar.md A1): four first-class concepts,
// in this order. A screen that is not in a band (a definition's own section) is
// reached from it, and the band of the screen it stands under stays marked.
const (
	bandActivity    = "activity"
	bandDefinitions = "definitions"
	bandRoles       = "roles"
	bandProfiles    = "profiles"
)

// band is one entry of the navigation.
type band struct {
	Name   string
	Href   string
	Active bool
}

// bandsOf is the navigation, with one entry marked as where the person is.
func bandsOf(current string) []band {
	titles := []struct{ key, href, name string }{
		{"web.band.activity", "/activity", bandActivity},
		{"web.band.definitions", "/definitions", bandDefinitions},
		{"web.band.roles", "/roles", bandRoles},
		{"web.band.profiles", "/profiles", bandProfiles},
	}
	out := make([]band, 0, len(titles))
	for _, entry := range titles {
		out = append(out, band{
			Name:   t(entry.key),
			Href:   entry.href,
			Active: entry.name == current,
		})
	}
	return out
}

// crumb is one step of the trail above the body: what it is called, and where it
// leads. The last step carries no href and is written as text (設計 2 節).
type crumb struct {
	Name string
	Href string
}

// The paths of the screens. Every one is a real path, so a deep link and a
// reload answer the whole page without JavaScript (設計 2 節 URL).
func definitionPath(name string) string { return "/definitions/" + url.PathEscape(name) }

func memberPath(def, member string) string {
	return definitionPath(def) + "/members/" + url.PathEscape(member)
}

func workerPath(def, from, purpose string) string {
	return definitionPath(def) + "/workers/" + url.PathEscape(from) + "/" + url.PathEscape(purpose)
}

func humanPath(def string) string { return definitionPath(def) + "/human" }

func turnPath(def, from, to string) string {
	return definitionPath(def) + "/turns/" + url.PathEscape(from) + "/" + url.PathEscape(to)
}

func agreementPath(def string) string { return definitionPath(def) + "/agreement" }

func rulePath(def, id string) string { return agreementPath(def) + "/" + url.PathEscape(id) }

func rolePath(role string) string { return "/roles/" + url.PathEscape(role) }

func roleRulePath(role, id string) string { return rolePath(role) + "/" + url.PathEscape(id) }

func profilePath(name string) string { return "/profiles/" + url.PathEscape(name) }

func teamPath(name string) string { return "/activity/" + url.PathEscape(name) }
