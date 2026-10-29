// r-b65e: the stop sheet's result says what the team left in git — the
// worktrees its panes stood in, the project's stashes, and the commands that
// remove them. hergy removes nothing.
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';

test('the stop result lists the worktrees and stashes the team left in git', async () => {
  document.body.innerHTML = '<div class="wrap"><div class="bar"></div><div id="tabs"></div><div id="view"></div></div>';
  const running = {
    herdr: { ok: true, text: '' },
    teams: [{
      team: 'draft', definition: 'draft', place: 'global', project: '/work/atlas', project_name: 'atlas',
      workspace: { id: 'w1', label: '本体' }, tab: { id: 't1', label: 'draft' },
      members: [{ name: 'lead', herdr: 'draft-lead', live: true, state: 'idle' }],
      workers: [{ name: 'bench', tab: 'worker:draft/bench', live: true }],
      waiting: [], starting: false,
    }],
  };
  const stopped = {
    team: 'draft', definition: 'draft', project: '/work/atlas',
    workspace: { id: 'w1', label: '本体' },
    closed: { members: 1, workers: 1 }, stopped_at: '2026-10-29T00:00:00Z',
    leftovers: {
      worktrees: [{ path: '/work/atlas/.worktrees/lane-1', branch: 'lane-1', dirty: true, ahead: 1, base: 'main' }],
      stashes: 1,
    },
  };
  serve((method, url) => {
    if (url === '/api/hq/running') return running;
    if (url === '/api/hq/stop') return stopped;
    return undefined;
  });
  const hq = await import('../src/lib/hq');
  await hq.readCore();
  await settle();
  const { stopSheet } = await import('../src/lib/hq/sheets.js');
  stopSheet('draft');
  await settle();

  document.querySelector('[data-hq=stopgo]').click();
  await settle();
  const sheet = document.querySelector('.hq-sheet');
  expect(sheet.querySelector('.hq-h b').textContent).toBe('終了しました');
  const left = sheet.querySelector('[data-role=leftovers]');
  expect(left).not.toBe(null);
  for (const want of [
    '/work/atlas/.worktrees/lane-1',
    '枝 lane-1',
    '保存していない変更あり',
    'main に入っていないコミット 1 件',
    '消すには: git worktree remove /work/atlas/.worktrees/lane-1',
    '保存していない変更か、入っていないコミットがあります。消す前に中身を確かめてください',
    'このプロジェクトの stash: 1 件',
    'hergy は消しません',
  ]) {
    expect(left.textContent).toContain(want);
  }
});
