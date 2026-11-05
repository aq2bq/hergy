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
