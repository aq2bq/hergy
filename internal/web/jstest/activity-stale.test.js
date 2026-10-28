// SHOWCASE-D1, the mark on the activity screen: a member whose definition
// changed carries the mark that opens 変更を伝える (r-a8fa). The activity answer
// is the test's; the mark is the screen's own.
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

test('a stale member carries the mark, and a current one does not', async () => {
  history.replaceState({}, '', '/activity');
  layout();
  serve((method, url) => {
    if (url === '/api/activity') {
      return {
        items: [{
          name: 'core', project: '', project_name: '注文 API',
          workspace: { id: 'w1', label: '本体' },
          summary: { definition: 'core', state: 'working', last: '' },
        }],
        herdr: { ok: true }, running: 1, waiting: 0,
      };
    }
    if (url === '/api/activity/core') {
      return { doc: { nodes: [
        { name: 'lead', herdr: 'core-lead', state: 'active', at: '', stale: false },
        { name: 'impl-a', herdr: 'core-impl-a', state: 'handed', at: '', stale: true },
        { name: 'impl-b', herdr: 'core-impl-b', state: 'idle', at: '', stale: true },
        { name: 'reviewer', herdr: 'core-reviewer', state: 'blocked', at: '', stale: false },
      ], hired: [], tail: { lines: [] }, herdr: '' } };
    }
    return undefined;
  });
  await import('../src/activity/main');
  await settle();

  const marked = [...document.querySelectorAll('#list .hq-detail .nodes li')]
    .filter((li) => li.querySelector('.dlv-badge'))
    .map((li) => li.querySelector('.nm').textContent);
  expect(marked).toEqual(['impl-a', 'impl-b']);
  expect(marked.length).toBe(2);
  // The mark is the press 変更を伝える answers: the screen's own press opens the
  // part's sheet (activity/board.js, lib/deliver/state.js).
  expect(document.querySelectorAll('#list .hq-detail .dlv-badge[data-deliver-open]').length).toBe(2);
  await import('../src/lib/deliver');
  await settle();
  document.querySelector('#list .hq-detail .dlv-badge').dispatchEvent(new Event('click', { bubbles: true }));
  await settle();
  expect(document.querySelector('.dlv-sheet')).not.toBe(null);
});
