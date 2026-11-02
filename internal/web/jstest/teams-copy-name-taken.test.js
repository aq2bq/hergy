// TEAMS-COPY-P3 — r-a028 (ac-456e): a copy that takes a name the destination
// already holds keeps its save shut, and its foot says the name is taken and
// what to do about it; a name that is free opens the save and takes the sentence
// away.
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';
import { copyOf, draftRow, page, save, seed, typing, why } from './teams-copy-place.js';

const TAKEN = 'チーム定義 "shared" はすでにあります: ~/.config/hergy/teams/shared（何も書き込んでいません）。別のチーム定義名を指定してください';

test('a copy whose name is taken says so, and the save opens once the name is free again', async () => {
  page();
  serve((method, url, body) => {
    if (url.startsWith('/api/team/check') && body && body.name === 'shared') {
      return seed({ problems: [{ sev: 'err', at: 'name', field: 'name', text: TAKEN }] });
    }
    if (url.startsWith('/api/team/check')) return seed();
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  await import('../src/teams/main');
  await settle();

  let copy = await copyOf();
  expect(save(copy).disabled).toBe(false);

  // The name moves to one the global layer already holds: the save shuts, and
  // the name's own sentence stands beside it.
  await typing(copy.querySelector('.in-name[data-f=name]'), 'shared');
  copy = draftRow();
  expect(save(copy).disabled).toBe(true);
  expect(why(copy)).toContain('別のチーム定義名');

  // The name is free again: the save is offered, and the sentence is gone.
  await typing(copy.querySelector('.in-name[data-f=name]'), 'beta-dev');
  copy = draftRow();
  expect(save(copy).disabled).toBe(false);
  expect(why(copy)).toBe('');
});
