// lib/resource.js: a reading whose answer is state. Only the newest request's
// answer is kept (webui-preact-design 5 章 3), so no screen numbers its
// requests to tell an old answer from the one it is waiting for.
import { expect, test } from 'bun:test';
import { resource } from '../static/lib/resource.js';

// later is a promise the test settles by hand, so an answer comes exactly when
// the test says and no test waits on a clock.
function later() {
  let settle;
  const promise = new Promise((resolve, reject) => { settle = { resolve, reject }; });
  return { promise, ...settle };
}

test('the newest request wins even when an older answer comes after it', async () => {
  const answers = [later(), later()];
  let n = 0;
  const r = resource(() => answers[n++].promise);
  const first = r.run();
  const second = r.run();
  answers[1].resolve('new');
  await second;
  answers[0].resolve('old');
  await first;
  expect(r.value.value).toBe('new');
  expect(r.loading.value).toBe(false);
});

test('a run aborts the request before it', async () => {
  const seen = [];
  const r = resource((signal) => { seen.push(signal); return later().promise; });
  r.run();
  r.run();
  expect(seen[0].aborted).toBe(true);
  expect(seen[1].aborted).toBe(false);
});

test('a failure is kept as the error, and the next run clears it', async () => {
  const answers = [later(), later()];
  let n = 0;
  const r = resource(() => answers[n++].promise);
  const failing = r.run();
  answers[0].reject(new Error('refused'));
  await failing;
  expect(r.error.value.message).toBe('refused');
  const next = r.run();
  expect(r.error.value).toBe(null);
  answers[1].resolve('ok');
  await next;
  expect(r.value.value).toBe('ok');
});

test('abort drops the request that is out, and its answer is not kept', async () => {
  const answer = later();
  const r = resource(() => answer.promise);
  const out = r.run();
  r.abort();
  expect(r.loading.value).toBe(false);
  answer.resolve('too late');
  await out;
  expect(r.value.value).toBe(null);
});
