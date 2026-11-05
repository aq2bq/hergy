import { test, expect } from './fixtures';
import { ready } from './helpers';
import type { Locator, Page } from '@playwright/test';

// r-ea17, ac-7fe6: after a save, a duplicate, a copy, a delete or a rename, the
// band's number is the list's rows without the page being read again, within a
// second. Each screen keeps its own rows; the band is written by the one module
// that counts them (lib/band-counts.ts).

// agree holds one screen's band to the rows its list draws, within the second
// the acceptance measures.
async function agree(page: Page, screen: string, rows: Locator): Promise<void> {
  const band = page.locator(`.scr[data-screen="${screen}"] .n`);
  await expect.poll(async () => {
    const n = await rows.count();
    return `${await band.textContent()}/${n}`;
  }, { timeout: 1000 }).toBe(`${await rows.count()}/${await rows.count()}`);
}

// saved waits for the saved row's own 保存しました, so the next count is the
// write's.
async function saved(page: Page, name: string): Promise<void> {
  const row = page.locator(`#rows .row[data-name="${name}"]`);
  await expect(row).toHaveCount(1);
  await expect(row.locator('.ok').first()).toHaveText('保存しました');
}

test.describe('profiles', () => {
  test.use({ reads: 'profiles' });

  test('the band follows a save, a duplicate, a copy, a delete and a rename', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    const rows = page.locator('#rows .row');
    await page.goto(`${app.base}/profiles`); await ready(page);

    // A new profile, saved.
    await page.locator('.list .hq-gh [data-act="new"]').first().click();
    const draft = page.locator('#rows .row').filter({ has: page.locator('[data-act="discard"]') });
    await expect(draft).toHaveCount(1);
    await draft.locator('.in-name').fill('band-new');
    await draft.locator('[data-act="save"]').click();
    await saved(page, 'band-new');
    await agree(page, 'profiles', rows);

    // A duplicate, saved.
    await page.goto(`${app.base}/profiles?open=codex-mini`); await ready(page);
    await page.locator('.row[data-name="codex-mini"] [data-act="dup"]').first().click();
    const dup = page.locator('#rows .row').filter({ has: page.locator('[data-act="discard"]') });
    await expect(dup).toHaveCount(1);
    await dup.locator('.in-name').fill('band-dup');
    await dup.locator('[data-act="save"]').click();
    await saved(page, 'band-dup');
    await agree(page, 'profiles', rows);

    // A copy into the other place, saved.
    await page.goto(`${app.base}/profiles?open=codex-mini`); await ready(page);
    await page.locator('.row[data-name="codex-mini"] [data-act="copy"]').first().click();
    const copy = page.locator('#rows .row').filter({ has: page.locator('[data-act="discard"]') });
    await expect(copy).toHaveCount(1);
    await copy.locator('.in-name').fill('band-copy');
    await copy.locator('[data-act="save"]').click();
    await saved(page, 'band-copy');
    await agree(page, 'profiles', rows);

    // A delete of the row the walk saved.
    const gone = page.locator('.row[data-name="band-new"]');
    await gone.locator('.sum').click();
    await gone.locator('[data-act="del"]').click();
    await page.locator('.confirm .btn.danger.solid').click();
    await expect(gone).toHaveCount(0);
    await agree(page, 'profiles', rows);

    // A rename, saved.
    const rename = page.locator('.row[data-name="band-dup"]');
    await rename.locator('.sum').click();
    await rename.locator('.in-name').fill('band-renamed');
    await rename.locator('[data-act="save"]').click();
    await saved(page, 'band-renamed');
    await agree(page, 'profiles', rows);
  });
});

test.describe('roles', () => {
  test.use({ reads: 'roles' });

  test('the band follows a save, a duplicate, a copy, a delete and a rename', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    const rows = page.locator('#rows .role');
    const named = (name: string) => rows.filter({ has: page.locator('.c-name .nm', { hasText: new RegExp(`^${name}$`) }) });
    await page.goto(`${app.base}/roles`); await ready(page);

    // A new role, saved.
    await page.locator('.list .hq-gh [data-act="new"]').first().click();
    const draft = rows.filter({ has: page.locator('[data-act="discard"]') });
    await expect(draft).toHaveCount(1);
    await draft.locator('[data-f="name"]').fill('band-new');
    await draft.locator('[data-act="save"]').click();
    await expect(named('band-new')).toHaveCount(1);
    await expect(named('band-new').locator('.ok').first()).toHaveText('保存しました');
    await agree(page, 'roles', rows);

    // A duplicate, saved.
    await page.goto(`${app.base}/roles?open=builder`); await ready(page);
    await named('builder').locator('[data-act="dup"]').first().click();
    const dup = rows.filter({ has: page.locator('[data-act="discard"]') });
    await expect(dup).toHaveCount(1);
    await dup.locator('[data-f="name"]').fill('band-dup');
    await dup.locator('[data-act="save"]').click();
    await expect(named('band-dup')).toHaveCount(1);
    await agree(page, 'roles', rows);

    // A copy into the other place, saved.
    await page.goto(`${app.base}/roles?open=builder`); await ready(page);
    await named('builder').locator('[data-act="copy"]').first().click();
    const copy = rows.filter({ has: page.locator('[data-act="discard"]') });
    await expect(copy).toHaveCount(1);
    await copy.locator('[data-f="name"]').fill('band-copy');
    await copy.locator('[data-act="save"]').click();
    await expect(named('band-copy')).toHaveCount(1);
    await agree(page, 'roles', rows);

    // A delete of the row the walk saved.
    await named('band-new').locator('.sum').click();
    await named('band-new').locator('[data-act="del"]').click();
    await page.locator('.confirm .btn.danger.solid').click();
    await expect(named('band-new')).toHaveCount(0);
    await agree(page, 'roles', rows);

    // A rename, saved.
    await named('band-dup').locator('.sum').click();
    // The row by the id it keeps while its name is being typed: the name column
    // follows the draft.
    const uid = await named('band-dup').getAttribute('data-uid');
    const open = page.locator(`#rows .role[data-uid="${uid}"]`);
    await open.locator('[data-f="name"]').first().fill('band-renamed');
    await open.locator('[data-act="save"]').first().click();
    await expect(named('band-renamed')).toHaveCount(1);
    await agree(page, 'roles', rows);
  });
});

test.describe('teams', () => {
  test.use({ reads: 'teams' });

  test('the band follows a save, a duplicate, a copy, a delete and a rename', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    const rows = page.locator('#view .tdef');
    const named = (name: string) => rows.filter({ has: page.locator(`[data-name="${name}"]`) });
    await page.goto(`${app.base}/teams`); await ready(page);

    // A new definition, saved: the starter's members take the profiles the
    // places hold (r-54b8), and the save writes the definition's own files.
    await page.locator('.list .hq-gh[data-place="project"] [data-act="new"]').first().click();
    const draft = page.locator('#view .tdef[data-name^="~new"]');
    await expect(draft).toHaveCount(1);
    await draft.locator('.in-name').fill('band-new');
    await expect(draft.locator('.in-name')).not.toHaveClass(/bad/);
    await expect(draft.locator('.cv-canvas svg')).toHaveCount(1);
    for (const member of ['lead', 'builder']) {
      await draft.locator(`.cv-n.member[data-cv-n="${member}"]`).click();
      const pick = draft.locator('.cv-side [data-cv-f="m-profile"]');
      await pick.selectOption('sonnet-fast');
      await expect(pick).toHaveValue('sonnet-fast');
    }
    await draft.locator('.dtabs .dtab[data-t="agree"]').click();
    const agreeTab = draft.locator('.panel.agree-p [data-act="save"]');
    await expect(agreeTab).toBeEnabled();
    await agreeTab.click();
    await expect(page.locator('#view .tdef[data-name^="~new"]')).toHaveCount(0);
    await expect(page.locator('#view .tdef[data-name="band-new"]')).toHaveCount(1);
    await agree(page, 'teams', rows);

    // A duplicate, saved.
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    await page.locator('.tdef[data-name="core"] [data-act="dup"]').click();
    const dup = page.locator('#view .tdef[data-name^="~new"]');
    await expect(dup).toHaveCount(1);
    await dup.locator('.in-name').fill('band-dup');
    await dup.locator('.dtabs .dtab[data-t="agree"]').click();
    const dupSave = dup.locator('.panel.agree-p [data-act="save"]');
    await expect(dupSave).toBeEnabled();
    await dupSave.click();
    await expect(page.locator('#view .tdef[data-name="band-dup"]')).toHaveCount(1);
    await agree(page, 'teams', rows);

    // A copy into the other place, saved.
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    await page.locator('.tdef[data-name="core"] [data-act="copy"]').click();
    const copy = page.locator('#view .tdef[data-name^="~new"]');
    await expect(copy).toHaveCount(1);
    await copy.locator('.in-name').fill('band-copy');
    await copy.locator('.dtabs .dtab[data-t="agree"]').click();
    const copySave = copy.locator('.panel.agree-p [data-act="save"]');
    await expect(copySave).toBeEnabled();
    await copySave.click();
    await expect(page.locator('#view .tdef[data-name="band-copy"]')).toHaveCount(1);
    await agree(page, 'teams', rows);

    // A delete of the definition the walk saved.
    const gone = page.locator('#view .tdef[data-name="band-new"]');
    await gone.locator('.sum').click();
    await gone.locator('.dacts [data-act="del"]').click();
    await page.locator('.confirm .btn.danger.solid').click();
    await expect(gone).toHaveCount(0);
    await agree(page, 'teams', rows);

    // A rename, saved through the rename sheet.
    // The project's own docs: the same name stands in the global layer too.
    const rename = page.locator('#view .tdef[data-name="docs"]').first();
    await rename.locator('.sum').click();
    await rename.getByRole('button', { name: '名前を変更', exact: true }).click();
    await rename.getByLabel('新しいチーム定義名').fill('band-renamed');
    await rename.getByRole('button', { name: '名前を保存', exact: true }).click();
    await expect(page.locator('#view .tdef[data-name="band-renamed"]')).toHaveCount(1);
    await agree(page, 'teams', rows);
  });
});
