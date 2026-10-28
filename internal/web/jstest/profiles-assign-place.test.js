// PROFILES-P5, the assign tab saves in the place its card stands in (r-f8b9): a
// definition that is only in the global layer is saved in the global file, with
// that file's hash, and the card says グローバル so a person knows the change
// reaches every project. The answer is the test's, so the body the press sends is
// what is read.
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';

function layout() {
  document.body.innerHTML = '<div class="wrap"><div class="bar"></div><div id="tabs"></div>'
    + '<main id="view" data-tab="assign" data-token="k" data-hash-project="" data-hash-shared=""></main></div>';
  const data = document.createElement('script');
  data.type = 'application/json';
  data.id = 'profiles-data';
  data.textContent = JSON.stringify({
    assign: {
      empty: false,
      kinds: [],
      teams: [{
        name: 'shared',
        place: 'global',
        path: '~/.config/hergy/teams/shared/team.yaml',
        hash: 'hg1',
        members: 1,
        workers: 0,
        err: 0,
        rows: [{
          id: 'm-0', kind: 'm', name: 'lead', role: 'lead', profile: 'coder', options: [{ kind: 'claude', names: ['coder', 'planner'] }],
        }],
      }],
    },
  });
  document.body.appendChild(data);
}

test('the assign card stands in the global place, says so, and saves there', async () => {
  history.replaceState({}, '', '/profiles/assign');
  layout();
  const asked = serve((method, url) => {
    if (url.startsWith('/api/team/')) return { hash: 'hg2' };
    return undefined;
  });
  await import('../static/profiles/main.js');
  await settle();

  const card = document.querySelector('.team[data-g="shared"]');
  expect(card).not.toBe(null);
  // The card names the place in the same word the team definition screen uses,
  // and says what a change to a global definition reaches.
  expect(card.querySelector('.pill.on').textContent).toBe('グローバル');
  const note = card.querySelector('[data-role=place-note]');
  expect(note).not.toBe(null);
  expect(note.textContent).toContain('すべてのプロジェクト');

  const select = card.querySelector('select');
  select.value = 'planner';
  select.dispatchEvent(new Event('change', { bubbles: true }));
  await settle();
  card.querySelector('[data-act=g-save]').dispatchEvent(new Event('click', { bubbles: true }));
  await settle();

  const saved = asked.filter((one) => one.method !== 'GET' && one.url.startsWith('/api/team/'));
  expect(saved.length).toBe(1);
  expect(saved[0].body.place).toBe('global');
  expect(saved[0].body.hash).toBe('hg1');
  expect(saved[0].body.doc.ops).toEqual([{ kind: 'memberProfile', member: 'lead', profile: 'planner' }]);
});
