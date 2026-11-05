import { signal } from '../../lib/preact.js';

// promptRun is the running team the prompt column and the whole-text panel both
// build with, per definition (r-0b69): the comparison panel writes the team it
// shows, and the column reads it as running_team. Empty means nothing was
// chosen, and the server then builds with the definition's own name.
export const promptRun = signal<Record<string, string>>({});
export const runOf = (key: string): string => promptRun.value[key] || '';
export function setRun(key: string, team: string): void {
  if (runOf(key) === team) return;
  promptRun.value = { ...promptRun.value, [key]: team };
}

// keepRun follows one answer for both panels: a team the person picked that is
// no longer running is let go, and a single running team the server took alone
// is taken here too. So neither panel keeps a stopped team's name, and both
// name the same team (r-0b69).
export function keepRun(key: string, answer: { runs?: { team: string }[]; selected?: { team: string } | null }): void {
  const runs = answer.runs || [];
  const picked = runOf(key);
  if (picked && !runs.some((run) => run.team === picked)) { setRun(key, ''); return; }
  if (!picked && answer.selected) setRun(key, answer.selected.team);
}
