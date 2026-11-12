import { mock } from 'bun:test';
export async function promptClock() {
  const source = await import('../src/teams/prompt-source.js');
  mock.module('../src/teams/prompt-source.js', () => ({ ...source, ...source.createPromptSource({ wait: 0 }) }));
}
