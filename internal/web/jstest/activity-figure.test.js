// ACTIVITY-FIGURE large teams keep every member and worker label readable in
// the tile, and the tile carries its actions without a third detail section.
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';

function layout() {
  document.body.innerHTML = '<div class="wrap"><div class="bar"><nav class="screens" data-key="k"><span class="scr on" data-screen="activity"><span class="n">0</span></span></nav></div><div id="tabs"></div><div id="list"></div></div>';
  const words = document.createElement('script');
  words.type = 'application/json';
  words.id = 'activity-words';
  words.textContent = '{}';
  document.body.appendChild(words);
}

const members = [
  { name: 'lead', state: 'active', doing: '設計を読む' },
  { name: 'webui-design', state: 'idle' },
  { name: 'long-reviewer', state: 'blocked' },
  { name: 'qa', state: 'absent' },
];
const workers = [
  { name: 'impl-b', from: 'lead', state: 'active' },
  { name: 'impl-a', from: 'lead', state: 'active' },
  { name: 'inv-b', from: 'webui-design', state: 'active' },
  { name: 'template-image', from: 'webui-design', state: 'idle' },
  { name: 'html-mail', from: 'long-reviewer', state: 'active' },
  { name: 'listing', from: 'long-reviewer', state: 'active' },
  { name: 'rules-audit', from: 'lead', state: 'active' },
  { name: 'api-contracts', from: 'lead', state: 'idle' },
  { name: 'prompt-review', from: 'webui-design', state: 'active' },
  { name: 'visual-regression', from: 'lead', state: 'active' },
];
const turns = [['human', 'lead'], ['lead', 'webui-design'], ['webui-design', 'long-reviewer'], ['qa', 'lead']];

function expectLabelsDoNotOverlap(svg) {
  const labels = [...svg.querySelectorAll('text[data-label-width]')];
  expect(labels).toHaveLength(members.length + workers.length);
  const byRow = new Map();
  for (const label of labels) {
    const row = label.getAttribute('data-label-row');
    const cells = byRow.get(row) || [];
    const x = Number(label.getAttribute('x'));
    const width = Number(label.getAttribute('data-label-width'));
    const start = label.classList.contains('left') ? x : x - width / 2;
    cells.push({ start, end: start + width });
    byRow.set(row, cells);
  }
  for (const cells of byRow.values()) {
    cells.sort((a, b) => a.start - b.start);
    for (let i = 1; i < cells.length; i++) expect(cells[i].start).toBeGreaterThanOrEqual(cells[i - 1].end);
  }
}

test('AC-6e5b four member columns hold ten workers and keep turns above the members', async () => {
  history.replaceState({}, '', '/activity');
  layout();
  serve((method, url) => {
    if (url === '/api/activity') return {
      items: [
        { name: 'large', project: '/work/alpha', project_name: 'alpha', summary: { definition: 'large', state: 'working' } },
        { name: 'idle', project: '/work/alpha', project_name: 'alpha', summary: { definition: 'idle', state: 'working' } },
      ],
      herdr: { ok: true, text: '' }, running: 1, waiting: 1,
    };
    if (url === '/api/activity/large') return {
      doc: {
        nodes: members.map((member) => ({ ...member, herdr: `large-${member.name}`, at: '' })),
        hired: workers,
        question: '',
      },
    };
    if (url === '/api/activity/idle') return { doc: { nodes: [], hired: [], question: '' } };
    if (url === '/api/hq/teams') return {
      groups: [{
        project: { path: '/work/alpha', name: 'alpha' },
        list: { items: [{ name: 'large', effective: 'project', places: { project: { doc: {
          members: members.map((member) => ({ name: member.name })), workers: [],
          turns: turns.map(([from, to]) => ({ from, to })),
        } } } }] },
      }],
    };
    if (url === '/api/apply') return { herdr: { ok: true }, count: 1, teams: [{ team: 'large', members: [{ name: 'large-lead' }] }] };
    if (url === '/api/apply/count') return { count: 1 };
    return undefined;
  });
  await import('../src/activity/main');
  await import('../src/lib/apply');
  await settle();

  const tile = document.querySelector('.hq-tile[data-team="large"]');
  expect(tile).not.toBe(null);
  expect(document.querySelector('.hq-detail')).toBe(null);
  expect(document.querySelectorAll('.hq-tile')).toHaveLength(2);
  expect(tile.querySelector('.hq-tc').textContent).toContain('メンバー 4人');
  expect(tile.querySelector('.hq-tc').textContent).toContain('ワーカー 10人');
  expect(tile.querySelector('.hq-tactions a').textContent).toBe('チーム定義を開く');
  expect(tile.querySelector('.hq-tactions a').getAttribute('href')).toContain('/teams?open=');
  expect(tile.querySelector('[data-hq="apply"]').textContent).toContain('反映していない変更');
  expect(tile.querySelector('[data-hq="stop"]').textContent).toBe('チームを終了する…');
  const idleTile = document.querySelector('.hq-tile[data-team="idle"]');
  expect(idleTile.querySelector('.hq-tactions a').textContent).toBe('チーム定義を開く');
  expect(idleTile.querySelector('[data-hq="apply"]')).toBe(null);
  expect(idleTile.querySelector('[data-hq="stop"]').textContent).toBe('チームを終了する…');

  const figure = tile.querySelector('svg.hq-svg.packed');
  const [, width] = figure.getAttribute('viewBox').match(/^0 0 (\d+) /);
  expect(Number(width)).toBeGreaterThanOrEqual(440);
  expectLabelsDoNotOverlap(figure);
  expect(figure.querySelectorAll('.hq-e.wk-stem')).toHaveLength(members.filter((member) => workers.some((worker) => worker.from === member.name)).length);
  expect(figure.querySelectorAll('.hq-e.wk-branch')).toHaveLength(workers.length);
  const expectedWorkerOwners = members.flatMap((member) => workers.filter((worker) => worker.from === member.name).map((worker) => worker.from));
  expect([...figure.querySelectorAll('.wkr')].map((el) => el.getAttribute('data-owner'))).toEqual(expectedWorkerOwners);
  const columns = [...figure.querySelectorAll('.hq-column[data-member]:not([data-member=""])')];
  expect(columns.map((column) => column.getAttribute('data-member'))).toEqual(members.map((member) => member.name));
  for (const column of columns) {
    const owner = column.getAttribute('data-member');
    const left = Number(column.getAttribute('data-x-left'));
    const right = Number(column.getAttribute('data-x-right'));
    const ownedWorkers = workers.filter((worker) => worker.from === owner);
    const stem = column.querySelector('.hq-e.wk-stem');
    const branches = [...column.querySelectorAll('.hq-e.wk-branch')];
    expect(branches).toHaveLength(ownedWorkers.length);
    const memberDot = column.querySelector('.hq-n:not(.wkr) .dot');
    const ownedDots = [...column.querySelectorAll('.wkr .dot')];
    if (ownedWorkers.length) {
      expect(stem).not.toBe(null);
      const stemValues = stem.getAttribute('d').match(/-?\d+(?:\.\d+)?/g).map(Number);
      expect(stemValues).toEqual([
        Number(memberDot.getAttribute('cx')),
        Number(memberDot.getAttribute('cy')) + 11,
        Math.max(...ownedDots.map((dot) => Number(dot.getAttribute('cy')))),
      ]);
    } else {
      expect(stem).toBe(null);
    }
    expect(branches.map((branch) => branch.getAttribute('data-worker'))).toEqual(ownedWorkers.map((worker) => worker.name));
    for (const branch of branches) {
      const values = branch.getAttribute('d').match(/-?\d+(?:\.\d+)?/g).map(Number);
      expect(values[0]).toBe(Number(memberDot.getAttribute('cx')));
      expect(values[2]).toBeGreaterThan(values[0]);
      for (let i = 0; i < values.length; i += 2) {
        expect(values[i]).toBeGreaterThanOrEqual(left);
        expect(values[i]).toBeLessThanOrEqual(right);
      }
    }
    const ownedNodes = [...column.querySelectorAll('.wkr')];
    expect(ownedNodes.map((node) => node.querySelector('text').textContent)).toEqual(ownedWorkers.map((worker) => worker.name));
    for (const node of ownedNodes) {
      const dot = node.querySelector('.dot');
      const label = node.querySelector('text');
      expect(label.classList.contains('left')).toBe(true);
      expect(Number(dot.getAttribute('cx'))).toBeGreaterThanOrEqual(left);
      expect(Number(label.getAttribute('x')) + Number(label.getAttribute('data-label-width'))).toBeLessThanOrEqual(right);
    }
    const label = column.querySelector('.hq-n:not(.wkr) text.lbl');
    expect(label.classList.contains('left')).toBe(true);
    expect(Number(label.getAttribute('x')) + Number(label.getAttribute('data-label-width'))).toBeLessThanOrEqual(right);
    const connectors = [...column.querySelectorAll('.hq-e.wk')];
    for (const connector of connectors) {
      expect(connector.getAttribute('data-owner')).toBe(owner);
      const values = connector.getAttribute('d').match(/-?\d+(?:\.\d+)?/g).map(Number);
      const xValues = connector.classList.contains('wk-stem') ? [values[0]] : [values[0], values[2]];
      for (const x of xValues) {
        expect(x).toBeGreaterThanOrEqual(left);
        expect(x).toBeLessThanOrEqual(right);
      }
    }
  }
  const memberY = Number(columns[0].querySelector('.hq-n:not(.wkr) .dot').getAttribute('cy'));
  const memberTurns = [...figure.querySelectorAll('.hq-e.turn[data-control-y]')];
  expect(memberTurns).toHaveLength(turns.filter(([from, to]) => from !== 'human' && to !== 'human').length);
  for (const edge of memberTurns) expect(Number(edge.getAttribute('data-control-y'))).toBeLessThan(memberY - 15);
  expect(figure.querySelector('.hq-column[data-member="lead"] .hq-n.active title').textContent).toBe('lead · 仕事中 · 設計を読む');
  expect(figure.querySelector('.hq-column[data-member="webui-design"] .hq-n.idle title').textContent).toBe('webui-design · 止まっている');
});
