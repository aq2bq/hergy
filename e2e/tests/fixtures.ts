// One project, one serve, one page per test.
//
// The Go walks started a serve for each test (internal/app startServe: `hergy
// serve --no-open` over the test's own project, the URL read from what the
// command prints) and drove it in a tab of the run's one browser. Here each test
// does the same thing itself: it lays its project out, starts its own serve on a
// port the operating system picks, and closes both when it is done — so two
// tests can run at the same time, which the fixed port of the measurement's
// webServer could not (research/bench.rewritten.md「Playwright に移すときに当たった穴」1).
//
// Nothing outside the test's own root is read or written: not the person's
// ~/.config/hergy, not ~/.local/state/hergy (d-a861).
import { test as base, expect, type Page } from '@playwright/test';
import { execFileSync, spawn, type ChildProcess } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, statSync } from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { hergyBin, lay, zone } from '../scripts/fixture.mjs';

// Screen is the screen a walk reads: the fixture each one stands on.
export type Screen = 'nav' | 'profiles' | 'roles' | 'teams' | 'activity';

// App is one test's project: where its parts stand, and the serve over it.
export type App = {
  base: string; // the URL the serve printed
  proj: string;
  cfg: string;
  state: string;
  home: string;
  work: string; // the activity fixture's bin/, herdr/ and gy/
};

// startServe starts `hergy serve --no-open --port 0` over one fixture, and
// returns the URL it printed, the way internal/app's startServe reads it.
async function startServe(fixture: {
  proj: string;
  env: Record<string, string>;
}): Promise<{ url: string; child: ChildProcess }> {
  const child = spawn(hergyBin, ['serve', '--no-open', '--port', '0'], {
    cwd: fixture.proj,
    env: { ...process.env, ...fixture.env },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let said = '';
  const url = await new Promise<string>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`serve printed no URL (${said})`)), 20_000);
    const look = (chunk: Buffer) => {
      said += chunk.toString();
      const found = said.match(/http:\/\/127\.0\.0\.1:[0-9]+/);
      if (found) {
        clearTimeout(timer);
        resolve(found[0]);
      }
    };
    child.stdout?.on('data', look);
    child.stderr?.on('data', look);
    child.on('exit', (code) => {
      clearTimeout(timer);
      reject(new Error(`serve stopped before it answered (${code}): ${said}`));
    });
  });
  return { url, child };
}

// stopServe stops one serve and waits for it to go.
async function stopServe(child: ChildProcess): Promise<void> {
  if (child.exitCode !== null) return;
  const gone = new Promise<void>((resolve) => child.once('exit', () => resolve()));
  child.kill('SIGTERM');
  await Promise.race([gone, new Promise((resolve) => setTimeout(resolve, 10_000))]);
  if (child.exitCode === null) child.kill('SIGKILL');
}

// listen starts listening to what the page writes in red, the way the Go walks
// did (e2eListen): a console error and an uncaught exception both fail the walk
// at its end. It is the protocol's own console events (Runtime.consoleAPICalled
// and Runtime.exceptionThrown), not the browser's security log: a note about the
// page's CSP is the browser talking about the page, not the page writing, and
// the Go walks counted the console's own calls alone.
async function listen(page: Page): Promise<string[]> {
  const session = await page.context().newCDPSession(page);
  const said: string[] = [];
  session.on('Runtime.consoleAPICalled', (event) => {
    if (event.type !== 'error') return;
    said.push(
      event.args
        .map((arg: { value?: unknown; description?: string }) =>
          `${arg.value ?? ''} ${arg.description ?? ''}`.trim(),
        )
        .join(' | '),
    );
  });
  session.on('Runtime.exceptionThrown', (event) => {
    const details = event.exceptionDetails as {
      text?: string;
      exception?: { description?: string };
    };
    said.push(`uncaught: ${details.text ?? ''}${details.exception?.description ? `: ${details.exception.description}` : ''}`);
  });
  await session.send('Runtime.enable');
  return said;
}

// test is the walk every spec file builds on: its own fixture and its own serve,
// closed after the test, and the page held to logging nothing.
export const test = base.extend<{ reads: Screen; app: App }>({
  // The screen a file's walks read; a file says which one it stands on.
  reads: ['teams', { option: true }],
  app: async ({ reads, page }, use) => {
    const fixture = lay(reads);
    const { url, child } = await startServe(fixture);
    const said = await listen(page);
    const app: App = {
      base: url,
      proj: fixture.proj,
      cfg: fixture.cfg,
      state: fixture.state,
      home: fixture.home,
      work: fixture.work,
    };
    await use(app);
    try {
      expect(said, 'the screens wrote to the console').toEqual([]);
    } finally {
      await stopServe(child);
      rmSync(fixture.root, { recursive: true, force: true });
    }
  },
});

// hergyNew is `hergy new <name> --lang ja` run in a project of its own, and the
// files it wrote, read back: the definition's two files, the role documents it
// writes beside them, and its profiles.yaml. A walk holds a save of the team
// screen to these byte for byte (internal/app/e2e_teams_test.go read
// scaffold.Team's own files; this is the command a person runs instead).
export function hergyNew(name: string): {
  teams: Map<string, string>;
  roles: Map<string, string>;
  profiles: string;
} {
  const root = mkdtempSync(path.join(os.tmpdir(), 'hergy-new-'));
  const proj = path.join(root, 'proj');
  for (const dir of [proj, path.join(root, 'cfg'), path.join(root, 'state'), path.join(root, 'home')]) {
    mkdirSync(dir, { recursive: true });
  }
  execFileSync('git', ['init', '-q'], { cwd: proj });
  try {
    execFileSync(hergyBin, ['new', name, '--lang', 'ja'], {
      cwd: proj,
      env: {
        ...process.env,
        XDG_CONFIG_HOME: path.join(root, 'cfg'),
        XDG_STATE_HOME: path.join(root, 'state'),
        HOME: path.join(root, 'home'),
        HERDR_ENV: '',
        HERDR_PANE_ID: '',
        HERGY_HERDR: path.join(root, 'cfg', 'no-herdr'),
        HERGY_GY: path.join(root, 'cfg', 'no-gy'),
        TZ: zone,
      },
      stdio: 'ignore',
    });
    const read = (dir: string) => {
      const out = new Map<string, string>();
      const walk = (at: string, prefix: string) => {
        for (const entry of readdirSync(at)) {
          const full = path.join(at, entry);
          if (statSync(full).isDirectory()) walk(full, path.join(prefix, entry));
          else out.set(path.join(prefix, entry), readFileSync(full, 'utf8'));
        }
      };
      if (statSync(dir, { throwIfNoEntry: false })?.isDirectory()) walk(dir, '');
      return out;
    };
    return {
      teams: read(path.join(proj, '.hergy', 'teams', name)),
      roles: read(path.join(proj, '.hergy', 'roles')),
      profiles: readFileSync(path.join(proj, '.hergy', 'profiles.yaml'), 'utf8'),
    };
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

export { expect };
