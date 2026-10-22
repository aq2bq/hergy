// The one place every test's project, serve and reading stands.
//
// It is internal/app's e2e harness written again for Playwright: newFixture and
// runningHerdr (app_test.go), startServe (activity_scale_test.go), and the waits
// r-3c41 put in e2e_harness_test.go. Every test gets a root of its own under the
// machine's temporary directory, so tests run side by side and none of them
// reads or writes the person's ~/.config/hergy or ~/.local/state/hergy.
import { spawn, execFileSync, type ChildProcess } from 'node:child_process';
import {
  chmodSync,
  cpSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import type { Page } from '@playwright/test';

const here = path.dirname(fileURLToPath(import.meta.url));
export const repo = path.resolve(here, '..', '..');

// The binary the e2e gate builds with -tags e2e. It is the only hergy the tests
// run: the tag is what lets it read HERGY_E2E_NOW (internal/app/e2e_clock.go,
// d-f978).
export const hergyBin = process.env.HERGY_E2E_BIN || path.join(repo, '.bin', 'hergy-e2e');

// The moment the activity fixture is counted back from, and the moment the
// e2e binary's clock starts at (e2eActivityNow / e2eActivityZone, r-f647).
export const activityNow = new Date('2026-10-21T00:30:00+09:00');
const activityNowText = '2026-10-21T00:30:00+09:00';

export type Fixture = {
  root: string;
  proj: string;
  cfg: string;
  state: string;
  home: string;
  env: Record<string, string>;
};

// The fake herdr a project with no team running answers with
// (fixture.runningHerdr(t, "", "")).
const runningHerdr = `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list")     echo '{"result":{"tabs":[]}}' ;;
  "agent list")   echo '{"result":{"agents":[]}}' ;;
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "tab create")   echo '{"result":{"tab":{"tab_id":"t1"},"root_pane":{"pane_id":"p1"}}}' ;;
  "pane split")   echo '{"result":{"pane":{"pane_id":"p2"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`;

// makeFixture lays one project out: the testdata layers the walk stands on, the
// config directory outside the home directory, its own state directory, a
// running mock herdr, and no hergy config anywhere else.
export function makeFixture(kind: 'nav' | 'profiles' | 'roles' | 'teams' | 'activity'): Fixture {
  const root = mkdtempSync(path.join(tmpdir(), 'hergy-e2e-'));
  const proj = path.join(root, 'proj');
  const cfg = path.join(root, 'cfg');
  const state = path.join(root, 'state');
  const home = path.join(root, 'home');
  for (const dir of [proj, cfg, state, home]) mkdirSync(dir, { recursive: true });
  execFileSync('git', ['init', '-q'], { cwd: proj });

  const env: Record<string, string> = {
    XDG_CONFIG_HOME: cfg,
    XDG_STATE_HOME: state,
    HOME: home,
    HERDR_ENV: '',
    HERDR_PANE_ID: '',
    HERGY_HERDR: path.join(cfg, 'no-herdr'),
    HERGY_GY: path.join(cfg, 'no-gy'),
    TZ: 'Asia/Tokyo',
    LANG: 'ja_JP.UTF-8',
    LC_ALL: 'ja_JP.UTF-8',
  };

  const layers: Record<string, string[]> = {
    nav: ['roles-v2', 'nav'],
    profiles: ['profiles-v2'],
    roles: ['roles-v2'],
    teams: ['teams-v2'],
    activity: ['teams-v2'],
  };
  for (const one of layers[kind]) copyLayers(one, proj, cfg);

  if (kind === 'activity') {
    // copyActivityFixture: the fake herdr and gy, the project's overlay and the
    // state files, with @T-<minutes>@ and @PROJECT@ filled in.
    const work = path.join(cfg, 'activity');
    for (const part of ['bin', 'herdr', 'gy']) {
      copyPlaces(path.join(repo, 'testdata', 'activity', part), path.join(work, part), activityNow, proj);
    }
    copyPlaces(path.join(repo, 'testdata', 'activity', 'project'), proj, activityNow, proj);
    copyPlaces(path.join(repo, 'testdata', 'activity', 'state'), state, activityNow, proj);
    env.HERGY_HERDR = path.join(work, 'bin', 'herdr');
    env.HERGY_GY = path.join(work, 'bin', 'gy');
    env.HERGY_E2E_NOW = activityNowText;
  } else {
    const herdr = path.join(cfg, 'herdr');
    writeFileSync(herdr, runningHerdr);
    chmodSync(herdr, 0o755);
    env.HERGY_HERDR = herdr;
  }

  mkdirSync(path.join(cfg, 'hergy'), { recursive: true });
  writeFileSync(path.join(cfg, 'hergy', 'config.yaml'), 'version: 1\nlang: ja\n');

  return { root, proj, cfg, state, home, env };
}

// copyLayers lays one testdata fixture's project and its global settings under
// the work root, as copyProfilesFixture and its siblings do.
function copyLayers(one: string, proj: string, cfg: string): void {
  for (const [from, to] of [
    [path.join('project', '.hergy'), path.join(proj, '.hergy')],
    [path.join('config', 'hergy'), path.join(cfg, 'hergy')],
  ]) {
    const src = path.join(repo, 'testdata', one, from);
    if (!existsSync(src)) continue;
    cpSync(src, to, { recursive: true });
    keepModes(src, to);
  }
}

// copyPlaces copies one tree, filling in @PROJECT@ and @T-<minutes>@ and
// dropping the .tmpl suffix, the way copyActivityPlaces does.
function copyPlaces(from: string, to: string, at: Date, project: string): void {
  mkdirSync(to, { recursive: true });
  for (const entry of readdirSync(from, { withFileTypes: true })) {
    const src = path.join(from, entry.name);
    const target = path.join(to, entry.name.replace(/\.tmpl$/, ''));
    if (entry.isDirectory()) {
      copyPlaces(src, target, at, project);
      continue;
    }
    const mode = statSync(src).mode & 0o777;
    const body = readFileSync(src, 'utf8')
      .replaceAll('@PROJECT@', project)
      .replace(/@T-([0-9]+)@/g, (_, minutes: string) =>
        new Date(at.getTime() - Number(minutes) * 60_000).toISOString(),
      );
    writeFileSync(target, body);
    chmodSync(target, mode);
  }
}

// keepModes gives a copied tree the source's permission bits, so the fake herdr
// and gy stay runnable.
function keepModes(from: string, to: string): void {
  for (const entry of readdirSync(from, { withFileTypes: true })) {
    const src = path.join(from, entry.name);
    const target = path.join(to, entry.name);
    if (entry.isDirectory()) keepModes(src, target);
    else if (existsSync(target)) chmodSync(target, statSync(src).mode & 0o777);
  }
}

export type Serve = { url: string; stop: () => Promise<void> };

// startServe runs the command a person runs — hergy serve --no-open — over the
// test's project, with its own XDG directories and mock herdr, and returns the
// URL it printed. Port 0 lets the operating system pick one, so two serves
// stand side by side (the sharp edge of the move, research/bench.md).
export async function startServe(f: Fixture): Promise<Serve> {
  if (!existsSync(hergyBin)) {
    throw new Error(`the e2e binary is not at ${hergyBin}; build it with: go build -tags e2e -o .bin/hergy-e2e .`);
  }
  const child: ChildProcess = spawn(hergyBin, ['serve', '--no-open', '--port', '0'], {
    cwd: f.proj,
    env: { ...process.env, ...f.env },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  const url = await new Promise<string>((resolve, reject) => {
    let out = '';
    let err = '';
    const timer = setTimeout(() => {
      child.kill('SIGKILL');
      reject(new Error(`serve printed no URL within 20s: ${err || out}`));
    }, 20_000);
    child.stdout!.on('data', (chunk: Buffer) => {
      out += chunk.toString();
      const found = out.match(/http:\/\/[^\s]+/);
      if (found) {
        clearTimeout(timer);
        resolve(found[0]);
      }
    });
    child.stderr!.on('data', (chunk: Buffer) => {
      err += chunk.toString();
    });
    child.on('exit', (code) => {
      clearTimeout(timer);
      reject(new Error(`serve stopped before it answered (${code}): ${err || out}`));
    });
  });
  return {
    url,
    stop: async () => {
      if (child.exitCode !== null) return;
      child.kill('SIGTERM');
      await Promise.race([
        new Promise<void>((done) => child.once('exit', () => done())),
        new Promise<void>((done) => setTimeout(done, 10_000)),
      ]);
      if (child.exitCode === null) child.kill('SIGKILL');
    },
  };
}

// removeRoot wipes a test's work root once its serve has stopped.
export function removeRoot(f: Fixture): void {
  rmSync(f.root, { recursive: true, force: true });
}

// ---- the waits (e2e_harness_test.go, r-3c41) ------------------------------------------

// DEADLINE is how long one act waits for the page to offer it (e2eDeadline).
export const DEADLINE = 8_000;
// STEP is how long it looks away before looking again (e2eStep).
const STEP = 50;

// exp compiles one JS expression into the function Playwright evaluates in the
// page. The expressions are the ones the Go tests wrote; compiling them here
// keeps them written the way the walks read.
function exp<T>(script: string): () => T {
  // eslint-disable-next-line no-new-func
  return new Function(`return (${script});`) as () => T;
}

type Taken<T> = { ok: true; value: T } | { ok: false; why: string };

async function look<T>(page: Page, script: string): Promise<Taken<T>> {
  try {
    const value = await page.evaluate(exp<T>(script));
    if (value === undefined || value === null) return { ok: false, why: 'the expression gave nothing' };
    return { ok: true, value };
  } catch (e) {
    return { ok: false, why: String((e as Error)?.message ?? e) };
  }
}

async function attempt<T>(
  page: Page,
  script: string,
  what: string,
  span = DEADLINE,
  step = STEP,
): Promise<T> {
  const deadline = Date.now() + span;
  let why = '';
  while (Date.now() < deadline) {
    const got = await look<T>(page, script);
    if (got.ok) return got.value;
    why = got.why;
    await page.waitForTimeout(step);
  }
  throw new Error(`${what} (${why || 'the page gave nothing'})`);
}

// ready waits until the document has loaded: a person cannot read or press
// before the page is there.
export async function ready(page: Page): Promise<void> {
  await attempt(page, `document.readyState === 'complete'`, 'the page never finished loading');
}

// text reads one expression as the text the page draws.
export async function text(page: Page, script: string): Promise<string> {
  await ready(page);
  return attempt<string>(page, script, `the page never gave ${script}`);
}

// number reads one expression as the number the page draws.
export async function number(page: Page, script: string): Promise<number> {
  await ready(page);
  return attempt<number>(page, script, `the page never gave ${script}`);
}

// truth reads one expression as the truth the page draws.
export async function truth(page: Page, script: string): Promise<boolean> {
  await ready(page);
  return attempt<boolean>(page, script, `the page never gave ${script}`);
}

// list reads one expression as the list the page draws.
export async function list<T>(page: Page, script: string): Promise<T[]> {
  await ready(page);
  return attempt<T[]>(page, script, `the page never gave ${script}`);
}

// wait waits until one expression is true.
export async function wait(page: Page, script: string, span = DEADLINE, step = STEP): Promise<void> {
  await ready(page);
  const deadline = Date.now() + span;
  let why = '';
  while (Date.now() < deadline) {
    const got = await look<boolean>(page, script);
    if (got.ok && got.value === true) return;
    why = got.ok ? String(got.value) : got.why;
    await page.waitForTimeout(step);
  }
  throw new Error(`never became true: ${script} (${why})`);
}

// findExpr is the JS expression that finds one element. The walks write either
// a plain selector (a.scr[data-screen="roles"], the way the Go helpers took one)
// or a whole expression (...querySelector('.sum')); both stand here.
function findExpr(find: string): string {
  if (find.includes('(') || find.startsWith('document')) return find;
  return `document.querySelector(${JSON.stringify(find)})`;
}

// press presses the element one expression stands for, once the screen offers
// it: the element may not be drawn yet, and the screen may have shut it. An SVG
// element has no click of its own, so the press is a mouse event that bubbles,
// the way a real press does.
export async function press(page: Page, find: string): Promise<void> {
  await ready(page);
  const body = `(function(){var el=${findExpr(find)};if(!el){return 'missing';}` +
    `if(el.disabled){return 'shut';}` +
    `if(typeof el.click==='function'){el.click();}else{el.dispatchEvent(new MouseEvent('click',{bubbles:true,cancelable:true,view:window}));}` +
    `return 'pressed';})()`;
  const deadline = Date.now() + DEADLINE;
  let word = '';
  while (Date.now() < deadline) {
    const got = await look<string>(page, body);
    word = got.ok ? got.value : got.why;
    if (word === 'pressed') return;
    await page.waitForTimeout(STEP);
  }
  throw new Error(word === 'missing' ? `no element for ${find}` : `never became pressable: ${find}`);
}

// setField writes one field's value and fires the input event a person's typing
// fires, once the field is there.
export async function setField(page: Page, find: string, value: string): Promise<void> {
  await write(page, `(function(){var el=${findExpr(find)};if(!el){return 'missing';}` +
    `el.value=${JSON.stringify(value)};el.dispatchEvent(new Event('input',{bubbles:true}));return 'set';})()`, find);
}

// choose picks one option of a select and fires the change event a person's
// picking fires, once the select is there.
export async function choose(page: Page, find: string, value: string): Promise<void> {
  await write(page, `(function(){var el=${findExpr(find)};if(!el){return 'missing';}` +
    `el.value=${JSON.stringify(value)};el.dispatchEvent(new Event('change',{bubbles:true}));return 'picked';})()`, find);
}

async function write(page: Page, body: string, find: string): Promise<void> {
  await ready(page);
  const deadline = Date.now() + DEADLINE;
  let word = '';
  while (Date.now() < deadline) {
    const got = await look<string>(page, body);
    word = got.ok ? got.value : got.why;
    if (word !== 'missing') return;
    await page.waitForTimeout(STEP);
  }
  throw new Error(`no element for ${find}`);
}

// script runs one script in the page for its effect.
export async function script(page: Page, code: string): Promise<void> {
  await ready(page);
  await page.evaluate(exp(code));
}

// waitPath waits until the page's address is one path, and the document has
// loaded. The address alone is not enough: the band points each screen's link at
// the place it was left when the document loads (nav.js 1.4).
export async function waitPath(page: Page, at: string): Promise<void> {
  let loc = '';
  const deadline = Date.now() + DEADLINE;
  while (Date.now() < deadline) {
    const got = await look<string>(page, `location.pathname + location.search`);
    loc = got.ok ? got.value : '';
    if (loc === at) {
      await ready(page);
      return;
    }
    await page.waitForTimeout(STEP);
  }
  throw new Error(`the page never reached ${JSON.stringify(at)} (it is at ${JSON.stringify(loc)})`);
}

// go opens one address of the test's serve.
export async function go(page: Page, url: string): Promise<void> {
  await page.goto(url);
  await ready(page);
}

// waitFor waits until one selector is drawn (chromedp.WaitReady: the element is
// there, not necessarily on screen).
export async function waitFor(page: Page, selector: string): Promise<void> {
  await page.waitForSelector(selector, { state: 'attached', timeout: DEADLINE });
}

// focus puts the cursor in one element, the way chromedp.Focus does.
export async function focus(page: Page, selector: string): Promise<void> {
  await ready(page);
  await page.locator(selector).focus({ timeout: DEADLINE });
}

// theme sets the theme the screens read, the way a person's choice lands in the
// page.
export async function theme(page: Page, value: string): Promise<void> {
  if (value === '') {
    await script(page, 'delete document.documentElement.dataset.theme');
    return;
  }
  await script(page, `document.documentElement.dataset.theme=${JSON.stringify(value)}`);
}

// watch listens to what the page says in red while a test drives it. The Go
// harness listened to runtime.ConsoleAPICalled, which does not carry a Content
// Security Policy report; Playwright's console event does, so those are left
// out here too — the walk is about what the screen logs, not about the browser
// refusing a style the page itself wrote.
export function watch(page: Page, exceptions: boolean): string[] {
  const lines: string[] = [];
  page.on('console', (message) => {
    if (message.type() !== 'error') return;
    if (message.text().includes('Content Security Policy')) return;
    lines.push(message.text());
  });
  if (exceptions) page.on('pageerror', (error) => lines.push(`uncaught: ${error.message}`));
  return lines;
}

// readFile is a test's read of one file its walk wrote.
export function readFile(at: string): string {
  return readFileSync(at, 'utf8');
}

// changedLines is the 1-based numbers of the lines that differ between two
// texts, with the line each became (e2eRolesChangedLines).
export function changedLines(before: string, after: string): Map<number, string> {
  const b = before.split('\n');
  const a = after.split('\n');
  const out = new Map<number, string>();
  const n = Math.max(a.length, b.length);
  for (let i = 0; i < n; i++) {
    const l = i < b.length ? b[i] : '';
    const r = i < a.length ? a[i] : '';
    if (l !== r) out.set(i + 1, r);
  }
  return out;
}
