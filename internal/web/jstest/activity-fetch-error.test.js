import { expect, test } from 'bun:test';
import { later, serve, settle } from './page.js';

test('activity shows a failed fetch, recovers, and retains cards after a later failure', async () => {
  document.body.innerHTML = '<div id="list"></div><script id="activity-words" type="application/json">{"none":"No teams are running"}</script>';
  const answers = [later(), later(), later(), later()];
  let request = 0;
  serve((method, url) => url === '/api/activity' ? answers[request++].promise : undefined);
  const state = await import('../src/activity/state');
  const { Board } = await import('../src/activity/board');
  const { h, render } = await import('../src/lib/preact');
  const { Refusal } = await import('../src/lib/api');
  render(h(Board, {}), document.getElementById('list'));
  const first = state.refresh();
  answers[0].reject(new Refusal(503, 'The team list could not be read', null));
  await first;
  await settle();
  expect(document.querySelector('[data-role="hq-error"]')?.textContent).toBe('The team list could not be read');
  expect(document.querySelector('#list .gone')).toBe(null);

  const second = state.refresh();
  answers[1].resolve({ items: [{ name: 'core', project: '', summary: { definition: 'core', state: 'working' } }], herdr: { ok: true } });
  await second;
  await settle();
  expect(document.querySelector('[data-role="hq-error"]')).toBe(null);
  expect(document.querySelector('.hq-tile')?.dataset.team).toBe('core');

  const third = state.refresh();
  answers[2].reject(new TypeError('Failed to fetch'));
  await third;
  await settle();
  expect(document.querySelector('[data-role="hq-error"]')?.textContent).toBe('チームの状態を読めませんでした');
  expect(document.querySelector('.hq-tile')?.dataset.team).toBe('core');

  const fourth = state.refresh();
  answers[3].resolve({ items: [], herdr: { ok: true } });
  await fourth;
  await settle();
  expect(document.querySelector('[data-role="hq-error"]')).toBe(null);
  expect(document.querySelector('#list .gone')?.textContent).toBe('No teams are running');
});
