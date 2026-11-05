import { test, expect } from './fixtures';
import { ready } from './helpers';
import { mkdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';

// r-933c: a team definition's name may be up to 25 characters, and the worker
// preview still shows the hire's text. The preview's placeholder name is not a
// name anyone is hired under, so its length is not held against it (d-26ce).
test.use({ reads: 'teams' });

test('PREVIEW-LONG-1 a long definition name still shows the worker\'s text', async ({ page, app }, info) => {
  const name = 'team-with-a-long-name';
  expect(name.length).toBeGreaterThanOrEqual(20);
  const dir = path.join(app.proj, '.hergy/teams', name);
  mkdirSync(dir, { recursive: true });
  writeFileSync(path.join(dir, 'team.yaml'), [
    'version: 1',
    'description: 長い名前の定義',
    'members:',
    '  - name: lead',
    '    role: lead',
    '    profile: opus-plan',
    'workers:',
    '  - from: lead',
    '    purpose: 依存の更新',
    '    profile: codex-mini',
    'turns:',
    '  - human -> lead',
    '  - lead -> human',
    '',
  ].join('\n'));
  writeFileSync(path.join(dir, 'agreement.md'), '# 長い名前の定義\n');

  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/teams?open=${name}`); await ready(page);
  const row = page.locator(`.tdef[data-name="${name}"]`);
  await expect(row.locator('.cv-n.worker').first()).toBeVisible();
  await row.locator('.cv-n.worker').first().click();
  await row.locator('[data-cv-side="prompt"]').click();
  const preview = row.locator('.cv-side details.cv-prompt');
  await expect(preview).toBeVisible();
  await expect(preview.locator('.cv-pr')).toBeVisible();
  await expect(preview).toContainText('<ワーカー名>');
  // The herdr name's shape is not reported: the text itself is shown.
  await expect(preview).not.toContainText('herdr 名');
  await page.screenshot({ path: info.outputPath('preview-long-light.png') });
});
