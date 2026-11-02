// TEAMS-COPY-P4 — r-a028 (ac-456e): a copy that grows an error the definition it
// stands on did not hold keeps its save shut (spec-teams X3), and its foot says
// the error and what to fix. The errors the base already held are the ones the
// save may keep (d-9abb): those shut nothing, and need no sentence.
import { expect, test } from 'bun:test';
import { serve, settle } from './page.js';
import { copyOf, draftRow, page, save, seed, why } from './teams-copy-place.js';

const MISSING = 'team.yaml: 説明が書かれていません。team.yaml に description: を 1 行書いてください';

test('a copy that grows an error the base did not hold keeps its save shut, and its foot says what to fix', async () => {
  page();
  serve((method, url, body) => {
    if (url.startsWith('/api/team/check') && body && body.doc && (body.doc.ops || []).length) {
      return seed({ problems: [{ sev: 'err', at: 'description', field: 'description', text: MISSING }] });
    }
    if (url.startsWith('/api/team/check')) return seed();
    if (url.startsWith('/api/role')) return { items: [] };
    return undefined;
  });
  const S = await import('../src/teams/state');
  await import('../src/teams/main');
  await settle();

  let copy = await copyOf();
  expect(save(copy).disabled).toBe(false);

  // The person clears the description the copy stands on: the error that
  // follows was not the base's, so the save shuts, and the sentence that says
  // what to fix stands beside it.
  S.pressDescription(S.allRecs().find((one) => one.draftRow), '');
  await settle();
  copy = draftRow();
  expect(save(copy).disabled).toBe(true);
  expect(why(copy)).toContain('description:');
});
