// PROFILES-P1, PROFILES-P11, PROFILES-C1 and PROFILES-ADB6 — walked in a
// browser. Only what the browser alone can see is compared here (d-3e43): the
// row a person presses opens inside the view (d-d475), the box that rewrites the
// people using a renamed profile is ticked from the start, a person's chip puts
// the cursor on the person's row of the assign tab, the YAML text a person
// types is still theirs after the check answered, and the delete of a profile
// goes through whatever shape the file wrote profiles: in (ac-adb6).
import { test, expect } from './fixtures';
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';

test.use({ reads: 'profiles' });

const row = (name: string) => `#rows .row[data-name="${name}"]`;

test('PROFILES-P1 rows open in place', async ({ page, app }) => {
  await page.goto(`${app.base}/profiles`);
  // Both layers hold this name; this test edits the project's profile.
  const profile = page.locator(`${row('sonnet-fast')}:not([data-group="global"])`);
  await profile.locator('.sum').click();
  await expect(profile.locator('.detail')).toBeVisible();
  await expect(profile.locator('.detail')).toBeInViewport();

  await page.goto(`${app.base}/profiles?open=sonnet-fast`);
  const panel = profile.locator('.panel[data-file=project]');
  await panel.locator('[data-f=name]').fill('sonnet-5.5');
  await expect(panel.locator('.rename')).toBeVisible();
  await expect(panel.locator('.rename input')).toBeChecked();
});

test('PROFILES-P11 usage jumps', async ({ page, app }) => {
  await page.goto(`${app.base}/profiles?open=opus-plan`);
  await page.locator(`${row('opus-plan')}[data-group="global"] .uchip`).first().click();
  await expect(page.locator('.team[data-g=core] .arow[data-k="m-0"]')).toHaveClass(/(^|\s)flash(\s|$)/);
  await expect.poll(() => page.evaluate(() =>
    document.activeElement?.closest('.arow')?.getAttribute('data-k'),
  )).toBe('m-0');
});

test('PROFILES-C1 typing in the YAML tab survives the check, and the save writes it', async ({ page, app }) => {
  // The check answers after typing. Its redraw must keep the typed text and
  // caret; an error closes the save only when that check has answered. Revert
  // restores the file, and a valid edit must still save (PROFILES-C1).
  await page.goto(`${app.base}/profiles/yaml`);
  const editor = page.locator('.yed').first();
  const field = editor.locator('.ytext');
  const save = editor.locator('[data-act=yaml-save]');
  await expect(editor.locator('.vlist .vhead')).toBeVisible();
  const file = await editor.locator('.path').textContent();
  const before = await field.inputValue();
  const typed = 'profiles:\n  x: [\n';

  await field.fill(typed);
  await expect(save, 'the check must close the save over an invalid draft').toBeDisabled();
  await expect(field, 'the check must not replace the typed text').toHaveValue(typed);
  await expect(editor.locator('.vitem').first(), 'the check must show the text problem').toBeVisible();
  await editor.locator('.vitem').first().click();
  await expect.poll(() => field.evaluate((element) => {
    const input = element as HTMLTextAreaElement;
    return input.value.slice(0, input.selectionStart).split('\n').length;
  }), { message: 'pressing the problem must move the caret to its line' }).toBe(2);

  await editor.locator('[data-act=yaml-revert]').click();
  await expect(field, 'revert must restore the file text').toHaveValue(before);
  await field.fill(`${before}\n# a line the check takes\n`);
  await save.click();
  await expect(editor.locator('.ok'), 'the save must confirm it wrote the edit').toHaveText('保存しました');
  expect(readFileSync(path.join(app.proj, file!), 'utf8').endsWith('# a line the check takes\n')).toBe(true);
});

test('PROFILES-ADB6 a profile is deleted from a file hergy could not rewrite', async ({ page, app }) => {
  // ac-adb6, r-430b: the master deleted a profile and hergy stopped with its own
  // bug, so the profile stayed. The file wrote profiles: in a shape the line edit
  // could not take an entry out of: the only profile of a block mapping (the
  // lines left a bare `profiles:`, which reads as a null value and not as the
  // empty mapping the delete means), and a flow mapping on the key's own line
  // (the entry stands on the line the key stands on, so the line edit took the
  // key's line). Both deletes go through, and the file keeps every line the
  // delete did not have to change.
  const file = path.join(app.proj, '.hergy', 'profiles.yaml');
  const cases = [
    { name: 'adb6-one', text: 'version: 1\nprofiles:\n  adb6-one:\n    kind: claude\n', want: 'version: 1\nprofiles: {}\n' },
    { name: 'adb6-doc', text: '---\nversion: 1\nprofiles:\n  adb6-doc:\n    kind: claude\n', want: '---\nversion: 1\nprofiles: {}\n' },
    { name: 'adb6-flow', text: 'version: 1\nprofiles: {adb6-flow: {kind: claude}, adb6-keep: {kind: codex}}\n', want: 'version: 1\nprofiles: {adb6-keep: {kind: codex}}\n' },
  ];
  for (const one of cases) {
    writeFileSync(file, one.text, 'utf8');
    await page.goto(`${app.base}/profiles?open=${one.name}`);
    const panel = `${row(one.name)} .panel[data-file=project]`;
    await page.locator(`${panel} [data-act=del]`).click();
    const answer = page.waitForResponse((r) => r.request().method() === 'DELETE' && r.url().includes(`/api/profile/${one.name}`));
    await page.locator(`${panel} [data-act=del-yes]`).click();
    const res = await answer;
    if (res.status() !== 200) {
      throw new Error(`the delete of ${one.name} was refused (${res.status()}): ${await res.text()}`);
    }
    if (readFileSync(file, 'utf8') !== one.want) {
      throw new Error(`the delete of ${one.name} wrote ${JSON.stringify(readFileSync(file, 'utf8'))}`);
    }
    await expect(page.locator(row(one.name))).toHaveCount(0);
  }
});
