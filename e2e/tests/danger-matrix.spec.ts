// r-ccf8 / ac-4ac5 / ac-ddb6 / ac-d0fb — the three tables the danger work is
// held to: what cannot be taken back is asked for and only 実行 does it, the
// confirmation's words are the things that go, and what can be taken back comes
// back once, with its 取り消す where the person pressed.
//
// The walks press Locators, not the page's DOM: a helper that reaches into the
// page to drive it would hide the very presses a person makes (d-3e43).
import { test, expect } from './fixtures';
import type { Locator, Page } from '@playwright/test';
import { ready } from './helpers';
import { existsSync, mkdirSync, readFileSync, realpathSync, renameSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

// OUT is where the pictures of the changed looks stand (the two themes).
const OUT = fileURLToPath(new URL('../../.local/2026-11-06-danger/', import.meta.url));
mkdirSync(OUT, { recursive: true });

const ask = (page: Page) => page.locator('.confirm');
// body is the confirmation's words about what goes: the server's lines (or
// changed's), not the button's label.
const body = (page: Page) => page.locator('.confirm .confirm-body');
const run = (page: Page) => ask(page).locator('.btn.danger.solid');
const giveUp = (page: Page) => ask(page).getByRole('button', { name: 'やめる', exact: true });
const notice = (page: Page) => page.locator('.undo');

// firstLine is one line of a file, read where the walk needs it.
const text = (file: string): string => readFileSync(file, 'utf8');

// ---- table 2: what the confirmation says is what goes --------------------
// The lines name the thing and the file it stands in; after 実行 the same thing
// is gone from the same place, and nothing else is.
test.describe('delete: the words and the loss agree', () => {
  test.use({ reads: 'teams' });

  test('a profile with two entries: the file stays, the other entry stays', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'profiles.yaml');
    writeFileSync(file, 'version: 1\nprofiles:\n  two-a:\n    kind: claude\n    args: []\n  two-b:\n    kind: claude\n    args: []\n');
    await page.goto(`${app.base}/profiles?open=two-a`); await ready(page);
    const panel = page.locator('.row[data-name="two-a"] .panel[data-file=project]');
    await panel.locator('[data-act="del"]').click();
    await expect(body(page)).toContainText('two-a');
    await expect(body(page)).toContainText('1 項目');
    await expect(body(page)).toContainText('ファイルは残ります');
    await expect(body(page)).toContainText('取り消せません');
    await run(page).click();
    await expect(ask(page)).toHaveCount(0);
    await expect(page.locator('.row[data-name="two-a"]')).toHaveCount(0);
    expect(existsSync(file), 'the confirmation said the file stays').toBe(true);
    expect(text(file), 'the deleted entry stayed').not.toContain('two-a:');
    expect(text(file), 'the other entry went with it').toContain('two-b:');
  });

  test('a profile with one entry: the file stays, empty', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'profiles.yaml');
    writeFileSync(file, 'version: 1\nprofiles:\n  solo:\n    kind: claude\n    args: []\n');
    await page.goto(`${app.base}/profiles?open=solo`); await ready(page);
    const panel = page.locator('.row[data-name="solo"] .panel[data-file=project]');
    await panel.locator('[data-act="del"]').click();
    await expect(body(page)).toContainText('1 項目 solo');
    await expect(body(page)).toContainText('ファイルは残ります');
    await run(page).click();
    await expect(page.locator('.row[data-name="solo"]')).toHaveCount(0);
    expect(existsSync(file), 'the confirmation said the file stays').toBe(true);
    expect(text(file), 'the deleted entry stayed').not.toContain('solo:');
  });

  test('a team definition with its agreement: the folder goes with both files', async ({ page, app }, info) => {
    const dir = path.join(app.proj, '.hergy', 'teams', 'core');
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    await page.locator('.tdef[data-name="core"] [data-act="del"]').click();
    await expect(body(page)).toContainText('team.yaml');
    await expect(body(page)).toContainText('agreement.md');
    await expect(body(page)).toContainText('フォルダも消えます');
    await expect(body(page)).toContainText('取り消せません');
    // The body is the server's lines now: the two themes of that look.
    for (const scheme of ['light', 'dark'] as const) {
      await page.emulateMedia({ colorScheme: scheme });
      await page.screenshot({ path: path.join(OUT, `confirm-lines-${scheme}.png`) });
    }
    await page.emulateMedia({ colorScheme: 'light' });
    await run(page).click();
    await expect(page.locator('.tdef[data-name="core"]')).toHaveCount(0);
    expect(existsSync(dir), 'the folder the confirmation said goes stayed').toBe(false);
  });

  test('a team definition with no agreement: only team.yaml is said and goes', async ({ page, app }) => {
    const dir = path.join(app.proj, '.hergy', 'teams', 'bare');
    mkdirSync(dir, { recursive: true });
    writeFileSync(path.join(dir, 'team.yaml'), text(path.join(app.proj, '.hergy', 'teams', 'docs', 'team.yaml')));
    await page.goto(`${app.base}/teams?open=bare`); await ready(page);
    await page.locator('.tdef[data-name="bare"] [data-act="del"]').click();
    await expect(body(page)).toContainText('team.yaml');
    await expect(body(page)).not.toContainText('agreement.md');
    await expect(body(page)).toContainText('フォルダも消えます');
    await run(page).click();
    await expect(page.locator('.tdef[data-name="bare"]')).toHaveCount(0);
    expect(existsSync(dir), 'the folder the confirmation said goes stayed').toBe(false);
  });

  test('a team definition whose folder holds another file: that file is said to stay, and stays', async ({ page, app }) => {
    const dir = path.join(app.proj, '.hergy', 'teams', 'core');
    writeFileSync(path.join(dir, 'notes.txt'), 'keep me\n');
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    await page.locator('.tdef[data-name="core"] [data-act="del"]').click();
    await expect(body(page)).toContainText('team.yaml');
    await expect(body(page)).toContainText('agreement.md');
    await expect(body(page)).toContainText('フォルダには notes.txt が残ります');
    await expect(body(page)).not.toContainText('フォルダも消えます');
    await run(page).click();
    // The definition is gone; the folder the words said stays, with its file.
    await expect(page.locator('.tdef[data-name="core"]')).toContainText('問題');
    expect(existsSync(path.join(dir, 'team.yaml')), 'team.yaml stayed').toBe(false);
    expect(existsSync(path.join(dir, 'agreement.md')), 'agreement.md stayed').toBe(false);
    expect(text(path.join(dir, 'notes.txt')), 'the file the confirmation said stays went').toBe('keep me\n');
  });

  test('a role three people read: the count is said, the teams are not touched', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'roles', 'builder.md');
    const core = path.join(app.proj, '.hergy', 'teams', 'core', 'team.yaml');
    const migrate = path.join(app.proj, '.hergy', 'teams', 'migrate', 'team.yaml');
    const before = { core: text(core), migrate: text(migrate) };
    await page.goto(`${app.base}/roles?open=builder`); await ready(page);
    const panel = page.locator('.role.open .panel[data-l=project]');
    await panel.locator('[data-act="del"]').click();
    await expect(body(page)).toContainText('builder.md');
    await expect(body(page)).toContainText('3 人が読んでいます');
    await expect(body(page)).toContainText('取り消せません');
    await run(page).click();
    await expect(ask(page)).toHaveCount(0);
    // The confirmation closes before the delete is asked for (lib/confirm.tsx),
    // so the file is read after the screen shows the delete came back: the
    // saved role's own delete button is gone with it.
    await expect(panel.locator('[data-act="del"]')).toHaveCount(0);
    expect(existsSync(file), 'the role the confirmation said goes stayed').toBe(false);
    expect(text(core), 'the team that reads the role moved').toBe(before.core);
    expect(text(migrate), 'the team that reads the role moved').toBe(before.migrate);
  });
});

// ---- table 3: what can be taken back comes back once --------------------
// The notice stands where the person pressed, and 取り消す puts the same thing
// back — for the figure, one operation.
test.describe('undo: one press takes it back where it was pressed', () => {
  test.use({ reads: 'teams' });

  test('the figure: a member', async ({ page, app }) => {
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    const panel = page.locator('.tdef[data-name="core"] .panel.team-p');
    const point = panel.locator('.cv-n.member[data-cv-n="impl-b"]');
    await point.click();
    await page.keyboard.press('Delete');
    await expect(notice(page)).toBeVisible();
    await expect(panel.locator('.undo')).toContainText('impl-b を外しました');
    await expect(panel.locator('.undo button')).toHaveText('取り消す');
    await expect(point).toHaveCount(0);
    await panel.locator('.undo button').click();
    await expect(point).toBeVisible();
    await expect(notice(page)).toHaveCount(0);
  });

  test('the figure: a turn', async ({ page, app }) => {
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    const panel = page.locator('.tdef[data-name="core"] .panel.team-p');
    const line = panel.locator('.cv-hit').first();
    await line.focus();
    await page.keyboard.press('Enter');
    await page.keyboard.press('Delete');
    await expect(panel.locator('.undo')).toContainText('のターンを外しました');
    await panel.locator('.undo button').click();
    await expect(panel.locator('.undo')).toHaveCount(0);
    // The turn is back: the draft is the file's again, and the line it was
    // drawn on shows its two directions.
    await expect(panel.locator('[data-act="t-save"]')).toBeDisabled();
    await line.focus();
    await page.keyboard.press('Enter');
    await expect(panel.locator('.cv-side .cv-t')).toContainText('⇄');
  });

  test('the figure: one direction of a line says that direction', async ({ page, app }) => {
    // A turn that goes one way only (the sample's lines go both ways): removing
    // the line takes that direction, and the notice names it with →.
    const file = path.join(app.proj, '.hergy/teams/core/team.yaml');
    writeFileSync(file, text(file).replace('  - human -> lead\n', '  - human -> lead\n  - analyst -> reviewer\n'));
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    const panel = page.locator('.tdef[data-name="core"] .panel.team-p');
    const line = panel.locator('.cv-hit[data-cv-e*="analyst"][data-cv-e*="reviewer"]');
    await line.focus();
    await page.keyboard.press('Enter');
    await page.keyboard.press('Delete');
    await expect(panel.locator('.undo')).toContainText('analyst → reviewer のターンを外しました');
    await panel.locator('.undo button').click();
    await expect(panel.locator('.undo')).toHaveCount(0);
    await expect(panel.locator('[data-act="t-save"]')).toBeDisabled();
  });

  test('the figure: a worker', async ({ page, app }) => {
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    const panel = page.locator('.tdef[data-name="core"] .panel.team-p');
    await panel.locator('.cv-n.worker').first().click();
    const remove = panel.getByRole('button', { name: 'ワーカーを外す' });
    await remove.click();
    await expect(panel.locator('.undo')).toContainText('ワーカー');
    await expect(panel.locator('.undo button')).toHaveText('取り消す');
    await panel.locator('.undo button').click();
    await expect(panel.locator('.undo')).toHaveCount(0);
    await expect(panel.getByRole('button', { name: 'ワーカーを外す' })).toHaveCount(0);
  });

  test('a draft: 元に戻す in the save band', async ({ page, app }) => {
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    const panel = page.locator('.tdef[data-name="core"] .panel.team-p');
    const field = panel.locator('.in-description');
    const before = await field.inputValue();
    await field.fill(`${before}書きかけ。`);
    await panel.locator('[data-act="t-revert"]').click();
    await expect(field).toHaveValue(before);
    await expect(panel.locator('.undo')).toContainText('保存した内容に戻しました');
    await panel.locator('.undo button').click();
    await expect(field).toHaveValue(`${before}書きかけ。`);
    await expect(panel.locator('.undo')).toHaveCount(0);
  });

  test('a conflict: 元に戻す reads the file in, 取り消す puts the draft back', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'roles', 'builder.md');
    const typed = '# 競合の下書き\n';
    const theirs = '# ほかが書いた本文\n';
    await page.goto(`${app.base}/roles?open=builder`); await ready(page);
    const panel = page.locator('.role.open .panel[data-l=project]');
    const input = panel.getByRole('textbox', { name: '規則の全文' });
    await input.fill(typed);
    writeFileSync(file, theirs);
    await panel.locator('[data-act=save]').click();
    await expect(panel.locator('.conflict')).toBeVisible();
    await panel.locator('[data-act=cf-revert]').click();
    await expect(input).toHaveValue(theirs);
    await expect(panel.locator('.undo')).toContainText('ファイルの今の内容に戻しました');
    await panel.locator('.undo button').click();
    await expect(input).toHaveValue(typed);
    await expect(panel.locator('.undo')).toHaveCount(0);
  });
});

test.describe('undo: the headquarters takes a project out of the list', () => {
  test.use({ reads: 'hq' });

  const listed = (page: Page) => page.locator('.hq-sheet .hq-sec').first().locator('.hq-li .hq-pn');
  const cfgFile = (app: { cfg: string }) => path.join(app.cfg, 'hergy', 'config.yaml');
  // launched writes the launch record the sample does not have: a project that
  // leaves the list stays on the sheet as a candidate, and that row is where
  // 取り消す stands (d-aeeb). The path is the directory as the machine spells it
  // (a symlinked temporary home would otherwise name the same project twice).
  const launched = (app: { state: string; home: string }): void => {
    const dir = path.join(app.state, 'hergy', 'launches');
    mkdirSync(dir, { recursive: true });
    const line = (name: string): string => JSON.stringify({
      time: '2026-11-01T00:00:00Z', command: 'up', hergy: 'e2e',
      project: realpathSync(path.join(app.home, 'src', name)),
    });
    writeFileSync(path.join(dir, 'log.jsonl'), ['alpha', 'beta', 'gamma'].map(line).join('\n') + '\n');
  };
  // The lead's own shape: a comment above every project, so a restore that
  // slips between a comment and its item is caught (d-2cce).
  const BEFORE = 'version: 1\nlang: ja\nprojects:\n  # 一つ目\n  - ~/src/alpha\n  # 二つ目\n  - ~/src/beta\n  # 三つ目\n  - ~/src/gamma\n';

  const cases = [
    { name: 'alpha', where: 'first', rest: ['beta', 'gamma'] },
    { name: 'beta', where: 'middle', rest: ['alpha', 'gamma'] },
    { name: 'gamma', where: 'last', rest: ['alpha', 'beta'] },
  ];
  for (const one of cases) {
    test(`the ${one.where} project comes back where it stood, comments and all`, async ({ page, app }) => {
      launched(app);
      const file = cfgFile(app);
      writeFileSync(file, BEFORE);
      await page.goto(`${app.base}/teams`); await ready(page);
      await page.locator('[data-hq="projects"]').click();
      const sheet = page.locator('.hq-sheet');
      await expect(sheet).toBeVisible();
      await expect(listed(page)).toHaveText(['alpha', 'beta', 'gamma']);
      const row = sheet.locator('.hq-li').filter({ hasText: one.name });
      await row.locator('[data-hq="unlist"]').click();
      await expect(listed(page)).toHaveText(one.rest);
      // The notice stands in the row that names that project now (the side it
      // moved to), not in the row the next project took over (d-aeeb).
      const moved = sheet.locator('.hq-li').filter({ hasText: one.name });
      await expect(moved.locator('.undo')).toContainText('並びから外しました');
      await expect(moved.locator('.undo button')).toHaveText('取り消す');
      await expect(sheet.locator('.hq-li').filter({ hasText: one.rest[0] }).locator('.undo')).toHaveCount(0);
      if (one.name === 'beta') {
        for (const scheme of ['light', 'dark'] as const) {
          await page.emulateMedia({ colorScheme: scheme });
          await sheet.screenshot({ path: path.join(OUT, `unlist-undo-${scheme}.png`) });
        }
        await page.emulateMedia({ colorScheme: 'light' });
      }
      await moved.locator('.undo button').click();
      await expect(listed(page)).toHaveText(['alpha', 'beta', 'gamma']);
      expect(text(file), 'config.yaml came back different').toBe(BEFORE);
    });
  }

  // outOfTheList takes beta out and answers nothing; the notice stands in beta's
  // row, as the walk above saw.
  const outOfTheList = async (page: Page, app: { base: string; state: string; home: string }, file: string): Promise<void> => {
    launched(app);
    writeFileSync(file, BEFORE);
    await page.goto(`${app.base}/teams`); await ready(page);
    await page.locator('[data-hq="projects"]').click();
    const sheet = page.locator('.hq-sheet');
    await expect(listed(page)).toHaveText(['alpha', 'beta', 'gamma']);
    await sheet.locator('.hq-li').filter({ hasText: 'beta' }).locator('[data-hq="unlist"]').click();
    await expect(listed(page)).toHaveText(['alpha', 'gamma']);
    await expect(sheet.locator('.hq-li').filter({ hasText: 'beta' }).locator('.undo')).toContainText('並びから外しました');
  };
  // buildAgain presses the band's project button a second time, so the sheet and
  // every row in it are made afresh. A redraw after the write (hergy:wrote, 150 ms
  // later) does the same to the row the notice stands in; this press does it at a
  // moment the walk chooses (d-2cb2).
  const buildAgain = (page: Page) => page.locator('[data-hq="projects"]').click();
  // fallback is where the notice says what it did once its row is gone: the
  // shared place below the band, not a screen's own (d-2cb2).
  const fallback = (page: Page) => page.locator('.undo-fallback .undo');

  test('a notice outlives the sheet being built again, and 取り消す puts the project back', async ({ page, app }) => {
    const file = cfgFile(app);
    await outOfTheList(page, app, file);
    await buildAgain(page);
    await expect(fallback(page)).toContainText('並びから外しました');
    await fallback(page).locator('button').click();
    await expect(listed(page)).toHaveText(['alpha', 'beta', 'gamma']);
    await expect(page.locator('.undo')).toHaveCount(0);
    expect(text(file), 'config.yaml came back different').toBe(BEFORE);
  });

  test('a restore the server refuses says why it could not, after the sheet is built again', async ({ page, app }) => {
    const file = cfgFile(app);
    await outOfTheList(page, app, file);
    await buildAgain(page);
    await expect(fallback(page)).toContainText('並びから外しました');
    // The file it must put the lines back in is broken under it: the way back
    // cannot be taken, and the notice says why instead of closing as if it had.
    // The broken content goes in whole (a rename), so no reader sees a half file.
    const broken = path.join(app.cfg, 'hergy', 'config.yaml.broken');
    writeFileSync(broken, 'projects: [\n');
    renameSync(broken, file);
    await fallback(page).locator('button').click();
    const said = page.locator('.undo-fallback .undo[role="alert"]');
    await expect(said).toContainText('config.yaml');
    await expect(said).toContainText('読み取れません');
    expect(text(file), 'a refused restore changed config.yaml').toBe('projects: [\n');
  });
});

// ---- table 1: the ways a person answers, and what each does ---------------
// The same place pressed again, two Enters, Esc and やめる leave the thing as it
// was; 実行 alone does it (r-d480, d-021e).

// saysNo walks the four answers that do not run the operation. unchanged is what
// the walk checks after each: the thing is still there, letter for letter.
async function saysNo(page: Page, trigger: Locator, unchanged: () => Promise<void>, reAsk = false): Promise<void> {
  const box = (await trigger.boundingBox())!;
  // The same place pressed twice: the dialog stands in front of the button.
  await trigger.click();
  await expect(ask(page)).toBeVisible();
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
  await expect(ask(page)).toBeVisible();
  await giveUp(page).click();
  await expect(ask(page)).toHaveCount(0);
  await unchanged();
  // Enter twice. The first press takes やめる (it holds the focus); the second
  // is the page's, and a focus gone back to a tab may ask again.
  await trigger.click();
  await expect(ask(page)).toBeVisible();
  await expect(giveUp(page)).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(ask(page)).toHaveCount(0);
  await unchanged();
  // The second Enter is the page's. A focus gone back to a tab may ask again (a
  // new ask, nothing run), so the ask is closed until it stays closed.
  await page.keyboard.press('Enter');
  if (reAsk) {
    await expect.poll(async () => {
      if (await ask(page).count()) await page.keyboard.press('Escape');
      return ask(page).count();
    }).toBe(0);
  } else {
    await page.keyboard.press('Escape');
    await expect(ask(page)).toHaveCount(0);
  }
  await unchanged();
  // Esc.
  await trigger.click();
  await expect(ask(page)).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(ask(page)).toHaveCount(0);
  await unchanged();
  // やめる.
  await trigger.click();
  await giveUp(page).click();
  await expect(ask(page)).toHaveCount(0);
  await unchanged();
}

test.describe('answers: only 実行 does it', () => {
  test.use({ reads: 'teams' });

  test('a team definition is deleted only by 実行', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'teams', 'core', 'team.yaml');
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    const row = page.locator('.tdef[data-name="core"]');
    const trigger = row.locator('[data-act="del"]');
    await saysNo(page, trigger, async () => {
      expect(existsSync(file), 'the file went without 実行').toBe(true);
      await expect(row).toHaveCount(1);
    });
    await trigger.click();
    await run(page).click();
    await expect(row).toHaveCount(0);
    expect(existsSync(file), 'the file stayed after 実行').toBe(false);
  });

  test('a role document is deleted only by 実行', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'roles', 'builder.md');
    await page.goto(`${app.base}/roles?open=builder`); await ready(page);
    const panel = page.locator('.role.open .panel[data-l=project]');
    const trigger = panel.locator('[data-act="del"]');
    await saysNo(page, trigger, async () => {
      expect(existsSync(file), 'the role went without 実行').toBe(true);
      await expect(panel).toBeVisible();
    });
    await trigger.click();
    await run(page).click();
    await expect(ask(page)).toHaveCount(0);
    expect(existsSync(file), 'the role stayed after 実行').toBe(false);
  });

  test('a profile entry is deleted only by 実行', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'profiles.yaml');
    await page.goto(`${app.base}/profiles?open=codex-mini`); await ready(page);
    const panel = page.locator('.row[data-name="codex-mini"] .panel[data-file=project]');
    const trigger = panel.locator('[data-act="del"]');
    await saysNo(page, trigger, async () => {
      expect(text(file), 'the entry went without 実行').toContain('codex-mini:');
      await expect(panel).toBeVisible();
    });
    await trigger.click();
    await run(page).click();
    await expect(page.locator('.row[data-name="codex-mini"]')).toHaveCount(0);
    expect(text(file), 'the entry stayed after 実行').not.toContain('codex-mini:');
  });

  test('a new team definition with input goes only by 実行', async ({ page, app }) => {
    await page.goto(`${app.base}/teams`); await ready(page);
    await page.locator('.list .hq-gh [data-act="new"]').first().click();
    const draft = page.locator('.tdef[data-name^="~new"]');
    await draft.locator('.in-name').fill('kept-name');
    const trigger = draft.locator('[data-act="discard"]');
    await saysNo(page, trigger, async () => {
      await expect(draft).toHaveCount(1);
      await expect(draft.locator('.in-name')).toHaveValue('kept-name');
    });
    await trigger.click();
    await run(page).click();
    await expect(draft).toHaveCount(0);
  });

  test('a new role with input goes only by 実行', async ({ page, app }) => {
    await page.goto(`${app.base}/roles`); await ready(page);
    await page.locator('[data-act="new"]').first().click();
    const draft = page.locator('.role .panel[data-mode=new], .role .panel').last();
    await draft.locator('.in-name').fill('kept-role');
    const trigger = draft.locator('[data-act="discard"]');
    await saysNo(page, trigger, async () => {
      await expect(draft.locator('.in-name')).toHaveValue('kept-role');
    });
    await trigger.click();
    await run(page).click();
    await expect(page.locator('.role .panel .in-name').filter({ hasText: 'kept-role' })).toHaveCount(0);
  });

  test('a new profile with input goes only by 実行', async ({ page, app }) => {
    await page.goto(`${app.base}/profiles`); await ready(page);
    await page.locator('[data-act="new"]').first().click();
    const fresh = page.locator('.row .panel').first();
    await fresh.locator('.in-name').fill('kept-profile');
    const trigger = fresh.locator('[data-act="discard"]');
    await saysNo(page, trigger, async () => {
      await expect(fresh.locator('.in-name')).toHaveValue('kept-profile');
    });
    await trigger.click();
    await run(page).click();
    await expect(fresh.locator('.in-name')).toHaveCount(0);
  });

  test('a team edit leaves by the tab strip only by 実行', async ({ page, app }) => {
    await page.goto(`${app.base}/profiles/yaml`); await ready(page);
    const area = page.locator('.ytext').first();
    const draft = `${await area.inputValue()}\n# まだ保存していない\n`;
    await area.fill(draft);
    const trigger = page.getByRole('tab', { name: '一覧', exact: true });
    await saysNo(page, trigger, async () => {
      await expect(page).toHaveURL(/profiles\/yaml/);
      await expect(area).toHaveValue(draft);
    }, true);
    await trigger.click();
    await run(page).click();
    await expect(page).toHaveURL(/profiles$/);
  });

  test('a role edit closes by its link only by 実行', async ({ page, app }) => {
    await page.goto(`${app.base}/teams?open=core`); await ready(page);
    const row = page.locator('.tdef[data-name="core"]');
    await row.locator('[data-cv-n="impl-a"]').click();
    await row.getByRole('button', { name: 'ロールを確認・編集' }).first().click();
    await row.getByRole('button', { name: 'すべて表示', exact: true }).click();
    const column = row.locator('.role-column').filter({ has: page.getByRole('heading', { name: /builder/ }) });
    const input = column.getByRole('textbox', { name: '規則の全文' });
    await input.fill('# 閉じる前の下書き\n');
    const trigger = column.getByRole('link', { name: 'ロールの画面で開く' });
    await saysNo(page, trigger, async () => {
      await expect(page).toHaveURL(/teams\?open=core/);
      await expect(input).toHaveValue('# 閉じる前の下書き\n');
    });
    await trigger.click();
    await run(page).click();
    await expect(page).toHaveURL(/roles\?/);
  });
});

test.describe('answers: a team stop, from the activity and the definition', () => {
  test.use({ reads: 'activity' });

  test('the activity tile asks, and only 実行 opens the stop', async ({ page, app }) => {
    await page.goto(`${app.base}/activity`); await ready(page);
    const trigger = page.locator('.hq-tile[data-team="core"] [data-hq="stop"]');
    await saysNo(page, trigger, async () => {
      await expect(page.locator('.hq-sheet')).toHaveCount(0);
      await expect(page.locator('.hq-tile[data-team="core"]')).toHaveCount(1);
    });
    await trigger.click();
    await run(page).click();
    await expect(page.locator('.hq-sheet')).toBeVisible();
  });

  test('the definition’s run cell asks, and only 実行 opens the stop', async ({ page, app }) => {
    await page.goto(`${app.base}/teams`); await ready(page);
    const trigger = page.locator('.tdef .hq-runc [data-hq="stop"]').first();
    await saysNo(page, trigger, async () => {
      await expect(page.locator('.hq-sheet')).toHaveCount(0);
    });
    await trigger.click();
    await run(page).click();
    await expect(page.locator('.hq-sheet')).toBeVisible();
  });
});
