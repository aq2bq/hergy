import { test, expect } from './fixtures';
import { ready } from './helpers';

// r-f0be: the attribute panel splits into 属性, YAML and プロンプト. The prompt
// is read only under プロンプト, the pick the person made stays while the tab
// stays, and a pick no prompt is built for says what to pick (ac-fd51).
test.use({ reads: 'teams' });

test('PROMPT-TAB-1 the prompt stands under its own tab only', async ({ page, app }, info) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/teams?open=core`); await ready(page);
  const side = page.locator('.tdef[data-name="core"] .cv-side');
  const promptTab = side.getByRole('button', { name: 'プロンプト', exact: true });
  const attrTab = side.getByRole('button', { name: '属性', exact: true });
  await expect(side.locator('[data-cv-side]')).toHaveCount(3);
  await expect(attrTab).toHaveClass(/on/);

  await page.locator('.cv-n.member[data-cv-n="lead"]').click();
  // 属性 carries attributes alone; the prompt is not there.
  await expect(side.locator('[data-cv-f="m-name"]')).toBeVisible();
  await expect(side.locator('.cv-prompt')).toHaveCount(0);
  await expect(side.locator('.cv-topology-prompt')).toHaveCount(0);

  await promptTab.click();
  await expect(promptTab).toHaveClass(/on/);
  await expect(side.locator('.cv-topology-prompt')).toBeVisible();
  const entry = side.getByRole('button', { name: '全文と差分を読む', exact: true });
  await expect(entry).toBeVisible();
  await expect(side.locator('.cv-f')).toHaveCount(0);

  // Picking another member keeps the tab where the person put it.
  await page.locator('.cv-n.member[data-cv-n="analyst"]').click();
  await expect(promptTab).toHaveClass(/on/);
  await expect(side.locator('.cv-topology-prompt')).toBeVisible();

  // The human point hands no prompt, so the tab says what to pick.
  await page.locator('.cv-n.human').click();
  await expect(side.locator('.cv-prompt-none')).toBeVisible();
  await expect(side.locator('.cv-prompt-none')).toContainText('メンバーかワーカーを選ぶと');

  for (const scheme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: scheme });
    await page.locator('.cv-n.member[data-cv-n="lead"]').click();
    await expect(side.locator('.cv-topology-prompt')).toBeVisible();
    await page.screenshot({ path: info.outputPath(`prompt-member-${scheme}.png`) });
    await page.locator('.cv-n.worker[data-cv-w="0"]').click();
    await expect(promptTab).toHaveClass(/on/);
    // The preview is read before the copy is taken, not the waiting words.
    await expect(side.locator('details.cv-prompt .cv-pr')).toBeVisible();
    await page.screenshot({ path: info.outputPath(`prompt-worker-${scheme}.png`) });
  }
});

test('PROMPT-TAB-2 a worker\'s preview stands open under the tab', async ({ page, app }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/teams?open=core`); await ready(page);
  const side = page.locator('.tdef[data-name="core"] .cv-side');
  await page.locator('.cv-n.worker[data-cv-w="0"]').click();
  await expect(side.locator('.cv-prompt')).toHaveCount(0);
  await side.getByRole('button', { name: 'プロンプト', exact: true }).click();
  const preview = side.locator('details.cv-prompt');
  await expect(preview).toHaveAttribute('open', '');
  await expect(preview).toContainText('プロンプト（hire のときに渡る文）');

  // 属性 still offers the worker's own fields, and the preview is not there.
  await side.getByRole('button', { name: '属性', exact: true }).click();
  await expect(side.locator('[data-cv-f="w-profile"]')).toBeVisible();
  await expect(side.locator('details.cv-prompt')).toHaveCount(0);
});
