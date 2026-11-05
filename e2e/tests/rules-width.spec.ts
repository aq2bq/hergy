import { test, expect } from './fixtures';
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import type { Locator } from '@playwright/test';
import { ready } from './helpers';

test.use({ reads: 'teams' });

async function textWidth(input: Locator) {
  return input.evaluate((node: HTMLTextAreaElement) => {
    const style = getComputedStyle(node), canvas = document.createElement('canvas');
    const context = canvas.getContext('2d')!;
    context.font = style.font;
    const glyph = context.measureText('規').width;
    const inner = node.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
    return { inner, glyph, chars: Math.floor(inner / glyph) };
  });
}

for (const theme of ['light', 'dark'] as const) {
  test(`rules use window width for six columns and longer lines (${theme})`, async ({ page, app }, info) => {
    const teamFile = path.join(app.proj, '.hergy/teams/core/team.yaml');
    writeFileSync(teamFile, readFileSync(teamFile, 'utf8').replace('name: impl-b\n    role: builder', 'name: impl-b\n    role: writer'));
    const sample = '# 幅の確認\n' + '規'.repeat(360) + '\n';
    const roleFile = path.join(app.proj, '.hergy/roles/writer.md');
    writeFileSync(roleFile, sample);
    await page.emulateMedia({ colorScheme: theme });
    const results: { viewport: { width: number; height: number }; wrap: number; widths: number[]; normal: Awaited<ReturnType<typeof textWidth>>; wide: Awaited<ReturnType<typeof textWidth>>; wideWidths: number[]; board: { visible: number; scroll: number }; role: Awaited<ReturnType<typeof textWidth>> }[] = [];
    for (const viewport of [{ width: 1440, height: 900 }, { width: 2560, height: 1440 }]) {
      await page.setViewportSize(viewport);
      await page.goto(`${app.base}/teams?open=core&agree=core`); await ready(page);
      await page.getByRole('button', { name: 'すべて表示', exact: true }).click();
      await page.evaluate(() => document.fonts.ready);
      const row = page.locator('.tdef[data-name="core"]'), board = row.locator('.prose-board');
      const columns = board.locator('.prose-column');
      await expect(columns).toHaveCount(6);
      const role = columns.filter({ has: page.getByRole('heading', { name: /writer/ }) });
      const input = role.locator('textarea');
      await expect(input).toHaveValue(sample);
      const normal = await textWidth(input);
      const widths = await columns.evaluateAll(nodes => nodes.map(n => n.getBoundingClientRect().width));
      const wrap = await page.locator('.wrap').evaluate(n => n.getBoundingClientRect().width);
      await expect.poll(() => page.locator('.wrap').evaluate(n => n.getBoundingClientRect().left >= 0 && n.getBoundingClientRect().right <= document.documentElement.clientWidth)).toBe(true);
      // Every column keeps the standard reading width, so six documents do not
      // fit and the board itself scrolls (r-e9a2).
      await expect.poll(() => columns.evaluateAll(nodes => nodes.every(n => n.getBoundingClientRect().width >= 560))).toBe(true);
      if (viewport.width === 2560) {
        await expect.poll(async () => (await columns.evaluateAll(nodes => nodes.map(n => n.getBoundingClientRect().width))).every((width, i) => width >= results[0].widths[i])).toBe(true);
        await expect.poll(() => board.evaluate(n => n.scrollWidth > n.clientWidth)).toBe(true);
      }
      // Content must fit at the actual column width, regardless of width mode.
      await expect.poll(() => columns.evaluateAll(nodes => {
        const budgets = nodes.map(node => node.querySelector('.prose-budget')!);
        const top = budgets[0].getBoundingClientRect().top;
        const saves = nodes.map(node => [...node.querySelectorAll('button')].find(button => button.textContent === '保存')!.getBoundingClientRect());
        if (!saves.every(save => save.top === saves[0].top)) return false;
        const feet = nodes.flatMap(node => [...node.querySelectorAll('.prose-actions .btn')].map(button => button.getBoundingClientRect()));
        if (!feet.every(foot => foot.top === feet[0].top && foot.height === feet[0].height)) return false;
        return budgets.every(budget => {
          const box = budget.getBoundingClientRect();
          return box.top === top && box.height === 24 && budget.scrollWidth <= budget.clientWidth
            && [...budget.querySelectorAll('.v, :scope > span:last-child')].every(child => {
              const rect = child.getBoundingClientRect();
              return rect.top >= box.top && rect.bottom <= box.bottom && rect.left >= box.left && rect.right <= box.right;
            });
        }) && nodes.every(node => [...node.querySelectorAll('.prose-actions .btn')].every(button => {
          const box = button.getBoundingClientRect(), column = node.getBoundingClientRect();
          return button.scrollWidth <= button.clientWidth && box.left >= column.left && box.right <= column.right;
        }));
      })).toBe(true);
      // A column at the standard reading width names its actions; the icon-only
      // form belongs to a column narrowed on purpose (r-e9a2).
      if (viewport.width === 1440) {
        for (const column of await columns.filter({ has: page.locator('.prose-actions') }).all()) {
          for (const name of ['ロールの画面で開く', 'ファイルからリロード']) {
            const action = column.getByRole(name === 'ロールの画面で開く' ? 'link' : 'button', { name, exact: true });
            await expect(action).toHaveAttribute('title', name);
            await expect(action.locator('svg')).toBeHidden();
            await expect(action.locator('span')).toBeVisible();
          }
        }
      }
      await columns.first().scrollIntoViewIfNeeded();
      await page.screenshot({ path: info.outputPath(`${viewport.width}-${theme}-normal.png`) });
      await role.getByRole('button', { name: '広く', exact: true }).click();
      const wide = await textWidth(input);
      await expect.poll(async () => (await textWidth(input)).inner).toBeGreaterThan(normal.inner);
      await expect.poll(async () => (await textWidth(input)).chars).toBeGreaterThan(normal.chars);
      const wideWidths = await columns.evaluateAll(nodes => nodes.map(n => n.getBoundingClientRect().width));
      // 広く takes a larger share of the board; it no longer narrows the others
      // below the standard reading width (r-e9a2).
      await expect.poll(() => columns.evaluateAll(nodes => nodes.every(n => n.getBoundingClientRect().width >= 560))).toBe(true);
      await expect.poll(async () => (await role.boundingBox())!.width).toBeGreaterThan(Math.max(...widths));
      for (const column of await columns.all()) {
        await column.scrollIntoViewIfNeeded();
        await expect(column.getByRole('button', { name: '広く', exact: true })).toBeInViewport();
      }
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      const boardWidth = await board.evaluate(n => ({ visible: n.clientWidth, scroll: n.scrollWidth }));
      if (viewport.width === 1440) {
        // Narrowed on purpose, a column shows its actions as icons alone.
        const first = columns.filter({ has: page.locator('.prose-actions') }).first();
        await first.getByRole('button', { name: '狭く（見出しだけにする）', exact: true }).click();
        await expect.poll(async () => (await first.boundingBox())!.width).toBe(240);
        await expect(first.getByRole('link', { name: 'ロールの画面で開く', exact: true }).locator('svg')).toBeVisible();
      }
      await role.scrollIntoViewIfNeeded();
      await page.screenshot({ path: info.outputPath(`${viewport.width}-${theme}-wide.png`) });
      await expect(input).toHaveValue(sample);
      expect(readFileSync(roleFile, 'utf8')).toBe(sample);
      await page.goto(`${app.base}/roles?open=writer|project`); await ready(page);
      const roleInput = page.locator('.role.open textarea').first();
      await expect(roleInput).toHaveValue(sample);
      let roleWidth = await textWidth(roleInput);
      const standard = await textWidth(roleInput);
      expect(standard.chars).toBeGreaterThanOrEqual(35); expect(standard.chars).toBeLessThanOrEqual(45);
      await page.locator('.role.open').getByRole('button', { name: '広く', exact: true }).click();
      await expect.poll(async () => (await textWidth(roleInput)).inner).toBeGreaterThan(832);
      if (results.length) await expect.poll(async () => (await textWidth(roleInput)).inner).toBeGreaterThan(results[0].role.inner);
      await page.screenshot({ path: info.outputPath(`${viewport.width}-${theme}-role.png`) });
      roleWidth = await textWidth(roleInput);
      results.push({ viewport, wrap, widths, normal, wide, wideWidths, board: boardWidth, role: roleWidth });
    }
    const measurements = info.outputPath('width-measurements.json');
    writeFileSync(measurements, JSON.stringify(results, null, 2));
    await info.attach('width-measurements', { path: measurements, contentType: 'application/json' });

  });
}


test('four columns retain aligned editors and save actions at 1440px', async ({ page, app }) => {
  const teamFile = path.join(app.proj, '.hergy/teams/core/team.yaml');
  writeFileSync(teamFile, readFileSync(teamFile, 'utf8').replace('role: analyst', 'role: lead'));
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${app.base}/teams?open=core&agree=core`); await ready(page);
  await page.getByRole('button', { name: 'すべて表示', exact: true }).click();
  const columns = page.locator('.tdef[data-name="core"] .prose-column');
  await expect(columns).toHaveCount(4);
  await expect.poll(() => columns.evaluateAll(nodes => {
    const edges = nodes.map(node => {
      const input = node.querySelector('textarea')!.getBoundingClientRect();
      const save = [...node.querySelectorAll('button')].find(button => button.textContent === '保存')!.getBoundingClientRect();
      return [input.top, input.bottom, save.top, input.height];
    });
    return edges.every(edge => edge[0] === edges[0][0] && edge[1] === edges[0][1] && edge[2] === edges[0][2] && edge[3] >= 320);
  })).toBe(true);
});
