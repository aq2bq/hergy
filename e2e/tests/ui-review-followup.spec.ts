import { test, expect } from './fixtures';
import { ready } from './helpers';
import { readFileSync, writeFileSync, realpathSync } from 'node:fs';
import path from 'node:path';

test.describe('review 1-8: listed projects', () => {
  test.use({ reads: 'hq' });
  for (const screen of ['profiles/yaml', 'roles/assign']) {
    test(`${screen} keeps a draft when switching is cancelled`, async ({ page, app }) => {
      await page.goto(`${app.base}/${screen}`); await ready(page);
      const project = page.getByRole('combobox', { name: '表示するプロジェクト' });
      const original = await project.inputValue();
      const beta = realpathSync(path.join(app.home, 'src', 'beta'));
      const field = screen === 'profiles/yaml' ? page.locator('.ytext').first() : page.locator('.teams select').first();
      if (screen === 'profiles/yaml') await field.fill((await field.inputValue()) + '\n# keep draft\n');
      else await field.selectOption('reviewer');
      const draft = await field.inputValue();
      const dialog = page.waitForEvent('dialog');
      const switching = project.selectOption(beta);
      const confirmation = await dialog;
      expect(confirmation.message()).toContain('保存していない変更');
      await confirmation.dismiss(); await switching;
      await expect(project).toHaveValue(original);
      await expect(field).toHaveValue(draft);
      page.once('dialog', (d) => d.accept());
      await project.selectOption(beta);
      await expect(project).toHaveValue(beta);
    });
  }
  for (const scoped of [false, true]) {
    test(`team YAML draft survives the band with project query ${scoped}`, async ({ page, app }) => {
      const beta = realpathSync(path.join(app.home, 'src', 'beta'));
      await page.goto(`${app.base}/teams/yaml${scoped ? '?project=' + encodeURIComponent(beta) : ''}`); await ready(page);
      const field = page.locator('.ytext').first();
      const draft = (await field.inputValue()) + '\n# unsaved team YAML\n';
      await field.fill(draft);
      await page.locator('.screens a[data-screen="roles"]').click(); await ready(page);
      await page.locator('.screens a[data-screen="teams"]').click(); await ready(page);
      await expect(field).toHaveValue(draft);
    });
  }
  test('a team finding opens the same-named role in its own project', async ({ page, app }) => {
    const beta = realpathSync(path.join(app.home, 'src', 'beta'));
    const file = path.join(beta, '.hergy', 'roles', 'builder.md');
    writeFileSync(file, readFileSync(file, 'utf8').replace('作る前に、確かめ方を 1 つ書く。', 'Use claude for the beta task.'));
    // Supply a role finding at the API boundary; follow its real link into the saved role.
    await page.route('**/api/team/check*', async (route) => {
      const response = await route.fetch();
      const json = await response.json();
      if (route.request().postDataJSON().name === 'beta-dev') {
        json.problems = [...(json.problems || []), { sev: 'warn', at: 'line', field: 'text', role: 'builder', text: 'beta role document' }];
      }
      await route.fulfill({ response, json });
    });
    await page.goto(`${app.base}/teams?${new URLSearchParams({ open: 'beta-dev|' + beta })}`); await ready(page);
    const link = page.locator('a.vitem').filter({ hasText: 'beta role document' });
    await expect(link).toHaveCount(1);
    await link.click(); await ready(page);
    await expect(page).toHaveURL(/project=.*beta/);
    await expect(page.locator('.role.open').getByRole('textbox', { name: '散文の全文' })).toHaveCount(1);
    await expect(page.locator('.role.open').getByRole('textbox', { name: '散文の全文' })).toHaveValue(/beta task/);
  });
  test('activity preserves the selected project and last tab', async ({ page, app }) => {
    const beta = realpathSync(path.join(app.home, 'src', 'beta'));
    await page.goto(`${app.base}/teams/yaml?project=${encodeURIComponent(beta)}`); await ready(page);
    await page.locator('.screens a[data-screen="activity"]').click(); await ready(page);
    await expect(page).toHaveURL(/activity\?project=/);
    await page.locator('.screens a[data-screen="teams"]').click(); await ready(page);
    await expect(page).toHaveURL(/teams\/yaml\?project=/);
    await expect(page.getByRole('combobox', { name: '表示するプロジェクト' })).toHaveValue(beta);
  });
  test('rule links resolve a different project and honor its group', async ({ page, app }) => {
    const beta = realpathSync(path.join(app.home, 'src', 'beta'));
    const file = path.join(beta, '.hergy', 'roles', 'builder.md');
    writeFileSync(path.join(beta, '.hergy', 'roles', 'beta-only.md'), readFileSync(file));
    for (const name of ['beta-only', 'builder']) {
      const open = name === 'builder' ? `${name}|${beta}` : name;
      await page.goto(`${app.base}/roles?${new URLSearchParams({ open, rule: `${name}.project.DAAA` })}`); await ready(page);
      await expect(page.locator('.role.open').getByRole('textbox', { name: '散文の全文' })).toHaveCount(1);
      await expect(page.locator('.role.open').getByRole('textbox', { name: '散文の全文' })).not.toBeEmpty();
    }
  });
});

test.describe('review 1-8: default project', () => {
  test.use({ reads: 'activity' });
  test('all scoped tabs identify the current project without projects configuration', async ({ page, app }) => {
    for (const screen of ['teams/yaml', 'roles/assign', 'roles/yaml', 'profiles/assign', 'profiles/yaml']) {
      await page.goto(`${app.base}/${screen}`); await ready(page);
      const project = page.getByRole('combobox', { name: '表示するプロジェクト' });
      await expect(project.locator('option')).toHaveCount(1);
      await expect(project).toHaveValue(realpathSync(app.proj));
    }
  });
  test('profile mouse and arrow tab navigation ask before discarding, modifier keys do not switch tabs', async ({ page, app }) => {
    await page.goto(`${app.base}/profiles/yaml`); await ready(page);
    const field = page.locator('.ytext').first();
    const draft = (await field.inputValue()) + '\n# unsaved\n';
    await field.fill(draft);
    for (const keyboard of [false, true]) {
      const dialog = page.waitForEvent('dialog');
      const action = keyboard ? page.getByRole('tab', { name: 'YAML', exact: true }).press('ArrowLeft') : page.getByRole('tab', { name: '一覧', exact: true }).click();
      await (await dialog).dismiss(); await action;
      await expect(field).toHaveValue(draft);
      await expect(page.getByRole('tab', { name: 'YAML', exact: true })).toBeFocused();
      await expect(page).toHaveURL(/profiles\/yaml$/);
    }
    // Ctrl+Arrow is not a tab command; unlike Alt+Left it does not navigate browser history.
    await page.getByRole('tab', { name: 'YAML', exact: true }).press('Control+ArrowLeft');
    await expect(page).toHaveURL(/profiles\/yaml$/);
    page.once('dialog', (d) => d.accept());
    await page.getByRole('tab', { name: '一覧', exact: true }).click();
    await expect(page).toHaveURL(/profiles$/);
  });
  test('approval boundaries never hide command lines or expose history', async ({ page, app }) => {
    const file = path.join(app.work, 'herdr', 'screens', 'migrate-tester.txt');
    const cases = [
      'Bash command\nrm -rf /data && \\\n echo "bash command finished"\n1. Yes\n3. No',
      'Bash command\n# step 3. normalize\nrm -rf /data\n1. Yes\n3. No',
      'bash command で確認しました\nAPI_TOKEN=abc123\nWould you like to run the following command?\nrm -rf /data\n1. Yes\n3. No',
      'bash command で確認しました\nAPI_TOKEN=abc123\nApprove this?\n1. Yes\n3. No',
      'Bash command\nrm -rf /data\n1. Yes',
      'Unknown approval format\nrm -rf /data && cat <<EOF\nBash command\nEOF\n1. Yes\n3. No',
      'Bash command\n$ ls\n1. Yes\n3. No\nDo you want to edit main.go?\n1. Yes\n3. No',
      'Bash command\n$ ls\n1. Yes\n2. No\nold output\nEdit main.go?\n1. Yes\n3. No',
      'Bash command\ncat <<EOF\n1. Yes\n3. No\nEOF\nrm -rf /data\n1. Yes\n3. No',
      'Would you like to run the following command?\nrm -rf /data && cat <<EOF\nBash command\nEOF\n1. Yes\n3. No',
    ];
    for (const [index, terminal] of cases.entries()) {
      writeFileSync(file, terminal);
      await page.goto(`${app.base}/activity`); await ready(page);
      const question = page.locator('.hq-at[data-pick="migrate"] .hq-aq');
      if (index < 3) {
        await expect(question).toContainText('rm -rf /data');
        await expect(question).toContainText('3. No');
      } else {
        await expect(question).toContainText('端末を開いて確認してください');
      }
      const response = await page.request.get(`${app.base}/api/activity/migrate`);
      const body = await response.text();
      expect(body).not.toContain('API_TOKEN');
      expect(body).not.toContain('bash command で確認しました');
    }
  });
  test('long terminal history is excluded and the command is initially visible', async ({ page, app }, testInfo) => {
    const file = path.join(app.work, 'herdr', 'screens', 'migrate-tester.txt');
    writeFileSync(file, 'private old log\n'.repeat(40) + readFileSync(file, 'utf8'));
    await page.goto(`${app.base}/activity`); await ready(page);
    const question = page.locator('.hq-at[data-pick="migrate"] .hq-aq');
    await expect(question).toContainText('0042_split_ledger.sql');
    await expect(question).not.toContainText('private old log');
    const reply = page.locator('.hq-at[data-pick="migrate"]').getByRole('button', { name: '端末で返答する' });
    await expect.poll(async () => (await reply.boundingBox())?.width || 0).toBeLessThan(240);
    await expect.poll(() => question.evaluate((el) => el.scrollHeight <= el.clientHeight)).toBe(true);
    const response = await page.request.get(`${app.base}/api/activity/migrate`);
    const body = await response.text();
    expect(body).not.toContain('private old log');
    await page.screenshot({ path: testInfo.outputPath('review-long-permission.png'), fullPage: true });
  });
  test('column terminology and dark worker headings match the UI contract', async ({ page, app }, testInfo) => {
    await page.goto(`${app.base}/teams`); await ready(page);
    await expect(page.locator('.hq-runh')).toHaveText('状態と操作');
    await page.emulateMedia({ colorScheme: 'dark' });
    await page.goto(`${app.base}/activity`); await ready(page);
    const label = page.locator('svg > text.lbl').first();
    await expect(label).toHaveText('ワーカー');
    await expect.poll(() => label.evaluate((el) => getComputedStyle(el).fill)).not.toBe('rgb(0, 0, 0)');
    await page.screenshot({ path: testInfo.outputPath('review-dark-workers.png'), fullPage: true });
  });
});
