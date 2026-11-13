import { mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
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

// Record the actual dry-built body separately from history's blob references.
export function receiveRunningPrompts(
  app: { state: string; proj: string }, team: string,
  members: { name: string; prompt: string; sources?: string[] }[],
): void {
  const file = path.join(app.state, 'hergy', 'teams', `${team}.json`);
  const state = JSON.parse(readFileSync(file, 'utf8'));
  state.members = members.map((member) => ({
    name: member.name, short: member.name.slice(team.length + 1),
    prompt: member.prompt, command: 'up', time: clockStart,
    executable: 'hergy', lang: 'ja', start: '', env: '',
    docs: (member.sources ?? []).map((source) => {
      const file = path.isAbsolute(source) ? source : path.join(app.proj, source);
      return { path: file, layer: 'local', text: readFileSync(file, 'utf8') };
    }),
  }));
  writeFileSync(file + '.tmp', JSON.stringify(state));
  renameSync(file + '.tmp', file);
}
