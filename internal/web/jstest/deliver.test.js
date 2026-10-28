// SHOWCASE-D1, 変更を伝える (r-a8fa, ac-92ea): the band counts the running
// members whose prompt is not the one hergy last handed them; the sheet starts
// with the members a send may reach picked, and the ones working or waiting for
// a permission cannot be picked; a save only reads the number again, and sends
// nothing; a send hands the picked members' prompts and takes them off the band.
// The answers are the test's, so what the part makes of them is what is read.
// The mark on the activity screen's member row is that screen's own press
// (activity/board.js), so the walk that opens the sheet from it stands with the
// screen's test (activity-stale.test.js).
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';

// layout writes the band the part mounts its button in, and a place for the
// sheet and the notice.
function layout() {
  document.body.innerHTML = '<div class="wrap"><div class="bar"><nav class="screens"></nav></div><div id="view"></div></div>';
}

const change = (document, rule, text) => ({ kind: 'rule', document, rule, text });
// answer is the sheet's reading: core's five members, three of them stale, and
// only the two builders may be sent to.
const answer = {
  herdr: { ok: true },
  teams: [{
    name: 'core', definition: 'core',
    members: [
      { name: 'core-impl-a', state: 'handed', can_send: true, changes: [change('.hergy/roles/builder.yaml', 'BUILD-5', '規則 BUILD-5 を書き換えた')] },
      { name: 'core-lead', state: 'active', can_send: false, changes: [] },
      { name: 'core-reviewer', state: 'blocked', can_send: false, changes: [] },
      { name: 'core-impl-b', state: 'idle', can_send: true, changes: [change('.hergy/roles/builder.yaml', 'BUILD-5', '規則 BUILD-5 を書き換えた')] },
    ],
  }],
};

const bandCount = () => Number(document.querySelector('.bar .dlv-bandbtn b')?.textContent ?? -1);
const boxes = () => [...document.querySelectorAll('.dlv-sheet input[type=checkbox]')];
const checked = () => boxes().filter((el) => el.checked).map((el) => el.dataset.dlvName).join(',');
const disabled = () => boxes().filter((el) => el.disabled).map((el) => el.dataset.dlvName).join(',');

test('the band, the sheet and the send hold the members a change was not told to', async () => {
  history.replaceState({}, '', '/activity');
  layout();
  let count = 3;
  const asked = serve((method, url, body) => {
    if (url === '/api/delivery/count') return { count };
    if (url === '/api/delivery') return answer;
    if (method === 'POST' && url === '/api/delivery') return { ok: true };
    return undefined;
  });
  await import('../src/lib/deliver');
  await settle();

  // The band's number, and the button that opens the sheet.
  expect(bandCount()).toBe(3);
  expect(document.querySelector('.bar .dlv-bandbtn').hidden).toBe(false);
  document.querySelector('.bar .dlv-bandbtn').click();
  await settle();

  // The sheet: the members a send may reach are picked; the ones working or
  // waiting for a permission are not.
  expect(checked()).toBe('core-impl-a,core-impl-b');
  expect(disabled()).toBe('core-lead,core-reviewer');
  // The changed rule opens its own place.
  expect(document.querySelector('.dlv-sheet .dlv-chg').getAttribute('href')).toBe('/roles?open=builder&rule=BUILD-5');
  // Opening the sheet sent nothing.
  expect(asked.filter((one) => one.method === 'POST')).toEqual([]);

  // Closing it asks for nothing either.
  document.querySelector('.dlv-sheet [data-dlv=close]').click();
  await settle();
  expect(document.querySelector('.dlv-sheet')).toBe(null);
  expect(asked.filter((one) => one.method === 'POST')).toEqual([]);

  // A save does not open the sheet and does not send: it reads the number again.
  count = 4;
  document.dispatchEvent(new CustomEvent('hergy:saved'));
  await settle();
  expect(document.querySelector('.dlv-sheet')).toBe(null);
  expect(bandCount()).toBe(4);
  expect(asked.filter((one) => one.method === 'POST')).toEqual([]);

  // The band's button opens the sheet again, with the answer's own picks.
  document.querySelector('.bar .dlv-bandbtn').click();
  await settle();
  expect(document.querySelector('.dlv-sheet')).not.toBe(null);

  // Sending hands the picked members' prompts, in the answer's own order, and
  // takes them off the band; the notice says who it reached.
  count = 2;
  document.querySelector('.dlv-sheet [data-dlv=send]').click();
  await settle();
  const sent = asked.filter((one) => one.method === 'POST');
  expect(sent.length).toBe(1);
  expect(sent[0].body.members).toEqual(['core-impl-a', 'core-impl-b']);
  expect(document.querySelector('.dlv-sheet')).toBe(null);
  expect(document.querySelector('.dlv-toast').textContent).toBe('core-impl-a・core-impl-b に伝えました');
  expect(bandCount()).toBe(2);
});
