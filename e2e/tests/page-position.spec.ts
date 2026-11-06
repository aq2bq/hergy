import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';
import { ready } from './helpers';

// r-6ff8 / ac-765d: what moves the page, and what only switches the display.
// Each walk sets the page where the act stands, then compares window.scrollY
// before and after. "stay" is a difference of 0; a move puts its destination
// into view (a destination already in view may leave the place alone).
const place = (page: Page) => page.evaluate(() => window.scrollY);

test.describe('teams', () => {
  test.use({ reads: 'teams' });

  test('PAGE-POSITION-T1 opening a row keeps the place', async ({ page, app }) => {
    await page.setViewportSize({ width: 1200, height: 700 });
    await page.goto(`${app.base}/teams`);
    await ready(page);
    const row = page.locator('.tdef[data-name="core"]');
    await row.locator('.sum').scrollIntoViewIfNeeded();
    const before = await place(page);
    expect(before).toBeGreaterThanOrEqual(0);
    await row.locator('.sum').click();
    await expect(row.locator('.panel').first()).toBeVisible();
    await expect.poll(() => place(page)).toBe(before);
  });

  test('PAGE-POSITION-T2 editing the selected attributes brings the panel into view', async ({ page, app }) => {
    await page.setViewportSize({ width: 1000, height: 700 });
    await page.goto(`${app.base}/teams?open=core`);
    await ready(page);
    const row = page.locator('.tdef[data-name="core"]');
    const canvas = row.locator('.panel.team-p');
    await canvas.locator('.cv-n.member[data-cv-n=lead]').evaluate((el) => el.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true })));
    await expect(canvas.locator('.cv-side [data-cv-f=m-name]')).toBeVisible();
    const panel = canvas.locator('.cv-side');
    await expect(canvas.locator('.cv-edit-selection')).toBeVisible();
    await canvas.locator('.cv-edit-selection').click();
    await expect(panel).toBeInViewport();
  });
});

test.describe('roles', () => {
  test.use({ reads: 'roles' });

  test('PAGE-POSITION-R1 opening a row keeps the place', async ({ page, app }) => {
    await page.setViewportSize({ width: 1200, height: 700 });
    await page.goto(`${app.base}/roles`);
    await ready(page);
    const row = page.locator('.role').nth(2);
    await row.locator('.sum').scrollIntoViewIfNeeded();
    const before = await place(page);
    await row.locator('.sum').click();
    await expect(row.locator('.detail')).toBeVisible();
    await expect.poll(() => place(page)).toBe(before);
  });
});

test.describe('profiles', () => {
  test.use({ reads: 'profiles' });

  test('PAGE-POSITION-P1 opening a row keeps the place', async ({ page, app }) => {
    await page.setViewportSize({ width: 1200, height: 700 });
    await page.goto(`${app.base}/profiles`);
    await ready(page);
    const row = page.locator('.row[data-name="opencode-local"]').first();
    await row.locator('.sum').scrollIntoViewIfNeeded();
    const before = await place(page);
    await row.locator('.sum').click();
    await expect(row.locator('.detail')).toBeVisible();
    await expect.poll(() => place(page)).toBe(before);
  });

  test('PAGE-POSITION-P2 a row the address names is brought into view', async ({ page, app }) => {
    await page.setViewportSize({ width: 1200, height: 400 });
    await page.goto(`${app.base}/profiles?open=opencode-local`);
    await ready(page);
    const row = page.locator('.row[data-name="opencode-local"]').first();
    await expect(row.locator('.detail')).toBeVisible();
    await expect(row).toBeInViewport();
    await expect.poll(() => place(page)).toBeGreaterThan(0);
  });

  test('PAGE-POSITION-P3 a person the usage chip names is brought into view', async ({ page, app }) => {
    await page.setViewportSize({ width: 1200, height: 400 });
    await page.goto(`${app.base}/profiles?open=opus-plan`);
    await ready(page);
    await page.locator('.row[data-name="opus-plan"][data-group="global"] .uchip').first().click();
    const person = page.locator('.team[data-g=core] .arow[data-k="m-0"]');
    await expect(person).toHaveClass(/(^|\s)flash(\s|$)/);
    await expect(person).toBeInViewport();
    await expect.poll(() => place(page)).toBeGreaterThan(0);
  });
});

test.describe('activity', () => {
  test.use({ reads: 'activity' });

  test('PAGE-POSITION-A1 a waiting row brings its team into view', async ({ page, app }) => {
    await page.setViewportSize({ width: 1200, height: 400 });
    await page.goto(`${app.base}/activity`);
    await ready(page);
    const waiting = page.locator('.hq-att .hq-at[data-pick]').first();
    await expect(waiting).toBeVisible();
    const team = await waiting.getAttribute('data-pick');
    await page.evaluate(() => window.scrollTo(0, 0));
    await waiting.click();
    await expect(page.locator(`.hq-tile[data-team="${team}"]`)).toBeInViewport();
    await expect.poll(() => place(page)).toBeGreaterThan(0);
  });

  test('PAGE-POSITION-A2 the team the address names is brought into view', async ({ page, app }) => {
    await page.setViewportSize({ width: 1200, height: 400 });
    await page.goto(`${app.base}/activity?open=core`);
    await ready(page);
    await expect(page.locator('.hq-tile[data-team="core"]')).toBeInViewport();
    await expect.poll(() => place(page)).toBeGreaterThan(0);
  });
});
