// R-3C77 — a save refused because the file moved keeps what the person wrote.
// The walk is ac-ccf0's: the person is in the middle of a change, the test
// writes the file, and the save is pressed. The save does not write over the
// file, the words on the screen stay letter for letter, the notice says what the
// file holds now, and the person chooses: read it again (and go back), compare,
// or write what they wrote over it. No sentence sends them to a fresh page.
import { test, expect } from './fixtures';
import { appendFileSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { press, q, read, readTo, set, there, wait, ready } from './helpers';

// noReload is the sentence the screens no longer say (d-a34a): a refused save
// keeps the draft, so nothing asks for a page to be read again.
async function noReload(page: Parameters<typeof wait>[0]): Promise<void> {
  await readTo<string>(page, 'document.body.innerText', 'a sentence sends the person back to a fresh page').not.toContain('開き直して');
  await readTo<string>(page, 'document.body.innerText', 'a sentence says ディスク where a person reads ファイル').not.toContain('ディスク');
}

test.describe('the role screen', () => {
  test.use({ reads: 'roles' });
  for (const action of ['overwrite', 'undo'] as const) {
    test(`role prose conflict ${action} preserves the user's choice`, async ({ page, app }) => {
      const file = path.join(app.proj, '.hergy/roles/builder.md');
      await page.goto(`${app.base}/roles?open=builder`); await ready(page);
      const panel = page.locator('.role.open .panel');
      const input = panel.getByRole('textbox', { name: '散文の全文' });
      const typed = '# 編集中の本文\n空白  \n';
      await input.fill(typed); appendFileSync(file, '\nほかの編集\n');
      const theirs = readFileSync(file, 'utf8');
      await panel.locator('[data-act=save]').click(); await expect(panel.locator('.conflict')).toBeVisible();
      await expect(input).toHaveValue(typed); expect(readFileSync(file, 'utf8')).toBe(theirs);
      await expect(panel.locator('.cf-text')).toContainText('builder.md');
      await panel.locator('[data-act=cf-compare]').click(); await expect(panel.locator('.cf-compare')).toContainText('ほかの編集');
      if (action === 'overwrite') {
        await panel.locator('[data-act=cf-overwrite]').click(); await expect(panel.locator('.ok')).toBeVisible();
        expect(readFileSync(file, 'utf8')).toBe(typed);
      } else {
        await panel.locator('[data-act=cf-revert]').click(); await expect(input).toHaveValue(theirs);
        await panel.locator('[data-act=cf-undo]').click(); await expect(input).toHaveValue(typed);
      }
    });
  }
});

test.describe('the team screen', () => {
  test.use({ reads: 'teams' });

  test('R3C77-3 the definition’s description is kept and written over', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'teams', 'core', 'team.yaml');
    const shape = '.tdef[data-name="core"] .panel.team-p';
    await page.goto(`${app.base}/teams?open=core`);
    await wait(page, there(`${shape} [data-act=t-save]`));
    const typed = 'ほかのところと重ならない説明の書き直し。';
    await set(page, q('.tdef[data-name="core"] .in-description'), typed);
    appendFileSync(file, '\n# ほかのところが書いた行\n');
    const theirs = readFileSync(file, 'utf8');
    await press(page, q(`${shape} [data-act=t-save]`));
    await wait(page, there(`${shape} .conflict`));
    expect(readFileSync(file, 'utf8'), 'the refused save wrote the file').toBe(theirs);
    await readTo<string>(page, `${q('.tdef[data-name="core"] .in-description')}.value`, 'the refused save lost the description').toBe(typed);
    await press(page, q(`${shape} [data-act=cf-overwrite]`));
    await wait(page, there(`${shape} .ok`));
    await readTo<boolean>(page, `${q(`${shape} .conflict`)}===null`, 'the notice stayed after the overwrite').toBe(true);
    expect(readFileSync(file, 'utf8'), 'the overwrite did not write the description').toContain(typed);
  });

  test('R3C77-4 team prose refuses a stale hash and allows explicit overwrite', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy/teams/core/agreement.md');
    await page.goto(`${app.base}/teams?open=core&agree=core`); await ready(page);
    const panel = page.locator('.tdef[data-name="core"] .agree-p');
    const input = panel.getByRole('textbox', { name: '散文の全文' });
    const typed = '# チームの下書き\n'; await input.fill(typed);
    appendFileSync(file, '\nほかの編集\n'); const theirs = readFileSync(file, 'utf8');
    await panel.locator('[data-act=save]').click(); await expect(panel.locator('.conflict')).toBeVisible();
    expect(readFileSync(file, 'utf8')).toBe(theirs); await expect(input).toHaveValue(typed);
    await panel.locator('[data-act=cf-overwrite]').click(); await expect(panel.locator('.ok')).toBeVisible();
    expect(readFileSync(file, 'utf8')).toBe(typed);
  });

  test('R3C77-5 the YAML tab keeps its text and writes it over', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'teams', 'core', 'team.yaml');
    await page.goto(`${app.base}/teams/yaml`);
    await wait(page, there('.yed .ytext'));
    const before = await read<string>(page, `document.querySelector('.yed .ytext').value`);
    const typed = `${before}\n# 書きかけの行\n`;
    await set(page, q('.yed .ytext'), typed);
    appendFileSync(file, '\n# ほかのところが書いた行\n');
    const theirs = readFileSync(file, 'utf8');
    await wait(page, `!${q('.yed [data-act=yaml-save]')}.disabled`);
    await press(page, q('.yed [data-act=yaml-save]'));
    await wait(page, there('.yed .conflict'));
    expect(readFileSync(file, 'utf8'), 'the refused save wrote the YAML file').toBe(theirs);
    await readTo<string>(page, `${q('.yed .ytext')}.value`, 'the refused save lost the text').toBe(typed);
    await press(page, q('.yed [data-act=cf-overwrite]'));
    await wait(page, there('.yed .ok'));
    const written = readFileSync(file, 'utf8');
    expect(written, 'the overwrite did not write the text').toContain('# 書きかけの行');
    await noReload(page);
  });
});

test.describe('the profile screen', () => {
  test.use({ reads: 'profiles' });

  test('R3C77-6 a profile is kept and written over', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'profiles.yaml');
    const panel = '[data-key^="sonnet-fast|"]';
    await page.goto(`${app.base}/profiles?open=sonnet-fast`);
    await wait(page, there(`${panel} [data-act=save]`));
    const typed = '21000';
    await set(page, `${q(panel)}.querySelector('[data-f=ev]')`, typed);
    appendFileSync(file, '# ほかのところが書いた行\n');
    const theirs = readFileSync(file, 'utf8');
    await press(page, q(`${panel} [data-act=save]`));
    await wait(page, there(`${panel} .conflict`));
    expect(readFileSync(file, 'utf8'), 'the refused save wrote profiles.yaml').toBe(theirs);
    await readTo<string>(page, `${q(panel)}.querySelector('[data-f=ev]').value`, 'the refused save lost the value').toBe(typed);
    await press(page, q(`${panel} [data-act=cf-overwrite]`));
    await wait(page, there(`${panel} .ok`));
    await readTo<boolean>(page, `${q(`${panel} .conflict`)}===null`, 'the notice stayed after the overwrite').toBe(true);
    expect(readFileSync(file, 'utf8'), 'the overwrite did not write the value').toContain(typed);
  });
});
