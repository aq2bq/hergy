// CANVAS-PROMPT, the worker: a worker's name and task are chosen at hire, so
// the column asks the check with two values of its own and draws slots where
// they stand. Putting the values back gives the prompt the server answered,
// the same one `hire --dry` builds with those values (internal/app's
// TestCANVAS_PROMPT_TheCheckSegmentsAreThePlans).
import { expect, test } from 'bun:test';
import { layout, serve, settle } from './page.js';
import { checkAnswer, click, docText, island, memberSegments } from './teams-page.js';

test('a worker\'s prompt shows the slots its name and task stand in', async () => {
  history.replaceState({}, '', '/teams?open=core');
  layout('teams-data', island());
  const workerSegments = [
    { kind: 'template', text: 'あなたは core-zz-worker-name-0 のワーカーです。\n依頼: zzTaskPlaceholder0\n', origin: { kind: 'hergy' } },
    { kind: 'herdr', text: '## 連絡のしかた\n\nherdr で送る。\n', origin: { kind: 'hergy' } },
  ];
  const asked = serve((method, url, body) => {
    if (url.startsWith('/api/team/check') && body && body.part === 'worker') return { prompt: workerSegments };
    if (url.startsWith('/api/team/check') && body && body.part === 'team') return checkAnswer(memberSegments);
    if (url.startsWith('/api/team/check')) return { problems: [] };
    if (url.startsWith('/api/role')) return { items: [{ name: 'lead' }] };
    return undefined;
  });
  await import('../static/teams/main.js');
  await settle();

  click(document.querySelector('.tdef[data-name="core"] .cv-n.worker'));
  await settle();

  const box = document.querySelector('.tdef[data-name="core"] .cv-prompt');
  expect(box).not.toBe(null);
  // The column asked with the values it shows as slots, the same ones hire
  // --dry is run with.
  const ask = asked.filter((one) => one.url.startsWith('/api/team/check') && one.body.part === 'worker').pop();
  expect(ask.body.doc.name).toBe('zz-worker-name-0');
  expect(ask.body.doc.task).toBe('zzTaskPlaceholder0');
  expect(ask.body.doc.purpose).toBe('実装');

  const slots = [...box.querySelectorAll('.cv-slot')];
  expect(slots.map((el) => el.dataset.v)).toEqual(['zz-worker-name-0', 'zzTaskPlaceholder0']);
  expect(slots.map((el) => el.textContent)).toEqual(['<ワーカー名>', '<依頼>']);

  // Put the values back and the lines are the prompt the server sent.
  const shown = [];
  box.querySelectorAll('.yl[data-ln] .yt').forEach((el) => {
    const copy = el.cloneNode(true);
    copy.querySelectorAll('.cv-slot').forEach((slot) => { slot.textContent = slot.dataset.v; });
    shown.push(copy.textContent);
  });
  while (shown.length && shown[shown.length - 1] === '') shown.pop();
  const want = docText(workerSegments).split('\n');
  while (want.length && want[want.length - 1] === '') want.pop();
  expect(shown).toEqual(want);
});
