// A stop may outlive the running row: panes close before herdr releases names.
// The submitted operation must keep its pending state and its eventual error.
import { expect, test } from 'bun:test';
import { later, serve, settle } from './page.js';
import { Refusal } from '../src/lib/api';

test('a late stop timeout stays visible after the running row disappears', async () => {
  document.body.innerHTML = '<div class="wrap"><div class="bar"></div><div id="tabs"></div><div id="view"></div></div>';
  let teams = [{ team: 'draft', definition: 'draft', project: '/work/atlas', project_name: 'atlas', place: 'project', workspace: { id: 'w1', label: 'atlas' }, tab: { id: 't1', label: 'draft' }, members: [], workers: [], waiting: [], starting: false }];
  const stop = later();
  serve((method, url) => {
    if (url === '/api/hq/running') return { herdr: { ok: true, text: '' }, teams };
    if (url === '/api/hq/stop') return stop.promise;
    return undefined;
  });
  const hq = await import('../src/lib/hq');
  await hq.readCore();
  const { stopSheet } = await import('../src/lib/hq/sheets');
  const { mountDialogs } = await import('../src/lib/dialogs.js');
  mountDialogs();
  stopSheet('draft'); await settle();
  document.querySelector('.confirm .btn.danger.solid').click(); await settle();
  teams = []; await hq.readCore(); await settle();
  expect(document.querySelector('.hq-sheet [role=status]').textContent).toContain('名前が消えるまで待っています');
  const message = 'draft-lead: 30000 ミリ秒待っても、herdr から名前が消えません。ペインは閉じました。hergy status を確かめてください';
  stop.reject(new Refusal(500, message, { text: message })); await settle();
  expect(document.querySelector('.hq-sheet').textContent).toContain(message);
  expect(document.querySelector('.hq-sheet [role=status]')).toBe(null);
  expect(document.querySelector('.hq-sheet .hq-h b').textContent).toBe('終了できませんでした');
  expect(document.querySelector('.hq-sheet').textContent).not.toContain('このチームは動いていません');
  expect(document.querySelector('.hq-sheet [data-hqx=close]')).not.toBe(null);
});
