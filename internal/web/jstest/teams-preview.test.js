// TEAMS-Y2, the topology tab's preview: the box draws the check's own lines (the
// saved team.yaml while nothing changed), a turn added from a member's card
// stands as an added line, and the box is sent to that line (B7, Y2, R2). That
// the preview's lines are the saved file's line for line is the server's
// (internal/web's TestTEAMS_Y6_ThePreviewIsTheSavedFile); here the answer is the
// test's, so what the box makes of it is what is read.
//
// The browser is what lays the box and its lines out; this test says where they
// stand, so the choice of line and the scroll that follows are held without a
// browser (the walk read the same two rectangles).
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
import { choose, click, island } from './teams-page.js';

const file = [
  'version: 1',
  'mission: 注文 API を作り直す',
  '# a comment a person wrote',
  'members:',
  '  - name: lead',
  '    role: lead',
  '    profile: coder',
  '  - name: analyst',
  '    role: analyst',
  '    profile: coder',
  '  - name: reviewer',
  '    role: reviewer',
  '    profile: coder',
  'turns:',
  '  - human -> lead',
  '  - lead -> human',
  '  - analyst -> lead',
];

// linesOfFile is the preview's answer for the file as it stands: every line,
// and what it stands for (definition.YAMLLines: the mission, each member's own
// entry, each turn).
const linesOfFile = [];
let member = '';
for (const text of file) {
  const named = /^\s+- name: (\w+)/.exec(text);
  if (named) member = named[1];
  else if (text.startsWith('turns:')) member = '';
  const turn = /^\s+- (\w+) -> (\w+)/.exec(text);
  const at = text.startsWith('mission:') ? 'mission'
    : turn ? `turn:${turn[1]}->${turn[2]}`
      : member ? `node:${member}` : '';
  linesOfFile.push({ text, at });
}

// rect is one element's place, the way the browser answers it.
const rect = (top, height) => ({ top, bottom: top + height, height, left: 0, right: 0, width: 0, x: 0, y: top });
Object.defineProperty(HTMLElement.prototype, 'clientHeight', {
  configurable: true,
  get() { return this.classList && this.classList.contains('ybox') ? 100 : 0; },
});
HTMLElement.prototype.getBoundingClientRect = function bounding() {
  if (this.classList && this.classList.contains('ybox')) return rect(0, 100);
  const parent = this.parentElement;
  if (parent && parent.classList && parent.classList.contains('ybox')) {
    const i = [...parent.children].indexOf(this);
    return rect(i * 20 - (parent.scrollTop || 0), 20);
  }
  return rect(0, 0);
};

test('the preview draws the check\'s lines and is sent to the line a turn adds', async () => {
  history.replaceState({}, '', '/teams?open=core');
  layout('teams-data', island({
    members: [
      { name: 'lead', role: 'lead', profile: 'coder' },
      { name: 'analyst', role: 'analyst', profile: 'coder' },
      { name: 'reviewer', role: 'reviewer', profile: 'coder' },
    ],
    turns: [['human', 'lead'], ['lead', 'human'], ['analyst', 'lead']],
  }));
  const added = { text: '  - analyst -> reviewer', at: 'turn:analyst->reviewer' };
  const asked = serve((method, url, body) => {
    if (url.startsWith('/api/team/check') && body && body.part === 'team') {
      const op = (body.doc.ops || []).find((one) => one.kind === 'addTurn' && one.from === 'analyst' && one.to === 'reviewer');
      const lines = op ? [...linesOfFile, added] : linesOfFile;
      return {
        problems: [], members: [],
        preview: { lines, diff: { added: op ? [lines.length - 1] : [], removedBefore: [] }, chars: 0 },
      };
    }
    if (url.startsWith('/api/team/check')) return { problems: [] };
    if (url.startsWith('/api/role')) return { items: [{ name: 'lead' }] };
    return undefined;
  });
  await import('../src/teams/main');
  await settle();

  const panel = '.tdef[data-name="core"] .panel.team-p';
  click(document.querySelector(`${panel} .cv-sh [data-cv-side=yaml]`));
  await settle();

  // Y6's drawing: the box shows the lines the check answered, in order.
  const drawn = () => [...document.querySelectorAll(`${panel} .tprev .yl .yt`)].map((el) => el.textContent);
  expect(drawn()).toEqual(file);
  expect(document.querySelectorAll(`${panel} .tprev .yl.ins`).length).toBe(0);

  // A turn added from analyst's card is an op of the unsaved draft; its line
  // stands marked as added.
  click(document.querySelector(`${panel} .cv-sh [data-cv-side=attr]`));
  click(document.querySelector(`${panel} .cv-n.member[data-cv-n=analyst]`));
  await settle();
  choose(document.querySelector(`${panel} .cv-side select[data-cv-f=turn-add]`), 'reviewer');
  await settle();
  click(document.querySelector(`${panel} .cv-sh [data-cv-side=yaml]`));
  await settle();

  const line = document.querySelector(`${panel} .tprev .yl.ins`);
  expect(line).not.toBe(null);
  expect(line.textContent).toContain('analyst -> reviewer');

  // R2: the box was sent to the line, and the line stands inside it.
  const box = document.querySelector(`${panel} .tprev .ybox`);
  const br = box.getBoundingClientRect(), lr = line.getBoundingClientRect();
  expect(lr.top >= br.top - 1 && lr.bottom <= br.bottom + 1, 'the box was not sent to the line the turn added').toBe(true);

  // The line came from the check of the unsaved draft.
  const last = asked.filter((one) => one.url.startsWith('/api/team/check') && one.body.part === 'team').pop();
  expect(last.body.doc.ops.some((one) => one.kind === 'addTurn' && one.from === 'analyst' && one.to === 'reviewer')).toBe(true);
});
