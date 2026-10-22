import { defineConfig } from '@playwright/test';

// The browser walks of checks/*.md's 大 items (r-1c41, d-3c9b). Each test lays
// its own project out and starts its own serve on a port the operating system
// picks (tests/fixtures.ts), so tests may run side by side; the count is the one
// setting that says how many do.
//
// The tools are bun and Playwright (d-bb0d), and both are pinned:
// e2e/package.json's @playwright/test is the version, and `bun` is the one
// tools/check puts in .bin/ (1.4.3), or the one on PATH for a person's own run.
export default defineConfig({
  testDir: './tests',
  globalSetup: './scripts/build.mjs',
  // CI measured two workers and four over the 11 walks (r-1c41): 33.5s of the
  // e2e gate with four, 34.0s with two, both with the cache warm. The difference
  // is small because ACTIVITY-P4 waits about 31s on its own re-reading, which is
  // the floor either way; four is the count that measured faster.
  workers: Number(process.env.E2E_WORKERS || 4),
  fullyParallel: true,
  retries: 0,
  timeout: 5 * 60_000,
  reporter: [['list']],
  use: {
    // The Go walks drove the pages at 2000x1100 (chromedp.EmulateViewport); a
    // walk that reads a narrower window sets its own.
    viewport: { width: 2000, height: 1100 },
    actionTimeout: 8_000,
    trace: 'off',
    video: 'off',
    screenshot: 'off',
  },
  expect: { timeout: 8_000 },
});
