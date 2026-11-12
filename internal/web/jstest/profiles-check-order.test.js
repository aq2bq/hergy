import { expect, test } from 'bun:test';
import { layout, later, serve } from './page.js';

test('only an accepted profile check changes the problems and save gate', async () => {
  layout('profiles-data', {});
  const pending = [];
  serve((method, url) => {
    if (url === '/api/profile/check') {
      const answer = later();
      pending.push(answer);
      return answer.promise;
    }
    return undefined;
  });
  const { addRow, rows, changeDraft, checkPanel, saveBlocked } = await import('../src/profiles/state');
  const row = addRow('project', '');
  const key = row.panels[0].key;
  changeDraft(key, { name: 'builder', kind: 'claude', args: [], env: [] });
  const panel = () => rows.value.find((r) => r.key === row.key).panels[0];
  const problem = { sev: 'err', field: 'name', text: 'name is taken' };
  const assertBlocked = () => {
    expect(panel().problems).toEqual([problem]);
    expect(panel().errors).toBe(1);
    expect(saveBlocked(panel())).toBe(true);
  };
  const start = () => {
    const done = checkPanel(key);
    return { done, answer: pending.at(-1) };
  };

  const old = start();
  const newest = start();
  newest.answer.resolve({ problems: [problem] });
  await newest.done;
  assertBlocked();
  old.answer.resolve({ problems: [] });
  await old.done;
  assertBlocked();

  const superseded = start();
  const current = start();
  superseded.answer.reject(new Error('aborted'));
  await superseded.done;
  assertBlocked();
  current.answer.reject(new Error('offline'));
  await current.done;
  assertBlocked();

  const valid = start();
  valid.answer.resolve({ problems: [] });
  await valid.done;
  expect(panel().problems).toEqual([]);
  expect(panel().errors).toBe(0);
  expect(saveBlocked(panel())).toBe(false);
});
