import { test, expect } from './fixtures';
import { ready } from './helpers';
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';

test.use({ reads: 'teams' });

test('keyboard role dialog preserves the canvas draft, traps focus and restores its trigger', async ({ page, app }) => {
  await page.goto(`${app.base}/teams?open=core`); await ready(page);
  const row = page.locator('.tdef[data-name="core"]');
  const description = row.getByRole('textbox', { name: /の説明/ });
  await description.fill('編集中のチーム定義');
  const node = row.locator('[data-cv-n="impl-a"]'); await node.click();
  const trigger = row.getByRole('button', { name: 'ロールを確認・編集' }).first();
  await trigger.focus(); await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'ロール builder', exact: true });
  await expect(dialog.getByLabel('タイトル', { exact: true })).toBeEnabled();
  await expect(dialog.locator('.role-usage')).toContainText('roles/builder.yaml');
  await expect(dialog.locator('.role-usage')).toContainText('migrate');
  await dialog.getByRole('button', { name: '閉じる', exact: true }).focus();
  await page.keyboard.press('Shift+Tab');
  await expect.poll(() => dialog.evaluate((el) => el.contains(document.activeElement))).toBe(true);
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0); await expect(trigger).toBeFocused();
  await expect(node).toHaveClass(/sel/); await expect(description).toHaveValue('編集中のチーム定義');
});

test('saving a role changes only its file and refreshes the draft prompt without reloading', async ({ page, app }) => {
  const file = path.join(app.proj, '.hergy/roles/builder.yaml');
  const teamFile = path.join(app.proj, '.hergy/teams/core/team.yaml');
  const agreeFile = path.join(app.proj, '.hergy/teams/core/agreement.yaml');
  const teamBefore = readFileSync(teamFile, 'utf8'), agreeBefore = readFileSync(agreeFile, 'utf8');
  await page.goto(`${app.base}/teams?open=core`); await ready(page);
  const row = page.locator('.tdef[data-name="core"]');
  const description = row.getByRole('textbox', { name: /の説明/ }); await description.fill('編集中の説明');
  await row.locator('[data-cv-n="impl-a"]').click();
  await row.getByText('プロンプト（このチーム定義から渡る行）', { exact: true }).click();
  const fold = row.locator('.cv-fold[data-doc="role"]');
  const lines = await fold.locator('span').last().textContent();
  await fold.getByRole('button', { name: 'ロールを確認・編集' }).click();
  const dialog = page.getByRole('dialog', { name: 'ロール builder', exact: true });
  await expect(dialog.getByLabel('タイトル', { exact: true })).toBeEnabled();
  await dialog.locator('.rule .rsum').first().click();
  const text = dialog.locator('.rule.open [data-f="text"]').first();
  await text.fill(`${await text.inputValue()}\nモーダル保存の確認`);
  await dialog.getByRole('button', { name: 'ロールを保存', exact: true }).click();
  await expect(dialog.getByText('ロールを保存しました', { exact: true })).toBeVisible();
  await expect.poll(() => readFileSync(file, 'utf8')).toContain('モーダル保存の確認');
  await dialog.getByRole('button', { name: '閉じる', exact: true }).click();
  await expect(fold.locator('span').last()).not.toHaveText(lines!);
  await expect(description).toHaveValue('編集中の説明');
  await expect(row.locator('[data-cv-n="impl-a"]')).toHaveClass(/sel/);
  expect(readFileSync(teamFile, 'utf8')).toBe(teamBefore); expect(readFileSync(agreeFile, 'utf8')).toBe(agreeBefore);
});

test('a global definition opens the global role even when a project role has the same name', async ({ page, app }) => {
  const globalFile = path.join(app.cfg, 'hergy/roles/writer.yaml');
  const globalText = readFileSync(globalFile, 'utf8').replace(/^title:.*$/m, 'title: グローバルの執筆');
  writeFileSync(globalFile, globalText);
  writeFileSync(path.join(app.proj, '.hergy/roles/writer.yaml'), 'version: 1\ntitle: プロジェクトの執筆\nsections:\n  - rules:\n      - text: project only\n');
  await page.goto(`${app.base}/teams?open=draft`); await ready(page);
  const row = page.locator('.tdef[data-name="draft"]'); await row.locator('[data-cv-n="writer"]').click();
  await row.getByRole('button', { name: 'ロールを確認・編集' }).first().click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByLabel('タイトル', { exact: true })).toHaveValue('グローバルの執筆');
  await expect(dialog.locator('.role-usage')).toContainText('グローバル');
  await expect(dialog.locator('.role-usage')).toContainText('ほかのプロジェクトからも使える');
});

test('dirty close can be cancelled and a concurrent file change is refused without losing the draft', async ({ page, app }) => {
  const file = path.join(app.proj, '.hergy/roles/builder.yaml');
  await page.goto(`${app.base}/teams?open=core`); await ready(page);
  const row = page.locator('.tdef[data-name="core"]'); await row.locator('[data-cv-n="impl-a"]').click();
  await row.getByRole('button', { name: 'ロールを確認・編集' }).first().click();
  const dialog = page.getByRole('dialog', { name: 'ロール builder', exact: true });
  const title = dialog.getByLabel('タイトル', { exact: true }); await expect(title).toBeEnabled(); await title.fill('保存前のタイトル');
  page.once('dialog', (question) => question.dismiss()); await page.keyboard.press('Escape');
  await expect(title).toHaveValue('保存前のタイトル');
  const external = `${readFileSync(file, 'utf8')}\n# changed by another editor\n`; writeFileSync(file, external);
  await dialog.getByRole('button', { name: 'ロールを保存', exact: true }).click();
  await expect(dialog.locator('.conflict')).toBeVisible(); await expect(title).toHaveValue('保存前のタイトル');
  expect(readFileSync(file, 'utf8')).toBe(external);
  page.once('dialog', (question) => question.accept()); await dialog.getByRole('button', { name: '閉じる', exact: true }).click();
  await expect(dialog).toHaveCount(0);
});

for (const state of ['missing', 'unreadable'] as const) {
  test(`a ${state} role opens an explanation without an empty editable document`, async ({ page, app }) => {
    if (state === 'missing') {
      const file = path.join(app.proj, '.hergy/teams/core/team.yaml');
      writeFileSync(file, readFileSync(file, 'utf8').replace('role: builder', 'role: absent-role'));
    } else writeFileSync(path.join(app.proj, '.hergy/roles/builder.yaml'), '[');
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    const row = page.locator('.tdef[data-name="core"]'); await row.locator('[data-cv-n="impl-a"]').click();
    await row.getByRole('button', { name: 'ロールを確認・編集' }).first().click();
    const dialog = page.getByRole('dialog'); await expect(dialog.locator('.role-usage [role="alert"]')).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'ロールを保存', exact: true })).toHaveCount(0);
    await expect(dialog.getByRole('link', { name: 'ロールの画面で開く', exact: true })).toBeVisible();
  });
}

test('invalid rules and failed usage reads disable saving and retain the entered text', async ({ page, app }) => {
  const file = path.join(app.proj, '.hergy/roles/builder.yaml'), original = readFileSync(file, 'utf8');
  await page.goto(`${app.base}/teams?open=core`); await ready(page);
  const row = page.locator('.tdef[data-name="core"]'); await row.locator('[data-cv-n="impl-a"]').click();
  await row.getByRole('button', { name: 'ロールを確認・編集' }).first().click();
  const dialog = page.getByRole('dialog'); await expect(dialog.getByLabel('タイトル', { exact: true })).toBeEnabled();
  await dialog.locator('.rule .rsum').first().click();
  const text = dialog.locator('.rule.open [data-f="text"]').first();
  const before = await text.inputValue(); await text.fill('');
  await expect(dialog.getByRole('button', { name: 'ロールを保存', exact: true })).toBeDisabled();
  await text.fill(`${before}\n残す編集`);
  await expect(dialog.getByRole('button', { name: 'ロールを保存', exact: true })).toBeEnabled();
  await page.route('**/api/role/*/context?**', (route) => route.fulfill({ status: 503, json: { text: '使用先を確認できません' } }));
  page.once('dialog', (question) => question.accept());
  await dialog.getByRole('button', { name: '文書を読み直す', exact: true }).click();
  await expect(dialog.getByRole('alert').filter({ hasText: '使用先を確認できません' })).toBeVisible();
  await expect(text).toHaveValue(`${before}\n残す編集`);
  await expect(dialog.getByRole('button', { name: 'ロールを保存', exact: true })).toBeDisabled();
  expect(readFileSync(file, 'utf8')).toBe(original);
});

test('opening the role screen from the dialog carries the team draft back through the band', async ({ page, app }) => {
  await page.goto(`${app.base}/teams?open=core`); await ready(page);
  const row = page.locator('.tdef[data-name="core"]');
  await row.getByRole('textbox', { name: /の説明/ }).fill('画面をまたいで残す編集');
  await row.locator('[data-cv-n="impl-a"]').click();
  await row.getByRole('button', { name: 'ロールを確認・編集' }).first().click();
  await page.getByRole('dialog').getByRole('link', { name: 'ロールの画面で開く', exact: true }).click();
  await ready(page); await page.locator('.screens a[data-screen="teams"]').click(); await ready(page);
  await expect(row.getByRole('textbox', { name: /の説明/ })).toHaveValue('画面をまたいで残す編集');
});

for (const outcome of ['success', 'failure'] as const) {
  test(`mouse save keeps focus inside the role dialog after ${outcome} and one Escape restores its trigger`, async ({ page, app }) => {
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    const row = page.locator('.tdef[data-name="core"]');
    await row.locator('[data-cv-n="impl-a"]').click();
    const trigger = row.getByRole('button', { name: 'ロールを確認・編集' }).first();
    await trigger.click();
    const dialog = page.getByRole('dialog', { name: 'ロール builder', exact: true });
    const title = dialog.getByLabel('タイトル', { exact: true });
    await expect(title).toBeEnabled(); await title.fill('マウス保存後のフォーカス');
    if (outcome === 'failure') {
      await page.route('**/api/role/builder', (route) => route.fulfill({ status: 503, json: { text: '保存失敗の検査' } }));
    }
    await dialog.getByRole('button', { name: 'ロールを保存', exact: true }).click();
    await expect(dialog.getByText(outcome === 'success' ? 'ロールを保存しました' : '保存失敗の検査', { exact: true })).toBeVisible();
    await expect.poll(() => dialog.evaluate(el => el.contains(document.activeElement))).toBe(true);
    if (outcome === 'failure') page.once('dialog', question => question.accept());
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expect(trigger).toBeFocused();
  });
}

for (const viewport of [{ width: 1440, height: 900 }, { width: 1280, height: 720 }, { width: 375, height: 667 }]) {
  for (const editing of [false, true]) {
    test(`role dialog wheel returns from a long ${editing ? 'editor' : 'preview'} at ${viewport.width}×${viewport.height}`, async ({ page, app }) => {
      const text = '規則の本文を読み、作業の前提と結果を確かめてから次の作業へ進む。'.repeat(8).slice(0, 170);
      const doc = { version: 1, title: '長いロール', sections: Array.from({ length: 4 }, (_, si) => ({
        heading: `節 ${si + 1}`, rules: Array.from({ length: 10 }, (_, ri) => ({
          id: `S${si}R${ri}`, text: text + (si === 3 && ri === 9 ? '最後の行を読む' : ''),
        })),
      })) };
      writeFileSync(path.join(app.proj, '.hergy/roles/builder.yaml'), JSON.stringify(doc));
      await page.setViewportSize(viewport);
      await page.goto(`${app.base}/teams?open=core`); await ready(page);
      const row = page.locator('.tdef[data-name="core"]');
      await row.locator('[data-cv-n="impl-a"]').click();
      await row.getByRole('button', { name: 'ロールを確認・編集' }).first().click();
      const dialog = page.getByRole('dialog', { name: 'ロール builder', exact: true });
      await expect(dialog.getByLabel('タイトル', { exact: true })).toBeEnabled();
      await expect(dialog.locator('.ybox .yl').last()).toContainText('最後の行を読む');
      if (editing) await dialog.locator('.rule .rsum').first().click();
      const body = dialog.locator('.role-dialog-body');
      const readScroll = () => body.evaluate(el => {
        const preview = el.querySelector<HTMLElement>('.ybox')!, last = preview.lastElementChild!;
        const br = el.getBoundingClientRect(), pr = preview.getBoundingClientRect(), lr = last.getBoundingClientRect();
        const input = el.querySelector<HTMLTextAreaElement>('.rule.open textarea');
        return {
          top: el.scrollTop, max: el.scrollHeight - el.clientHeight, clientHeight: el.clientHeight, scrollHeight: el.scrollHeight,
          windowY: window.scrollY,
          preview: { height: pr.height, clientHeight: preview.clientHeight, scrollHeight: preview.scrollHeight, top: preview.scrollTop, overscroll: getComputedStyle(preview).overscrollBehaviorY },
          input: input ? { height: input.clientHeight, scrollHeight: input.scrollHeight, overscroll: getComputedStyle(input).overscrollBehaviorY } : null,
          lastVisible: lr.top >= Math.max(br.top, pr.top, 0) && lr.bottom <= Math.min(br.bottom, pr.bottom, innerHeight),
        };
      });
      const initial = await readScroll(), box = (await body.boundingBox())!;
      await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
      let lastSeen = false, backgroundStable = true;
      for (let i = 0; i < 40; i++) {
        await page.mouse.wheel(0, 400);
        const s = await readScroll(); lastSeen ||= s.lastVisible; backgroundStable &&= s.windowY === initial.windowY;
      }
      const down = await readScroll(); lastSeen ||= down.lastVisible;
      for (let i = 0; i < 40; i++) {
        await page.mouse.wheel(0, -400);
        const s = await readScroll(); backgroundStable &&= s.windowY === initial.windowY;
      }
      const up = await readScroll();
      await test.info().attach('scroll-measurements', { body: JSON.stringify({ viewport, editing, initial, down, up, lastSeen, backgroundStable }, null, 2), contentType: 'application/json' });
      expect(Math.abs(down.top - down.max), 'wheel reaches the bottom of the dialog body').toBeLessThanOrEqual(1);
      expect(lastSeen, 'the last preview line appears while wheeling down').toBe(true);
      await expect.poll(() => body.evaluate(el => el.scrollTop), 'wheel returns to the top of the dialog body').toBe(0);
      expect(backgroundStable, 'the page behind the dialog does not move').toBe(true);
      expect(up.windowY).toBe(initial.windowY);
      await dialog.getByRole('button', { name: '閉じる', exact: true }).click();
      await page.goto(`${app.base}/roles?open=builder`); await ready(page);
      await expect(page.locator('.role.open .ybox')).toHaveCSS('overscroll-behavior-y', 'contain');
    });
  }
}
