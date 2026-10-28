// ACTIVITY-C1 who the screen says is waiting: from the activity reading, the
// first tier names each member a person owes an answer to, the herdr name to
// answer at, and the team's own figure carries the same state (spec 6 章). The
// answer is the test's, so the screen's reading of it is what is read. The
// ten-second re-reading and where it leaves the person are the browser's
// (ACTIVITY-P4).
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';

// layout writes the activity page's body: the board's list and hergy's words.
function layout() {
  document.body.innerHTML = '<div class="wrap"><div class="bar"></div><div id="tabs"></div><div id="list"></div></div>';
  const words = document.createElement('script');
  words.type = 'application/json';
  words.id = 'activity-words';
  words.textContent = '{}';
  document.body.appendChild(words);
}

// team is one item of the activity reading: a team, its state, and where it
// runs.
const team = (name, state) => ({
  name, project: '', project_name: '注文 API',
  workspace: { id: 'w1', label: '本体' },
  summary: { definition: name, state },
});
// sheet is one team's own reading: the members herdr knows, with their state.
const sheet = (nodes, question = '') => ({ doc: { nodes, hired: [], question } });

test('the first tier and the figure say whose answer a person owes', async () => {
  history.replaceState({}, '', '/activity');
  layout();
  serve((method, url) => {
    if (url === '/api/activity') {
      return { items: [team('core', 'turn'), team('migrate', 'answer'), team('stalled', 'stalled')], herdr: { ok: true }, running: 3, waiting: 2 };
    }
    if (url === '/api/activity/core') return sheet([{ name: 'lead', herdr: 'core-lead', state: 'handed', at: '' }], 'Continue with the change?');
    if (url === '/api/activity/migrate') return sheet([{ name: 'tester', herdr: 'migrate-tester', state: 'blocked', at: '' }], 'Trust this folder?');
    if (url === '/api/activity/stalled') return sheet([{ name: 'lead', herdr: 'stalled-lead', state: 'idle', at: '' }]);
    return undefined;
  });
  await import('../src/activity/main');
  await settle();

  const waiting = (name) => {
    const row = document.querySelector(`.hq-at[data-pick="${name}"]`);
    expect(row, `no waiting row for ${name}`).not.toBe(null);
    return [row.querySelector('.st').textContent, row.querySelector('.hq-aw code').textContent];
  };
  // core's lead handed the turn over; migrate's tester waits for a permission.
  expect(waiting('core')).toEqual(['人間の番', 'core-lead']);
  expect(waiting('migrate')).toEqual(['許可待ち', 'migrate-tester']);
  expect(document.querySelector('.hq-at[data-pick="core"] .hq-aq').textContent).toBe('Continue with the change?');
  expect(document.querySelector('.hq-at[data-pick="migrate"] .hq-aq').textContent).toBe('Trust this folder?');

  // The team's own figure says the same state.
  const tile = document.querySelector('.hq-tile[data-team="core"] .st');
  expect(tile).not.toBe(null);
  expect(tile.textContent).toBe('人間の番');

  const scrolled = [];
  Object.defineProperty(Element.prototype, 'scrollIntoView', { configurable: true, value(options) { scrolled.push([this.dataset.team, options.block]); } });
  for (const name of ['migrate', 'stalled']) {
    document.querySelector(`.hq-at[data-pick="${name}"]`).click();
    await settle();
    expect(document.querySelector('.hq-tile.sel').dataset.team).toBe(name);
    expect(location.search).toContain(`open=${name}`);
  }
  expect(scrolled).toEqual([['migrate', 'nearest'], ['stalled', 'nearest']]);
});
