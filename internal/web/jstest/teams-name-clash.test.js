// TEAMS-N2, a member's name typed while another member holds it (r-4fd7, n-4b24,
// ac-5edb): the figure's name field does not put a name another member holds into
// the draft, so the member being typed cannot take that member's name — and with
// it its turns and workers — while the letters are typed. The letters stay in the
// field and the words beside it say why; once the name is free, only the member
// being typed changes. The answers are the test's, so what the screen makes of the
// typing is what is read.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
import { click, island } from './teams-page.js';

// definition is a team with a spec: the member whose name a person would type
// into the new member's field, one letter at a time, to make a spec-2.
const definition = {
  members: [
    { name: 'lead', role: 'lead', profile: 'coder' },
    { name: 'spec', role: 'spec', profile: 'coder' },
  ],
  turns: [['human', 'lead'], ['lead', 'spec'], ['spec', 'lead']],
  workers: [{ from: 'spec', purpose: '調べ物', profile: 'coder' }],
};

test('typing a name another member holds leaves that member, its turns and its workers as they are', async () => {
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
  // ＋メンバー adds one member and picks it; its name field is where a person
  // types the new name.
  click(row().querySelector('[data-cv-act=m-add]'));
  await settle();
  const field = () => row().querySelector('[data-cv-f="m-name"]');
  expect(field()).not.toBe(null);

  // type writes the field as a person does, one letter at a time: each key is one
  // input event with the whole text the field now holds.
  const type = async (text) => {
    const el = field();
    el.value = text;
    el.dispatchEvent(new Event('input', { bubbles: true }));
    await settle();
  };
  const members = () => S.byName('core').draft.value.members.map((m) => m.name);
  const msgs = () => [...row().querySelectorAll('.cv-msg')].map((x) => x.textContent).join('\n');

  // s, sp, spe: the name is free, so the member being typed is the one renamed.
  await type('s');
  await type('sp');
  await type('spe');
  // spec: another member holds it. The word stands beside the field, the field
  // keeps the letters, and the draft holds one spec — the member that was there.
  await type('spec');
  expect(msgs()).toContain('このチームにもういます');
  expect(field().value).toBe('spec');
  expect(members().filter((n) => n === 'spec').length).toBe(1);

  // spec-, spec-2: the name is free again, and only the member being typed
  // changes.
  await type('spec-');
  await type('spec-2');
  const t = S.byName('core').draft.value;
  expect(t.members.map((m) => m.name)).toContain('spec-2');
  expect(t.members.filter((m) => m.name === 'spec').length).toBe(1);
  // The spec that was there keeps its role and profile.
  const spec = t.members.find((m) => m.name === 'spec');
  expect(spec.role).toBe('spec');
  expect(spec.profile).toBe('coder');
  // Its turns and workers are untouched, and the new member has none yet.
  expect(t.turns).toContainEqual(['lead', 'spec']);
  expect(t.turns).toContainEqual(['spec', 'lead']);
  expect(t.turns.some(([a, b]) => a === 'spec-2' || b === 'spec-2')).toBe(false);
  expect(t.workers.map((w) => w.from)).toContain('spec');
  expect(t.workers.some((w) => w.from === 'spec-2')).toBe(false);
  // No member was left behind under a name the person typed on the way.
  expect(t.members.map((m) => m.name).sort()).toEqual(['lead', 'spec', 'spec-2']);
});
