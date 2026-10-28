// CANVAS-PROMPT, the lines: the attribute column draws the check's own
// segments, every line of the prompt once, the folded documents by their line
// ranges, and the template's chip from the segment's origin (r-4cf9, ac-5bfa).
// That the check's segments join to `up --dry`'s prompt is
// internal/app's TestCANVAS_PROMPT_TheCheckSegmentsAreThePlans; here the answer
// is the test's, so what the column makes of it is what is read.
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
import { checkAnswer, click, docText, foldsOf, island, linesOf, memberSegments, real } from './teams-page.js';

test('every line of a member\'s prompt is drawn once, and a folded document by its range', async () => {
  history.replaceState({}, '', '/teams?open=core');
  layout('teams-data', island());
  const asked = serve((method, url, body) => {
    if (url.startsWith('/api/team/check') && body && body.part === 'team') return checkAnswer(memberSegments);
    if (url.startsWith('/api/team/check') && body && body.part === 'agreement') return { problems: [] };
    if (url.startsWith('/api/team/check')) return { problems: [] };
    if (url.startsWith('/api/role')) return { items: [{ name: 'lead', places: { project: { doc: { title: '進行' } } } }] };
    return undefined;
  });
  await import('../src/teams/main');
  await settle();

  const node = document.querySelector('.tdef[data-name="core"] .cv-n.member[data-cv-n="lead"]');
  expect(node).not.toBe(null);
  click(node);
  await settle();

  const box = document.querySelector('.tdef[data-name="core"] .cv-prompt');
  expect(box).not.toBe(null);
  const want = docText(memberSegments).split('\n');
  while (want.length && want[want.length - 1] === '') want.pop();
  const shown = linesOf(box);
  expect(shown.size).toBeGreaterThan(0);

  // Every shown line is the prompt's own line at that number.
  for (const [i, text] of shown) {
    expect(real(text), `line ${i + 1}`).toBe(real(want[i] ?? ''));
  }

  // Every line is shown once, or stands inside one folded document.
  const count = new Array(want.length).fill(0);
  for (const i of shown.keys()) if (count[i] !== undefined) count[i]++;
  const folds = foldsOf(box);
  const role = folds.find((fold) => fold.doc === 'role');
  const team = folds.find((fold) => fold.doc === 'team');
  expect(role, 'the role document is folded').toBeTruthy();
  expect(team, 'the team rules are folded').toBeTruthy();
  for (const fold of folds) if (fold.doc) for (let i = fold.a; i < fold.b; i++) count[i]++;
  expect(count.map((n, i) => [n, i]).filter(([n]) => n !== 1), 'every line sits in one place').toEqual([]);

  // A fold's range holds the segment's own lines (the blank line its block
  // carries to the next one aside), and names the document the owner screen
  // opens.
  const roleLines = memberSegments.find((segment) => segment.kind === 'role_doc').text.split('\n');
  while (roleLines.length && roleLines[roleLines.length - 1] === '') roleLines.pop();
  const body = want.slice(role.a, role.b);
  while (body.length && body[body.length - 1] === '') body.pop();
  expect(body).toEqual(roleLines);
  expect(role.role).toBe('lead');
  expect(role.place).toBe('project');
  expect(role.box.textContent).toContain('.hergy/roles/lead.yaml');

  // The template's own chip says where the template came from.
  const tpl = [...box.querySelectorAll('.cv-ptpl')].map((el) => el.textContent);
  expect(tpl.join(' ')).toContain('ひな形: hergy の既定');

  // The column is the check's reading, not one the screen sent on its own.
  expect(asked.some((one) => one.url.startsWith('/api/team/check') && one.body.part === 'team')).toBe(true);
});
