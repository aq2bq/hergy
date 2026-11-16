import { expect, test } from 'bun:test';
import { later, serve, settle } from './page.js';

test('the empty board appears only after the first empty answer', async () => {
  document.body.innerHTML = '<div id="list"></div><script id="activity-words" type="application/json">{"none":"No teams are running"}</script>';
  const answer = later();
  serve((method, url) => url === '/api/activity' ? answer.promise : undefined);
  const state = await import('../src/activity/state');
  const { Board } = await import('../src/activity/board');
  const { h, render } = await import('../src/lib/preact');
  render(h(Board, {}), document.getElementById('list'));
  expect(state.empty.value).toBe(false);
  expect(document.querySelector('#list').textContent).toBe('');
  const first = state.refresh();
  expect(state.reading.loading.value).toBe(true);
  expect(state.empty.value).toBe(false);
  await settle();
  expect(document.querySelector('#list').textContent).toBe('');
  answer.resolve({ items: [], herdr: { ok: true } });
  await first;
  expect(state.empty.value).toBe(true);
  await settle();
  expect(document.querySelector('#list .gone').textContent).toBe('No teams are running');
  state.reading.abort();
  expect(state.empty.value).toBe(true);
});
