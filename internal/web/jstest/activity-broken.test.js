// ACTIVITY-R2, a team whose definition cannot be built says why (r-fc16): the
// activity reading carries the reason in hergy status's words, with the way to
// fix it, and the team's row shows it rather than leaving a person to guess why
// the figure has no member of its own.
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';

function layout() {
  document.body.innerHTML = '<div class="wrap"><div class="bar"></div><div id="tabs"></div><div id="list"></div></div>';
  const words = document.createElement('script');
  words.type = 'application/json';
  words.id = 'activity-words';
  words.textContent = '{}';
  document.body.appendChild(words);
}

const reason = '.hergy/teams/core/team.yaml: ロール名「進行管理」は使えません。ロール名は小文字で始めます。人に読ませる名前は title に書きます';

// team is one item of the activity reading: the definition it runs, the live
// agent it has, and why its definition cannot be built.
const team = {
  name: 'core', project: '/work/alpha', project_name: '注文 API',
  workspace: { id: 'w1', label: '本体' },
  summary: { definition: 'core', state: 'stopped' },
  error: reason,
};

test('the row of a team whose definition cannot be built says why', async () => {
  history.replaceState({}, '', '/activity');
  layout();
  serve((method, url) => {
    if (url === '/api/activity') {
      return { items: [team], herdr: { ok: true }, running: 1, waiting: 0 };
    }
    if (url === '/api/activity/core') {
      return { doc: { nodes: [{ name: 'lead', herdr: 'core-lead', state: 'idle', at: '' }], hired: [], question: '' } };
    }
    return undefined;
  });
  await import('../src/activity/main');
  await settle();

  const tile = document.querySelector('.hq-tile[data-team="core"]');
  expect(tile).not.toBe(null);
  const broken = tile.querySelector('[data-role=definition-error]');
  expect(broken).not.toBe(null);
  expect(broken.textContent).toBe(reason);
  // The live agent is still named: the reason stands beside the team it is about.
  expect(tile.textContent).toContain('lead');
});
