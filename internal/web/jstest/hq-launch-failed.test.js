// SHOWCASE-A1 follow-up, a start that stopped partway (r-cf68, d-5f02): the
// start sheet says which member herdr refused, why in herdr's own words, what
// started and what is left, and offers the way to put the remains away —
// 「チームを終了する…」 opens the stop sheet for the same team. The answers are
// the test's, so what the sheet makes of them is what is read.
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';

function layout() {
  document.body.innerHTML = '<div class="wrap"><div class="bar"></div><div id="tabs"></div><div id="view"></div></div>';
}

// failed is the answer the launch gives: two members started, the third was
// refused, and the team's tab and panes are still there.
const failed = {
  id: 'l-9', action: 'launch', team: 'draft', definition: 'draft', place: 'global',
  project: '/work/atlas', project_name: 'atlas',
  workspace: { id: 'w1', label: '本体' }, tab: { id: 't1', label: 'draft' },
  phase: 'failed', waiting: null, members: [],
  error: 'draft-writer の起動が失敗しました: the profile coder is not installed。'
    + '起動できたメンバー: draft-lead、draft-builder。残っているもの: タブ draft、ペイン 3 個。'
    + 'このチームは動いている扱いです。'
    + '直すには、atlas で hergy down draft のあと hergy up draft を実行してください。',
  steps: [{ at: '', kind: 'done', text: 'タブ draft を作りました' }],
};

const running = () => ({
  herdr: { ok: true, text: '' },
  teams: [{
    team: 'draft', definition: 'draft', place: 'global', project: '/work/atlas', project_name: 'atlas',
    workspace: { id: 'w1', label: '本体' }, tab: { id: 't1', label: 'draft' },
    members: [
      { name: 'lead', herdr: 'draft-lead', live: true, state: 'idle' },
      { name: 'builder', herdr: 'draft-builder', live: true, state: 'idle' },
      { name: 'writer', herdr: '', live: false, state: 'absent' },
    ],
    workers: [], waiting: [], starting: false,
  }],
});

test('a start that failed says who, why and what is left, and opens the stop sheet', async () => {
  history.replaceState({}, '', '/activity');
  layout();
  serve((method, url) => {
    if (url.startsWith('/api/hq/launches/')) return { launch: failed };
    if (url === '/api/hq/running') return running();
    return undefined;
  });
  const { runStart } = await import('../src/lib/hq/sheets');
  await settle();

  runStart(failed);
  await settle();
  const sheet = document.querySelector('.hq-sheet');
  expect(sheet).not.toBe(null);
  expect(sheet.querySelector('[data-role=title]').textContent).toBe('起動できませんでした');
  const error = sheet.querySelector('[data-role=launch-error]').textContent;
  expect(error).toContain('draft-writer の起動が失敗しました');
  expect(error).toContain('the profile coder is not installed');
  expect(error).toContain('残っているもの: タブ draft、ペイン 3 個');
  expect(error).not.toContain('{"error"');

  // The way to put the remains away is the stop sheet for the same team.
  const stop = sheet.querySelector('[data-hq=stop]');
  expect(stop.textContent).toBe('チームを終了する…');
  stop.click();
  await settle();
  const second = document.querySelector('.hq-sheet');
  expect(second.querySelector('.hq-h b').textContent).toBe('終了する');
  expect(second.querySelector('.hq-h code').textContent).toBe('draft');
  expect(second.querySelector('[data-hq=stopgo]')).not.toBe(null);
});
