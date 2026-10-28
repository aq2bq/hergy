// TEAMS-B2, a team without an agreement.yaml and 元に戻す (n-e3de): the row
// opens on the empty rules document defOf makes, and a rule added to it is put
// back to that document — not to null, which the panel cannot draw. The answer
// is the test's, so what the screen makes of it is what is read.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';

// island is a team whose definition has no agreement.yaml: its place holds
// team.yaml, and the answer's file list is empty.
const island = {
  dirs: { project: '.hergy/teams', global: '~/.config/hergy/teams' },
  profiles: ['coder'],
  team: {
    items: [{
      name: 'core', effective: 'project',
      places: { project: { path: '.hergy/teams/core/team.yaml', hash: 'h1', doc: {
        mission: '注文 API を作り直す',
        members: [{ name: 'lead', role: 'lead', profile: 'coder' }],
        workers: [],
        turns: [['human', 'lead'], ['lead', 'human']],
      } } },
    }],
    files: [],
  },
};

// answer is what POST /api/team/check answers: no problems, an empty preview,
// and the effect a check has.
const answer = {
  problems: [],
  preview: { lines: [], diff: { added: [], removedBefore: [], removedAtEnd: false } },
  effect: { diff: { added: [], removedBefore: [], removedAtEnd: false }, turns: [] },
  readers: [],
};

test('a team with no agreement goes back to the document it opened with', async () => {
  history.replaceState({}, '', '/teams');
  layout('teams-data', island);
  serve((method, url) => {
    if (url.startsWith('/api/team/check')) return answer;
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await import('../src/teams/main');
  const state = await import('../src/teams/state');
  await settle();

  const r = state.allRecs()[0];
  expect(r, 'the definition is a row').toBeTruthy();
  // The row opened with no agreement: its draft is the empty document.
  expect(r.agreeSaved.value).toBe(null);
  expect(r.agreeDraft.value.title).toBe('core チームのルール');
  expect(r.agreeDraft.value.sections.length).toBe(0);

  // A rule is added to the draft, and 元に戻す is pressed.
  r.agreeDraft.value = {
    ...r.agreeDraft.value,
    sections: [{ heading: '仕事', level: 0, intro: '', rules: [{ id: 'NEW-1', _k: 900, text: '足した。', note: '', in_prompt: true }] }],
  };
  state.pressRevert(r);
  await settle();

  // The document is the one it opened with, not null.
  expect(r.agreeDraft.value).not.toBe(null);
  expect(r.agreeDraft.value.title).toBe('core チームのルール');
  expect(r.agreeDraft.value.sections).toEqual([]);
});
