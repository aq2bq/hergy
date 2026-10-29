// ACTIVITY-FIGURE-OWNERLESS the workers whose owner the reading cannot name keep
// the last column of the packed figure, headlined 持ち主不明 instead of carrying a
// member's point and a line (r-f72e, d-54bb).
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
  { name: 'qa', state: 'idle' },
];
const workers = [
  { name: 'impl-a', from: 'lead', state: 'active' },
  { name: 'stray-review', from: 'gone-member', state: 'active' },
  { name: 'stray-probe', from: '', state: 'blocked' },
];

test('AC-7f11 the ownerless workers stand in the last column under a 持ち主不明 heading, with no point and no line', async () => {
  history.replaceState({}, '', '/activity');
  layout();
  serve((method, url) => {
    if (url === '/api/activity') return {
      items: [
        { name: 'stray', project: '/work/alpha', project_name: 'alpha', summary: { definition: 'stray', state: 'working' } },
      ],
      herdr: { ok: true, text: '' }, running: 1, waiting: 0,
    };
    if (url === '/api/activity/stray') return {
      doc: {
        nodes: members.map((member) => ({ ...member, herdr: `stray-${member.name}`, at: '' })),
        hired: workers,
        question: '',
      },
    };
    return undefined;
  });
  await import('../src/activity/main');
  await settle();

  const figure = document.querySelector('.hq-tile[data-team="stray"] svg.hq-svg.packed');
  expect(figure).not.toBe(null);

  // The ownerless group is the last column, and the only one without a member.
  const columns = [...figure.querySelectorAll('.hq-column')];
  expect(columns.map((column) => column.getAttribute('data-member'))).toEqual(['lead', 'qa', '']);
  const last = columns[columns.length - 1];
  const owned = columns.slice(0, -1);

  // Its heading says 持ち主不明, where a member's name stands, and the group has
  // no point of its own.
  const heading = last.querySelector('.hq-n.ownerless');
  expect(heading).not.toBe(null);
  expect(heading.querySelector('text').textContent).toBe('持ち主不明');
  expect(heading.querySelector('circle')).toBe(null);
  const memberLabel = owned[0].querySelector('.hq-n:not(.wkr) text.lbl');
  expect(Number(heading.querySelector('text').getAttribute('y'))).toBe(Number(memberLabel.getAttribute('y')));

  // The heading stays inside the column it heads.
  const left = Number(last.getAttribute('data-x-left'));
  const right = Number(last.getAttribute('data-x-right'));
  const label = heading.querySelector('text');
  expect(Number(label.getAttribute('x'))).toBeGreaterThanOrEqual(left);
  expect(Number(label.getAttribute('x')) + Number(label.getAttribute('data-label-width'))).toBeLessThanOrEqual(right);

  // Nothing is drawn but the workers: no stem, no branch, no line.
  expect(last.querySelectorAll('.hq-e')).toHaveLength(0);
  expect(figure.querySelectorAll('.wk-stem')).toHaveLength(1);
  expect(figure.querySelectorAll('.wk-branch')).toHaveLength(1);

  // The stray workers stand in the heading's column, unowned and in order.
  const strayNodes = [...last.querySelectorAll('.wkr')];
  expect(strayNodes.map((node) => node.querySelector('text').textContent)).toEqual(['stray-review', 'stray-probe']);
  for (const node of strayNodes) {
    expect(node.getAttribute('data-owner')).toBe('');
    expect(Number(node.querySelector('.dot').getAttribute('cx'))).toBeGreaterThanOrEqual(left);
    expect(Number(node.querySelector('.dot').getAttribute('cx'))).toBeLessThanOrEqual(right);
  }
});
