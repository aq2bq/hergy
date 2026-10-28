// ROLES-P6, the assign tab saves in the place its card stands in (r-f8b9): a
// definition that is only in the global layer is saved in the global file, with
// that file's hash, and the card says グローバル so a person knows the change
// reaches every project. The answer is the test's, so the body the press sends is
// what is read.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';

const doc = (name, title) => ({
  name, title, revision: 1,
  sections: [{ heading: '進め方', level: 0, intro: '', rules: [{ id: 'R-1', text: '書く。', note: '', in_prompt: true }] }],
});

const island = {
  places: { project: '.hergy/roles/', global: '~/.config/hergy/roles/' },
  texts: {},
  role: {
    items: [
      { name: 'lead', places: { global: { doc: doc('lead', '進行'), hash: 'hr1', text: '' } }, summary: {} },
      { name: 'builder', places: { global: { doc: doc('builder', '実装'), hash: 'hr2', text: '' } }, summary: {} },
    ],
  },
  team: {
    items: [{
      name: 'shared',
      effective: 'global',
      places: {
        global: {
          path: '~/.config/hergy/teams/shared/team.yaml',
          hash: 'hg1',
          doc: { mission: '共通のチーム', turns: [['human', 'lead'], ['lead', 'human']] },
        },
      },
      summary: { members: [{ name: 'lead', role: 'lead' }] },
    }],
  },
};

test('the assign card stands in the global place, says so, and saves there', async () => {
  history.replaceState({}, '', '/roles/assign');
  layout('roles-data', island);
  const asked = serve((method, url) => {
    if (url.startsWith('/api/role')) return { items: [] };
    if (url.startsWith('/api/team/')) return { hash: 'hg2' };
    return undefined;
  });
  await import('../src/roles/main');
  await settle();

  const card = document.querySelector('.team[data-t="shared"]');
  expect(card).not.toBe(null);
  // The card names the place in the same word the team definition screen uses,
  // and says what a change to a global definition reaches.
  expect(card.querySelector('.pill.on').textContent).toBe('グローバル');
  const note = card.querySelector('[data-role=place-note]');
  expect(note).not.toBe(null);
  expect(note.textContent).toContain('すべてのプロジェクト');

  const select = card.querySelector('select');
  select.value = 'builder';
  select.dispatchEvent(new Event('change', { bubbles: true }));
  await settle();
  card.querySelector('[data-act=t-save]').dispatchEvent(new Event('click', { bubbles: true }));
  await settle();

  const saved = asked.filter((one) => one.method !== 'GET' && one.url.startsWith('/api/team/'));
  expect(saved.length).toBe(1);
  expect(saved[0].body.place).toBe('global');
  expect(saved[0].body.hash).toBe('hg1');
  expect(saved[0].body.doc.ops).toEqual([{ kind: 'memberRole', member: 'lead', role: 'builder' }]);
});
