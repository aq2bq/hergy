// Empty herdr workspace names leave no dangling separator or empty row on the
// activity screen, stop sheet, or the headquarters' 動き column.
import { expect, test } from 'bun:test';
import { h, render } from '../src/lib/preact.js';
import { serve, settle } from './page.js';

function activityPage() {
  document.body.innerHTML = '<div class="wrap"><div class="bar"></div><div id="tabs"></div><div id="list"></div></div>';
  const words = document.createElement('script');
  words.type = 'application/json';
  words.id = 'activity-words';
  words.textContent = '{}';
  document.body.appendChild(words);
}

const teams = [
  {
    name: 'empty', project: '/work/alpha', project_name: '注文 API',
    workspace: { id: '', label: '' }, summary: { definition: 'empty', state: 'working' },
  },
  {
    name: 'named', project: '/work/alpha', project_name: '注文 API',
    workspace: { id: 'w1', label: '本体' }, summary: { definition: 'named', state: 'working' },
  },
];

const running = {
  herdr: { ok: true, text: '' },
  teams: teams.map((team) => ({
    team: team.name, definition: team.name, place: 'project', project: team.project,
    project_name: team.project_name, workspace: team.workspace,
    members: [], workers: [], waiting: [], starting: false,
  })),
};

test('an empty workspace has no trailing separator or stop-sheet row, and a named workspace is shown', async () => {
  history.replaceState({}, '', '/activity');
  activityPage();
  serve((method, url) => {
    if (url === '/api/activity') return { items: teams, herdr: { ok: true }, running: 2, waiting: 0 };
    if (url === '/api/activity/empty' || url === '/api/activity/named') return { doc: { nodes: [], hired: [], question: '' } };
    if (url === '/api/danger/activity/empty') return { lines: ['メンバー 0 人のペインを閉じます。', '会話は残りません。'] };
    if (url === '/api/danger/activity/named') return { lines: ['メンバー 0 人のペインを閉じます。', 'ワークスペース w1 は閉じません。', '会話は残りません。'] };
    if (url === '/api/hq/running') return running;
    return undefined;
  });
  await import('../src/activity/main');
  await settle();

  const emptyHeading = document.querySelector('.hq-tile[data-team="empty"] .hq-pw');
  expect(emptyHeading.textContent).toBe('注文 API');
  expect(emptyHeading.textContent).not.toMatch(/·\s*$/);
  const namedHeading = document.querySelector('.hq-tile[data-team="named"] .hq-pw');
  expect(namedHeading.textContent).toBe('注文 API · 本体');

  const { ConfirmDialog } = await import('../src/lib/confirm.js');
  const { render, h } = await import('../src/lib/preact.js');
  const confirmHost = document.createElement('div');
  document.body.appendChild(confirmHost);
  render(h(ConfirmDialog, {}), confirmHost);
  document.querySelector('.hq-tile[data-team="empty"] [data-hq="stop"]').click();
  await settle();
  let ask = document.querySelector('.confirm');
  expect(ask.textContent).not.toContain('ワークスペース');

  ask.querySelector('.confirm footer .btn').click();
  await settle();
  document.querySelector('.hq-tile[data-team="named"] [data-hq="stop"]').click();
  await settle();
  ask = document.querySelector('.confirm');
  expect(ask.textContent).toContain('ワークスペース w1 は閉じません。');
});

test('the headquarters movement field has no dangling separator when the workspace is unnamed', async () => {
  document.body.innerHTML = '<div id="mount"></div>';
  const A = await import('../src/lib/hq.js');
  const { RunCell } = await import('../src/lib/hq/run.js');
  const mount = document.getElementById('mount');
  const answer = (workspace) => ({
    herdr: { ok: true, text: '' },
    teams: [{
      team: 'core', definition: 'core', place: 'global', project: '/work/alpha', project_name: 'alpha',
      workspace, members: [], workers: [], waiting: [], starting: false,
    }],
  });

  A.setRunning(answer({ id: '', label: '' }));
  render(h(RunCell, { definition: 'core', place: 'global' }), mount);
  const emptyWhere = mount.querySelector('.hq-wsn');
  expect(emptyWhere.textContent).toBe('alpha');
  expect(emptyWhere.textContent).not.toMatch(/·\s*$/);

  A.setRunning(answer({ id: 'w1', label: '本体' }));
  render(h(RunCell, { definition: 'core', place: 'global' }), mount);
  expect(mount.querySelector('.hq-wsn').textContent).toBe('alpha · 本体');
});
