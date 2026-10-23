// SHOWCASE-S1 and SHOWCASE-R1 — the showcase's screens stay the real screens.
// The showcase (tools/showcase) is started here the way a person starts it, and
// every screen is opened and counted against the sample data
// (testdata/showcase): six rows for seven definitions (docs stands in both
// places), core's five members and six workers, eleven profiles, six roles, and
// the files their YAML tabs list. A screen that moved, or sample data that
// drifted from what the screens say, fails here.
//
// The two scenes are the ones the brief keeps: standard (nothing running) and
// running (core is running in the made-up herdr). The walk reads only what the
// browser alone can see; the counts are read again until they match (r-291a).
import { test as base, expect, type Page } from '@playwright/test';
import { execFileSync, spawn, spawnSync, type ChildProcess } from 'node:child_process';
import {
  mkdirSync,
  mkdtempSync,
  realpathSync,
  renameSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { hergyBin, repo } from '../scripts/fixture.mjs';
import { press, q, readTo, there, wait } from './helpers';

// scene is one state of the screens the showcase lays out.
type Scene = 'standard' | 'running';

// App is one scene's screens: the URL the tool printed, its work directory, and
// the tool itself.
type App = { base: string; work: string; tool: ChildProcess };

// bin is where the e2e run keeps what it builds, and tool is the showcase
// built there once per walker.
const bin = path.join(repo, 'e2e', '.bin');
const tool = path.join(bin, 'showcase');

// buildTool builds `go run ./tools/showcase` once, the way a person runs it.
function buildTool(): void {
  mkdirSync(bin, { recursive: true });
  execFileSync('go', ['build', '-o', tool, './tools/showcase'], { cwd: repo, stdio: 'inherit' });
}

// start lays one scene out with the tool and returns the URL it printed. The
// work directory is this walk's own, so two scenes may run side by side. mock,
// when set, is a prototype's directory for --mock. built false lets the tool
// build its own bin/hergy, which the run road rebuilds.
async function start(scene: Scene, mock: string, built: boolean): Promise<App> {
  buildTool();
  const work = mkdtempSync(path.join(os.tmpdir(), `hergy-showcase-${scene}-`));
  const args = [scene, '--port', '0', '--work', work];
  if (built) args.push('--bin', hergyBin);
  if (mock) args.push('--mock', mock);
  const child = spawn(tool, args, {
    env: { ...process.env },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let said = '';
  const base = await new Promise<string>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`showcase printed no URL (${said})`)), 30_000);
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
      reject(new Error(`showcase stopped before it answered (${code}): ${said}`));
    });
  });
  return { base, work, tool: child };
}

// stop tells the tool to stop, which stops the serve it started, and clears the
// work directory it laid out.
async function stop(app: App): Promise<void> {
  if (app.tool.exitCode === null) {
    const gone = new Promise<void>((resolve) => app.tool.once('exit', () => resolve()));
    app.tool.kill('SIGTERM');
    await Promise.race([gone, new Promise((resolve) => setTimeout(resolve, 15_000))]);
    if (app.tool.exitCode === null) app.tool.kill('SIGKILL');
  }
  rmSync(app.work, { recursive: true, force: true });
}

// knownCSP is the one console error the screens carry and this walk does not
// fix: n-19e4 — internal/web/static/teams-v2.js:547 writes the 「＋ ワーカー」
// button with a style attribute, and the serve's Content-Security-Policy
// (internal/web/server.go) refuses it. The walk opens no topology point, so it
// does not stand today; naming it here keeps the day it does from reading as a
// new break. Fixing n-19e4 takes this line away.
const knownCSP = 'Refused to apply inline style';

// listen is what the page wrote in red: a console error and an uncaught
// exception both fail the walk at its end (the protocol's own console events,
// as fixtures.ts reads them).
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
    const at = details.exception?.description ?? '';
    said.push(`uncaught: ${details.text ?? ''}${at ? `: ${at}` : ''}`);
  });
  await session.send('Runtime.enable');
  return said;
}

// test is one scene's screens, started for the walk and stopped after it, with
// the console held to the one known break.
const test = base.extend<{ scene: Scene; mock: string; built: boolean; app: App }>({
  scene: ['standard', { option: true }],
  mock: ['', { option: true }],
  built: [true, { option: true }],
  app: async ({ scene, mock, built, page }, use) => {
    const app = await start(scene, mock, built);
    const said = await listen(page);
    await use(app);
    try {
      expect(
        said.filter((one) => !one.includes(knownCSP)),
        'the screens wrote to the console',
      ).toEqual([]);
    } finally {
      await stop(app);
    }
  },
});

// plan is one hergy command run the way the tool runs it for a person
// (`showcase run`), over the walk's own work directory: hergy's JSON answer.
type Plan = { members: { name: string; cwd: string; prompt: string }[] };

function plan(app: App, scene: Scene, args: string[]): Plan {
  const said = execFileSync(
    tool,
    ['run', scene, '--work', app.work, '--bin', hergyBin, '--', ...args],
    { encoding: 'utf8' },
  );
  return JSON.parse(said) as Plan;
}

// count is one screen's number of elements.
const count = (selector: string) => `document.querySelectorAll(${JSON.stringify(selector)}).length`;

// open opens one address and waits for what the screen draws there.
async function open(page: Page, url: string, drawn: string): Promise<void> {
  await page.goto(url);
  await wait(page, there(drawn));
}

// crew is one activity row's members and workers: the people with a state word,
// split by the mark a hired worker carries (activity-v2.js).
const crew = (name: string) => {
  const row = q(`#list .act[data-team="${name}"]`);
  return (
    `(function(){var a=${row};if(!a)return 'none';` +
    `return a.querySelectorAll('.crew .nodes .nm:not(.wk)').length+'|'+a.querySelectorAll('.crew .nodes .nm.wk').length})()`
  );
};

// walkTeams opens the team definitions: the list, every row's two tabs, and the
// YAML tab. Seven definitions stand in two places (docs in both), so the list
// holds six rows and seven place pills; core is the sample's largest team.
async function walkTeams(page: Page, base: string): Promise<void> {
  await open(page, `${base}/teams`, '#view .tdef');
  await readTo<number>(page, count('#view .tdef'), 'the team definitions read, want the sample data\'s six rows').toBe(6);
  await readTo<number>(
    page,
    count('#view .tdef .c-place .pill'),
    'the places the definitions stand in read, want seven (four in the project, three shared)',
  ).toBe(7);
  const core = '.tdef[data-name="core"] .sum';
  await readTo<string>(
    page,
    `[${q(`${core} .c-members`)}.textContent,${q(`${core} .c-turns`)}.textContent].join('|')`,
    "core's members and workers read, want the sample data's five and six",
  ).toBe('5|6');

  for (const name of ['core', 'docs', 'draft', 'migrate', 'research', 'review']) {
    const row = `.tdef[data-name="${name}"]`;
    await open(page, `${base}/teams?open=${name}`, `${row} .panel.team-p`);
    await press(page, q(`${row} .dtabs .dtab[data-t=agree]`));
    await wait(page, there(`${row} .panel.agree-p`));
    await readTo<number>(page, count(`${row} .panel.agree-p`), `${name}'s rules tab is drawn, want one`).toBe(1);
  }

  await open(page, `${base}/teams/yaml`, '#view .ylay');
  await readTo<number>(
    page,
    count('#view .file'),
    'the files the YAML tab lists read, want the sample data\'s fourteen (seven definitions in two places)',
  ).toBe(14);
}

// walkProfiles opens the profile screen's three tabs. The sample data's eight
// project profiles and four shared ones make eleven names (sonnet-fast stands in
// both) and two files.
async function walkProfiles(page: Page, base: string): Promise<void> {
  await open(page, `${base}/profiles`, '#rows .row');
  await readTo<number>(page, count('#rows .row'), 'the profiles read, want eleven').toBe(11);
  await open(page, `${base}/profiles/assign`, '.arow');
  await readTo<number>(page, count('.team'), 'the teams the assign tab reads, want six').toBe(6);
  await open(page, `${base}/profiles/yaml`, '.yed');
  await readTo<number>(page, count('.yed'), 'the profiles.yaml files read, want two').toBe(2);
}

// walkRoles opens the role screen's three tabs: four project roles and three
// shared ones make six names (scout stands in both).
async function walkRoles(page: Page, base: string): Promise<void> {
  await open(page, `${base}/roles`, '#rows .role');
  await readTo<number>(page, count('#rows .role'), 'the roles read, want six').toBe(6);
  await open(page, `${base}/roles/assign`, '.arow');
  await readTo<number>(page, count('.team'), 'the teams the assign tab reads, want six').toBe(6);
  await open(page, `${base}/roles/yaml`, '.file');
  await readTo<number>(page, count('.file'), 'the role documents read, want seven').toBe(7);
}

// standard is the showcase with nothing running: the front page is the team
// definitions, and the activity screen says no team is running.
test('SHOWCASE-S1 the standard showcase holds the sample data', async ({ page, app }) => {
  await open(page, `${app.base}/`, '#view .tdef');
  await readTo<string>(page, 'location.pathname', 'the front page with nothing running, want /teams').toBe('/teams');

  await open(page, `${app.base}/activity`, '#list .gone');
  await readTo<number>(page, count('#list .act'), 'the activity screen with nothing running, want no row').toBe(0);

  await walkTeams(page, app.base);
  await walkProfiles(page, app.base);
  await walkRoles(page, app.base);

  // The run road stands on the serve's own environment (tools/showcase run):
  // the project a prompt names is this scene's project and the hire command
  // names the binary the serve runs, so a prompt read here is the prompt the
  // screen would show. The sample's two rewritten templates are read the same
  // way: research's team template reaches both its members, and docs' member
  // template stays with docs-writer.
  const shop = realpathSync(path.join(app.work, 'shop'));
  const core = plan(app, 'standard', ['up', 'core', '--dry', '--json']);
  const prompts = core.members.map((one) => one.prompt).join('\n');
  expect(core.members[0]?.cwd, "the run road names the serve's own project").toBe(shop);
  expect(prompts, "the hire command names the serve's own binary").toContain(`${hergyBin} hire`);
  expect(prompts, "the hire command names the serve's own project").toContain(`-C ${shop}`);

  const research = plan(app, 'standard', ['up', 'research', '--dry', '--json']);
  expect(
    research.members.every((one) => one.prompt.includes('調べ始める前に、問いを 1 文で書き')),
    "the research team's own template does not reach every member",
  ).toBe(true);
  const docs = plan(app, 'standard', ['up', 'docs', '--dry', '--json']);
  const writer = docs.members.find((one) => one.name === 'docs-writer')?.prompt ?? '';
  const lead = docs.members.find((one) => one.name === 'docs-lead')?.prompt ?? '';
  expect(writer, "the sample's member template does not reach its member").toContain(
    '書き始める前に、読み手を 1 人決め',
  );
  expect(lead, "the member template did not stay with its member").not.toContain(
    '書き始める前に、読み手を 1 人決め',
  );
});

// mock puts a prototype's files into the real screens: the sample's one script
// adds an element, and the screen itself is still the app's own (six rows).
test.describe('mock', () => {
  test.use({
    scene: 'standard',
    mock: path.join(repo, 'testdata', 'showcase', 'mock', 'sample'),
  });

  test('SHOWCASE-M1 the mock files stand in the real screen', async ({ page, app }) => {
    await open(page, `${app.base}/teams`, '#view .tdef');
    await readTo<string>(
      page,
      `document.querySelector('.mock-mark')?.textContent ?? ''`,
      "the mock's element does not stand in the real screen",
    ).toBe('mock が差し込んだ行');
    await readTo<number>(page, count('#view .tdef'), 'the screen under the mock reads, want six rows').toBe(6);
  });
});

// Two serves must not share one work directory: the second stops before it
// lays anything out, so the first keeps answering and its data is not copied
// again under it (2026-10-23, the owner's running showcase was rewritten under
// them). It asks for --work instead.
test.describe('two serves', () => {
  test.use({ scene: 'running', mock: path.join(repo, 'testdata', 'showcase', 'mock', 'sample') });

  test('SHOWCASE-L1 a second serve on a work directory in use stops', async ({ page, app }) => {
    const data = path.join(app.work, 'shop', '.hergy', 'teams', 'core', 'team.yaml');
    const before = statSync(data).mtimeMs;

    const second = spawnSync(
      tool,
      [
        'running',
        '--port',
        '0',
        '--work',
        app.work,
        '--bin',
        hergyBin,
        '--mock',
        path.join(repo, 'testdata', 'showcase', 'mock', 'sample'),
      ],
      { encoding: 'utf8' },
    );
    expect(second.status, second.stderr).toBe(1);
    expect(second.stderr, 'the second serve does not say the directory is in use').toContain(
      '同じ作業用ディレクトリ',
    );
    expect(second.stderr, 'the second serve does not ask for --work').toContain('--work');

    expect(
      statSync(data).mtimeMs,
      "the second serve laid the first one's data out again",
    ).toBe(before);
    await open(page, `${app.base}/teams`, '#view .tdef');
    await readTo<number>(
      page,
      count('#view .tdef'),
      'the first serve under the second one reads, want six rows',
    ).toBe(6);
  });
});

// The settings outside the project stand in a home of the scene's own, where a
// person's stand (2026-10-23): the role screen's 共通 panel reads
// ~/.config/hergy/roles/scout.yaml, not a path under the work directory. A
// person's machine never shows the absolute path, so the screens must not.
test.describe('outside settings', () => {
  test.use({ scene: 'running' });

  test('SHOWCASE-X1 the outside settings read from home', async ({ page, app }) => {
    await open(page, `${app.base}/roles?open=scout`, '.role.open .panel');
    const paths = `[...document.querySelectorAll('.role.open .panel .ph .path')].map(function(e){return e.textContent.trim()}).join('|')`;
    await readTo<string>(
      page,
      paths,
      'the role screen does not read the outside settings as ~/.config/hergy',
    ).toContain('~/.config/hergy/roles/scout.yaml');
    await readTo<boolean>(
      page,
      `(${paths}).indexOf(${JSON.stringify(app.work)}) >= 0`,
      'the role screen reads a path under the work directory',
    ).toBe(false);
  });
});

// The command roads stand on the code as it is now, not on the binary an
// earlier run left: a work directory without --bin has its bin/hergy rebuilt on
// every run, through a rename, and the serve that was already up stays on the
// file it started with. The walk puts an old binary where the new one goes (by
// rename, the way the tool does), so the change is real; the run says so on
// stderr once, the command's own answer stays on stdout, and the serve is still
// there afterwards.
test.describe('rebuild', () => {
  test.use({ scene: 'standard', built: false });

  test('SHOWCASE-B1 run rebuilds bin/hergy when the code changed', async ({ page, app }) => {
    const bin = path.join(app.work, 'bin', 'hergy');
    const old = path.join(app.work, 'bin', '.old-hergy');
    writeFileSync(old, '#!/bin/sh\necho "old hergy"\n', { mode: 0o755 });
    renameSync(old, bin);

    const done = spawnSync(tool, ['run', 'standard', '--work', app.work, '--', 'version'], {
      encoding: 'utf8',
    });
    expect(done.status, done.stderr).toBe(0);
    expect(done.stderr, 'the rebuild was not said on stderr').toContain('bin/hergy を作り直しました');
    expect(done.stdout, "the command's own answer is not on stdout").toMatch(/^hergy /);

    // The serve started before the rebuild still answers: it is the old file.
    await open(page, `${app.base}/teams`, '#view .tdef');
    await readTo<number>(page, count('#view .tdef'), 'the serve under the rebuild reads, want six rows').toBe(6);
  });
});

// running is the same sample data with core running in the made-up herdr: its
// five members carry four of herdr's states (working, idle, blocked, done, the
// last two both read 止まっている) and two workers are working.
test.describe('running', () => {
  test.use({ scene: 'running' });

  test('SHOWCASE-R1 the running showcase holds the sample data', async ({ page, app }) => {
    await open(page, `${app.base}/`, '#list .act');
    await readTo<string>(page, 'location.pathname', 'the front page with a team running, want /activity').toBe('/activity');
    await readTo<number>(page, count('#list .act'), 'the activity screen reads, want the sample data\'s one running team').toBe(1);
    await readTo<string>(
      page,
      `document.querySelector('#list .act[data-team="core"] .c-st .st').textContent`,
      'core\'s state reads, want 許可待ち (its reviewer is blocked)',
    ).toBe('許可待ち');

    await wait(page, there('#list .act[data-team="core"] .crew'));
    await readTo<string>(
      page,
      crew('core'),
      "core's crew reads, want the sample data's five members and two workers",
    ).toBe('5|2');
    await readTo<string>(
      page,
      `[...new Set([...document.querySelectorAll('#list .act[data-team="core"] .crew .nodes .sw')].map(function(n){return n.textContent}))].sort().join(',')`,
      'the states core\'s crew shows read, want 仕事中・止まっている・許可待ち',
    ).toBe('仕事中,止まっている,許可待ち');

    await walkTeams(page, app.base);
    await walkProfiles(page, app.base);
    await walkRoles(page, app.base);

    // The made-up herdr is the one a command sees: hire --dry reads herdr to
    // find the running team, and this scene's herdr answers for core.
    const hire = plan(app, 'running', [
      'hire', 'load-1', '--purpose', '負荷の試験', '--from', 'reviewer',
      '--task', '5 分の負荷', '--team', 'core', '--dry', '--json',
    ]);
    expect(hire.members[0]?.name, 'the hire the running scene answers reads').toBe('core-load-1');
  });
});
