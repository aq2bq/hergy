// ROLES-P14 and ROLES-R1 — walked in a browser. Only
// what the browser alone can see is compared here (d-3e43): the role a person
// picks being the one line a save writes, and the switch whose day a save
// writes. The warning a document as a whole puts on the preview's first line
// (R4) and the text a rule's save carries are the module's
// (internal/web/jstest/roles-warning.test.js).
import { test, expect } from './fixtures';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { changedLines, ready } from './helpers';

test.use({ reads: 'roles' });

test('ROLES-P14 the person whose role is swapped', async ({ page, app }) => {
  // P14・P6: one person's role is picked in its card, and the save writes that
  // one line of team.yaml.
  const file = path.join(app.proj, '.hergy', 'teams', 'research', 'team.yaml');
  const before = readFileSync(file, 'utf8');

  await page.goto(`${app.base}/roles/assign`);
  await ready(page);
  const row = page.locator('.team[data-t=research]');
  await row.locator('.arow[data-m=scout-2] select').selectOption('writer');
  await row.locator('[data-act=t-save]').click();
  await expect(row.locator('.ok')).toBeVisible();

  const changed = changedLines(before, readFileSync(file, 'utf8'));
  expect(
    changed.size,
    `the assign save changed lines, want scout-2's role alone: ${JSON.stringify([...changed])}`,
  ).toBe(1);
  for (const [line, text] of changed) {
    expect(text, `line ${line} became, want scout-2's new role`).toBe('    role: writer');
  }
});
