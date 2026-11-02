// TEAMS-COPY-P2 — r-a028 (ac-456e): a copy whose base cannot be read keeps its
// save shut, and its foot says what happened and what to do about it. Before
// this, the refusal reached nothing: the row stood with its save off and no
// sentence on it at all.
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';
import { copyOf, page, refused, save, why } from './teams-copy-place.js';

const SAID = '元にしたチーム定義 ~/.config/hergy/teams/beta-dev/team.yaml を読めませんでした（そのファイルはありません）。何も書き込んでいません。取り消してから、その行を開いてファイルを直し、もう一度 複製 か コピー をしてください';

test('a copy whose base cannot be read keeps its save shut, and its foot says why and what to do', async () => {
  page();
  serve((method, url) => {
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  refused(SAID);
  await import('../src/teams/main');
  await settle();

  const copy = await copyOf();
  expect(copy).not.toBe(undefined);
  expect(save(copy).disabled).toBe(true);
  // The server's own sentence stands beside the save: what happened, and what
  // the person does next (ac-456e).
  expect(why(copy)).toBe(SAID);
});
