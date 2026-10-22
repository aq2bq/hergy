import { defineConfig } from '@playwright/test';

// Every test brings its own project and its own serve (tests/harness.ts), so
// the tests stand on nothing another test changes and can run side by side.
// Playwright's own webServer is not used: it starts one serve before the run
// and has to know its address in advance, while the serve here picks its port
// from the operating system and prints it (hergy serve --port 0).
export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  // The baseline measured in the move (d-d922): two workers. E2E_WORKERS lets
  // the measurement try another number without a commit.
  workers: Number(process.env.E2E_WORKERS || 4),
  retries: 0,
  timeout: 5 * 60_000,
  expect: { timeout: 8_000 },
  reporter: [['list']],
  use: {
    viewport: { width: 2000, height: 1100 },
    // The screens read their clocks in the zone the serve is given (r-f647);
    // the browser stands in the same zone, so a reading made by the page and
    // one made by the server agree.
    timezoneId: 'Asia/Tokyo',
    trace: 'off',
    video: 'off',
    screenshot: 'off',
  },
});
