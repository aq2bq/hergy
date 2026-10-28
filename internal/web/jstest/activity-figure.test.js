// ACTIVITY-FIGURE large teams keep every member and worker label readable at
// the tile size and in the opened team's larger diagram (r-8b04, ac-ecb0).
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
  { name: 'lead', state: 'active' },
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
  { name: 'rules-audit', from: 'qa', state: 'active' },
];

function expectLabelsDoNotOverlap(svg) {
  const labels = [...svg.querySelectorAll('text[data-label-width]')];
  expect(labels).toHaveLength(members.length + workers.length);
  const byRow = new Map();
  for (const label of labels) {
    const row = label.getAttribute('data-label-row');
    const cells = byRow.get(row) || [];
    cells.push({ x: Number(label.getAttribute('x')), width: Number(label.getAttribute('data-label-width')) });
    byRow.set(row, cells);
  }
  for (const cells of byRow.values()) {
    cells.sort((a, b) => a.x - b.x);
    for (let i = 1; i < cells.length; i++) {
      expect(cells[i].x - cells[i - 1].x).toBeGreaterThanOrEqual((cells[i].width + cells[i - 1].width) / 2);
    }
  }
}

test('four members and seven differently named workers fit both activity diagrams', async () => {
  history.replaceState({}, '', '/activity');
  layout();
  serve((method, url) => {
    if (url === '/api/activity') return {
      items: [{ name: 'large', project: '/work/alpha', project_name: 'alpha', summary: { definition: 'large', state: 'working' } }],
      herdr: { ok: true, text: '' }, running: 1, waiting: 1,
    };
    if (url === '/api/activity/large') return {
      doc: {
        nodes: members.map((member) => ({ ...member, herdr: `large-${member.name}`, at: '' })),
        hired: workers, tail: { lines: [] }, herdr: '',
      },
    };
    return undefined;
  });
  await import('../src/activity/main');
  await settle();

  const tile = document.querySelector('.hq-tile[data-team="large"]');
  const detail = document.querySelector('.hq-detail');
  expect(tile).not.toBe(null);
  expect(detail).not.toBe(null);
  expect(tile.querySelector('.hq-tc').textContent).toContain('メンバー 4人');
  expect(tile.querySelector('.hq-tc').textContent).toContain('ワーカー 7人');

  const compact = tile.querySelector('svg.hq-svg.packed');
  const large = detail.querySelector('.hq-dleft svg.hq-svg.packed');
  expect(compact.getAttribute('viewBox')).toMatch(/^0 0 440 /);
  expect(large.getAttribute('viewBox')).toMatch(/^0 0 720 /);
  expectLabelsDoNotOverlap(compact);
  expectLabelsDoNotOverlap(large);
  expect(large.querySelectorAll('.hq-e.wk')).toHaveLength(workers.length);
  expect([...large.querySelectorAll('.wkr')].map((el) => el.getAttribute('data-owner'))).toEqual(workers.map((worker) => worker.from));
});
