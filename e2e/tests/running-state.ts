import { mkdirSync, renameSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { clockStart } from '../scripts/fixture.mjs';

// Register provenance separately from the fixture's history and herdr answers.
export function registerRunningTeam(
  app: { state: string; proj: string }, name: string, definition: string,
  layer = 'local', teamYAML = path.join(app.proj, '.hergy', 'teams', definition, 'team.yaml'),
): void {
  const dir = path.join(app.state, 'hergy', 'teams');
  mkdirSync(dir, { recursive: true });
  const file = path.join(dir, `${name}.json`);
  writeFileSync(file + '.tmp', JSON.stringify({
    generation: `fixture-${name}`, team: name, definition, project: app.proj,
    layer, team_yaml: teamYAML, workspace: { id: 'w1', label: 'workspace' },
    started_at: clockStart, executable: 'hergy', lang: 'ja',
    members: [], worker_definitions: [], workers: [],
  }));
  renameSync(file + '.tmp', file);
}
