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

  document.querySelector('.hq-tile[data-team="empty"] [data-hq="stop"]').click();
  await settle();
  let sheet = document.querySelector('.hq-sheet');
  expect([...sheet.querySelectorAll('.hq-kv dt')].map((row) => row.textContent)).toEqual(['プロジェクト', '閉じるペイン']);
  expect(sheet.querySelector('.hq-kv').textContent).not.toContain('ワークスペース');
  expect(sheet.querySelector('.hq-sub').textContent).not.toContain('ワークスペース');

  sheet.querySelector('[data-hqx="close"]').click();
  await settle();
  document.querySelector('.hq-tile[data-team="named"] [data-hq="stop"]').click();
  await settle();
  sheet = document.querySelector('.hq-sheet');
  const workspaceRow = [...sheet.querySelectorAll('.hq-kv dt')].find((row) => row.textContent === 'ワークスペース');
  expect(workspaceRow).not.toBe(undefined);
  expect(workspaceRow.nextElementSibling.textContent).toBe('本体');
  expect(sheet.querySelector('.hq-sub').textContent).toContain('ワークスペース 本体 は閉じません。');
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
