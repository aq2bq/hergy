// TEAMS-COPY-P1 — r-a028 (ac-f07b): a definition that stands under a listed
// project's head is that project's file, so the copy the screen makes of it
// carries that project as its base, stands on the answer, and offers its save.
//
// The answers are the test's, so what the screen makes of them is what is read.
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';
import { copyOf, page, save, seed, why } from './teams-copy-place.js';

test('a copy of a listed project’s definition carries that project as its base, and its save is offered', async () => {
  page();
  const asked = serve((method, url) => {
    if (url.startsWith('/api/team/check')) return seed();
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await import('../src/teams/main');
  await settle();

  const copy = await copyOf();
  expect(copy).not.toBe(undefined);
  const check = asked.find((one) => one.url.startsWith('/api/team/check') && one.body && one.body.place === 'global');
  expect(check).not.toBe(undefined);
  // Without the project the server reads the file of the project it was started
  // in, and the copy has no base at all: its save cannot be pressed.
  expect(check.body.base).toEqual({ place: 'project', name: 'beta-dev', project: '/work/beta' });

  // The row stands on that answer, so its save is offered, and nothing has to be
  // said about a save that is not shut.
  expect(save(copy).disabled).toBe(false);
  expect(why(copy)).toBe('');

  save(copy).dispatchEvent(new Event('click', { bubbles: true }));
  await settle();
  const wrote = asked.find((one) => one.method === 'PUT' && one.url.startsWith('/api/team/beta-dev'));
  expect(wrote).not.toBe(undefined);
  expect(wrote.body.place).toBe('global');
  expect(wrote.body.base).toEqual({ place: 'project', name: 'beta-dev', project: '/work/beta' });
});
