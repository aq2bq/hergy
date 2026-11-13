package ops

import (
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/launcher"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
	"github.com/aq2bq/hergy/internal/source"
)

func operationTime(now func() time.Time) time.Time {
	if now != nil {
		return now()
	}
	return time.Now()
}

func beginRunningState(store runningstate.Store, env Environment, r *Resolved, workspace runningstate.Workspace, now func() time.Time) error {
	launch := runningstate.Launch{Definition: r.Team.Definition.Name, Project: env.ProjectRoot,
		Layer: domainplace.Selection(r.Team.Definition.Layer), TeamYAML: r.Team.Definition.TeamYAML.Project(),
		Workspace: workspace, StartedAt: operationTime(now), Executable: executable(), Lang: i18n.Language()}
	for _, worker := range r.Team.Definition.Graph.Workers {
		p, ok := r.Profiles[worker.Profile]
		if !ok {
			continue
		}
		launch.Workers = append(launch.Workers, runningstate.WorkerDefinition{
			From: worker.From, Purpose: worker.Purpose.String(), Profile: worker.Profile,
			Start: launchlog.StartFingerprint(p.Kind, p.Args), Env: launchlog.EnvFingerprint(p.Env)})
	}
	return store.Begin(r.ModelTeamName(), launch)
}

func receiveRunningState(r *Resolved, command string, members []definition.Start, now func() time.Time) error {
	if len(members) == 0 {
		return nil
	}
	store, err := runningstate.Open()
	if err != nil {
		return err
	}
	// Read no rules and create no state for teams that predate this store.
	if _, exists, err := store.Read(r.ModelTeamName()); err != nil || !exists {
		return err
	}
	at := operationTime(now)
	receipts := make([]runningstate.Receipt, 0, len(members))
	for i := range members {
		member := &members[i]
		receipt := runningstate.Receipt{Name: member.Name.String(), Short: member.Short.String(),
			Prompt: member.Prompt, Command: command, Time: at, Executable: executable(), Lang: i18n.Language(),
			Start: launchlog.StartFingerprint(member.Kind, member.Args), Env: launchlog.EnvFingerprint(member.Env)}
		for _, doc := range docsReadBy(member) {
			text, err := source.Disk{}.Read(doc)
			if err != nil {
				return err
			}
			receipt.Docs = append(receipt.Docs, runningstate.Doc{Path: doc.Path, Layer: doc.Layer, Text: string(text)})
		}
		receipts = append(receipts, receipt)
	}
	return store.Receive(r.ModelTeamName(), receipts)
}

func hireRunningState(r *Resolved, worker definition.Start, now func() time.Time) error {
	store, err := runningstate.Open()
	if err != nil {
		return err
	}
	workerName, _ := domainname.ParseWorkerName(worker.Short.String())
	return store.Hire(r.ModelTeamName(), runningstate.Worker{Name: worker.Name.String(), From: worker.From.String(),
		Purpose: worker.Purpose, Tab: r.ModelTeamName().WorkerTab(workerName).String(), Time: operationTime(now)})
}

func runningStateWarning(err error) string {
	if err != nil {
		return i18n.Tf("launch.not_recorded", err)
	}
	return ""
}

// Legacy observed team names need not be valid file keys; they have no new state.
func endRunningState(store runningstate.Store, raw string) error {
	name, err := domainname.ParseTeamName(raw)
	if err != nil {
		return nil
	}
	return store.End(name)
}

// prepareRunningState installs the start operation's write at the launcher's
// workspace boundary; cleanup removes it only if no tab was created.
func prepareRunningState(l *launcher.Launcher, in StartInput, r *Resolved, workspace WorkspaceInfo) func(bool) {
	store, stateErr := runningstate.Open()
	began := false
	l.BeforeBoot(func(id, label string) error {
		if stateErr != nil {
			return nil
		}
		if label == "" {
			label = workspace.Label
		}
		stateErr = beginRunningState(store, in.Env, r, runningstate.Workspace{ID: id, Label: label}, in.Now)
		began = stateErr == nil
		// State storage follows the existing startup contract: unavailable state
		// cannot stop a boot. Its failure is reported after the operation.
		return nil
	})
	return func(warned bool) {
		if !warned {
			warning(in.Observe, runningStateWarning(stateErr))
		}
		if began && !l.CreatedTeamTab() {
			warning(in.Observe, runningStateWarning(store.End(r.ModelTeamName())))
		}
	}
}

func clearRunningState(raw string) error {
	store, err := runningstate.Open()
	if err != nil {
		return err
	}
	return endRunningState(store, raw)
}
