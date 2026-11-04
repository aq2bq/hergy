// R-B41C — a save of one's own is not refused at the next one. The master's
// steps are the first walk: add a rule, save, add another rule, save. Every
// screen that writes a file does it three times without opening the page again,
// and the file holds the last words that were typed (ac-cca6).
//
// The cause was hergy's own: a save writes the person's lines, gives the rules
// their ids after it, and answered with the text from before the ids, so the
// screen's next save carried a hash the file no longer had. A save's answer is
// the file the write left, and every screen puts the answer's files under its
// feet (r-b41c).
import { test, expect } from './fixtures';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { press, q, set, there, wait, ready } from './helpers';

// save presses one 保存: the request is out, and it was taken.
async function save(page: Parameters<typeof wait>[0], find: string, at: string): Promise<void> {
  const answer = page.waitForResponse((one) => one.request().method() === 'PUT' && one.url().includes(at));
  await press(page, find);
  const answer0 = await answer;
  expect(answer0.status(), `the save at ${at} was refused: ${await answer0.text()}`).toBe(200);
}

test.describe('the role screen', () => {
  test.use({ reads: 'roles' });
  test('RB41C-1 a role is saved three times without stale hashes', async ({ page, app }) => {
    await page.goto(`${app.base}/roles?open=builder`); await ready(page);
    const panel = page.locator('.role.open .panel[data-l=project]');
    const input = panel.getByRole('textbox', { name: '散文の全文' });
    let text = await input.inputValue();
    for (let i = 1; i <= 3; i++) {
      text += `\n${i} 回目の保存。\n`;
      await input.fill(text); await panel.locator('[data-act=save]').click();
      await expect(panel.locator('.ok')).toBeVisible();
      expect(readFileSync(path.join(app.proj, '.hergy/roles/builder.md'), 'utf8')).toBe(text);
    }
  });
  test('RB41C-5 saving one document leaves another document unchanged', async ({ page, app }) => {
    const lead = path.join(app.proj, '.hergy/roles/lead.md');
    const original = readFileSync(lead, 'utf8');
    await page.goto(`${app.base}/roles?open=builder&open=lead`); await ready(page);
    const builder = page.locator('.role.open').filter({ has: page.locator('.nm').filter({ hasText: /^builder$/ }) }).locator('.panel');
    await builder.getByRole('textbox', { name: '散文の全文' }).fill('# 更新した設計担当\n');
    await builder.locator('[data-act=save]').click(); await expect(builder.locator('.ok')).toBeVisible();
    expect(readFileSync(lead, 'utf8')).toBe(original);
    await page.locator('.role').filter({ has: page.locator('.nm').filter({ hasText: /^lead$/ }) }).locator('.sum').click();
    const leadPanel = page.locator('.role.open').filter({ has: page.locator('.nm').filter({ hasText: /^lead$/ }) }).locator('.panel');
    await leadPanel.getByRole('textbox', { name: '散文の全文' }).fill('# 更新した進行担当\n');
    await leadPanel.locator('[data-act=save]').click(); await expect(leadPanel.locator('.ok')).toBeVisible();
    expect(readFileSync(lead, 'utf8')).toBe('# 更新した進行担当\n');
  });
});

test.describe('the profile screen', () => {
  test.use({ reads: 'profiles' });

  test('RB41C-2 a profile is saved three times in a row', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'profiles.yaml');
    await page.goto(`${app.base}/profiles?open=sonnet-fast`);
    const panel = '[data-key^="sonnet-fast|"]';
    await wait(page, there(`${panel} [data-act=save]`));
    const values = ['11001', '12001', '13001'];
    for (const [i, value] of values.entries()) {
      await set(page, `${q(panel)}.querySelector('[data-f=ev]')`, value);
      await wait(page, `!${q(`${panel} [data-act=save]`)}.disabled`);
      await save(page, q(`${panel} [data-act=save]`), '/api/profile/sonnet-fast');
      await wait(page, there(`${panel} .ok`));
      expect(readFileSync(file, 'utf8'), `the save ${i + 1} did not write ${value}`).toMatch(new RegExp(`MAX_THINKING_TOKENS: ["']?${value}`));
    }
  });
});

test.describe('the team screen', () => {
  test.use({ reads: 'teams' });

  // shape is the row's topology panel: the save that writes team.yaml.
  const shape = '.tdef[data-name="core"] .panel.team-p';

  test('RB41C-3 a team definition’s description is saved three times in a row', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy', 'teams', 'core', 'team.yaml');
    await page.goto(`${app.base}/teams?open=core`);
    await wait(page, there(`${shape} [data-act=t-save]`));
    const descriptions = ['説明を 1 度書き直した。', '説明を 2 度書き直した。', '説明を 3 度書き直した。'];
    for (const [i, description] of descriptions.entries()) {
      await set(page, q('.tdef[data-name="core"] .in-description'), description);
      await wait(page, `!${q(`${shape} [data-act=t-save]`)}.disabled`);
      await save(page, q(`${shape} [data-act=t-save]`), '/api/team/core');
      await wait(page, there(`${shape} .ok`));
      expect(readFileSync(file, 'utf8'), `the save ${i + 1} did not write the description`).toContain(`description: "${description}"`);
    }
  });

  test('RB41C-4 a team prose document is saved three times in a row', async ({ page, app }) => {
    const file = path.join(app.proj, '.hergy/teams/core/agreement.md');
    await page.goto(`${app.base}/teams?open=core&agree=core`); await ready(page);
    const panel = page.locator('.tdef[data-name="core"] .agree-p');
    const input = panel.getByRole('textbox', { name: '散文の全文' });
    let text = await input.inputValue();
    for (let i = 1; i <= 3; i++) {
      text += `\n${i} 回目の保存。\n`;
      await input.fill(text); await panel.locator('[data-act=save]').click();
      await expect(panel.locator('.ok')).toBeVisible();
      expect(readFileSync(file, 'utf8')).toBe(text);
      await expect(page.locator('.tdef[data-name="core"] .c-size')).toContainText([...text].length.toLocaleString());
    }
  });
});
