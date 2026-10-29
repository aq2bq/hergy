// r-1db7 L4 and L5: a team being started says 起動の途中 · 許可待ち on its tile,
// the way the 動き column does, and a waiting member whose question the screen
// cannot read says so rather than leaving the field empty.
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

test('a starting team and an unread question say what they are', async () => {
  history.replaceState({}, '', '/activity');
  layout();
  serve((method, url) => {
    if (url === '/api/activity') {
      return {
        items: [
          { name: 'migrate', project: '/work/alpha', project_name: 'alpha', workspace: { id: 'w1', label: '本体' }, summary: { definition: 'migrate', state: 'answer' } },
          { name: 'ledger', project: '/work/alpha', project_name: 'alpha', workspace: { id: 'w1', label: '本体' }, summary: { definition: 'ledger', state: 'answer' } },
        ],
        herdr: { ok: true }, running: 2, waiting: 2,
      };
    }
    if (url === '/api/activity/migrate') {
      return { doc: { nodes: [{ name: 'builder', herdr: 'migrate-builder', state: 'blocked', at: '' }], hired: [], question: 'Trust this folder?' } };
    }
    if (url === '/api/activity/ledger') {
      // The server says this when it cannot read the question (r-1db7 L5); the
      // screen draws the words it is given and writes none of its own.
      return { doc: { nodes: [{ name: 'builder', herdr: 'ledger-builder', state: 'blocked', at: '' }], hired: [], question: '聞いていることは、画面から読めませんでした' } };
    }
    if (url === '/api/hq/running') {
      return {
        herdr: { ok: true, text: '' },
        teams: [{
          team: 'migrate', definition: 'migrate', place: 'project', project: '/work/alpha', project_name: 'alpha',
          workspace: { id: 'w1', label: '本体' }, tab: { id: 't1', label: 'migrate' },
          members: [{ name: 'builder', herdr: 'migrate-builder', live: true, state: 'blocked' }],
          workers: [], waiting: ['builder'], starting: true,
        }, {
          team: 'ledger', definition: 'ledger', place: 'project', project: '/work/alpha', project_name: 'alpha',
          workspace: { id: 'w1', label: '本体' }, tab: { id: 't1', label: 'ledger' },
          members: [{ name: 'builder', herdr: 'ledger-builder', live: true, state: 'blocked' }],
          workers: [], waiting: ['builder'], starting: false,
        }],
      };
    }
    return undefined;
  });
  await import('../src/activity/main');
  await settle();

  expect(document.querySelector('.hq-tile[data-team="migrate"] .st').textContent).toBe('起動の途中 · 許可待ち');
  expect(document.querySelector('.hq-tile[data-team="ledger"] .st').textContent).toBe('許可待ち');
  expect(document.querySelector('.hq-at[data-pick="migrate"] .hq-aq').textContent).toBe('Trust this folder?');
  expect(document.querySelector('.hq-at[data-pick="ledger"] .hq-aq').textContent).toBe('聞いていることは、画面から読めませんでした');
});
