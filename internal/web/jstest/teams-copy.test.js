// TEAMS-C4, a member is copied on the figure (r-c21b, d-48a9, ac-ef85): the
// member's panel offers 複製, and the press adds one member with the same role,
// profile, turns (both ways) and worker purposes, named <name>-2 and then -3, -4, …
// — as one operation, so one undo takes it back. The copy stands picked with its
// name field waiting, and the member it was copied from is not changed. The
// answers are the test's, so what the screen makes of the press is what is read.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
import { click, island } from './teams-page.js';

// definition is a team with a spec: the member a person copies when one spec is
// not enough — the reason the copy is for (n-4e16).
const definition = {
  members: [
    { name: 'lead', role: 'lead', profile: 'coder' },
    { name: 'spec', role: 'spec', profile: 'coder' },
  ],
  turns: [['human', 'lead'], ['lead', 'spec'], ['spec', 'lead']],
  workers: [{ from: 'spec', purpose: '調べ物', profile: 'coder' }],
};

test('a member is copied with its role, profile, turns and worker purposes, and one undo takes it back', async () => {
  history.replaceState({}, '', '/teams?open=core');
  layout('teams-data', island(definition));
  serve((method, url) => {
    if (url.startsWith('/api/team/check')) return { problems: [] };
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  const S = await import('../static/teams/state.js');
  await import('../static/teams/main.js');
  await settle();

  const row = () => document.querySelector('.tdef[data-name="core"]');
  const draft = () => S.byName('core').draft.value;
  const names = () => draft().members.map((m) => m.name);
  const copyButton = () => [...row().querySelectorAll('[data-cv-act=m-copy]')][0];

  // Pick the spec point: its panel is the member's own.
  click(row().querySelector('[data-cv-n="spec"]'));
  await settle();
  expect(copyButton()).not.toBe(undefined);
  expect(copyButton().textContent).toBe('複製');

  click(copyButton());
  await settle();

  // spec-2: the same role and profile, the turns both ways, and the worker's
  // purpose. The member it was copied from is as it was.
  const t = draft();
  expect(t.members.find((m) => m.name === 'spec')).toEqual({ name: 'spec', role: 'spec', profile: 'coder' });
  expect(t.members.find((m) => m.name === 'spec-2')).toEqual({ name: 'spec-2', role: 'spec', profile: 'coder' });
  expect(t.turns).toContainEqual(['lead', 'spec-2']);
  expect(t.turns).toContainEqual(['spec-2', 'lead']);
  expect(t.turns).toContainEqual(['lead', 'spec']);
  expect(t.turns).toContainEqual(['spec', 'lead']);
  expect(t.workers.filter((w) => w.from === 'spec-2')).toEqual([{ from: 'spec-2', purpose: '調べ物', profile: 'coder' }]);
  expect(t.workers.filter((w) => w.from === 'spec')).toEqual([{ from: 'spec', purpose: '調べ物', profile: 'coder' }]);

  // The copy stands picked, and its name field has the focus.
  expect(S.byName('core').sel.value).toEqual({ type: 'node', id: 'spec-2' });
  expect(document.activeElement).toBe(row().querySelector('[data-cv-f="m-name"]'));

  // A second press on the same member copies again, to the next free number.
  click(row().querySelector('[data-cv-n="spec"]'));
  await settle();
  click(copyButton());
  await settle();
  expect(names()).toContain('spec-3');

  // One undo takes one copy back, and one more takes the other: the whole copy
  // is one operation (r-c21b).
  S.canvasUndo(S.byName('core'));
  await settle();
  expect(names()).toContain('spec-2');
  expect(names()).not.toContain('spec-3');
  expect(draft().turns.some(([a, b]) => a === 'spec-3' || b === 'spec-3')).toBe(false);
  expect(draft().workers.some((w) => w.from === 'spec-3')).toBe(false);
  S.canvasUndo(S.byName('core'));
  await settle();
  expect(names()).toEqual(['lead', 'spec']);
  expect(draft().turns.filter(([a, b]) => a === 'spec-2' || b === 'spec-2')).toEqual([]);
  expect(draft().workers.map((w) => w.from)).toEqual(['spec']);
});
