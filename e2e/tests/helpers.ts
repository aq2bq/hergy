// The waits every act in these walks goes through, and the words the screen says
// about time: internal/app's e2e harness (e2e_harness_test.go: e2eReady, e2eRead,
// e2eWait, e2ePressWhenEnabled, r-3c41) and internal/activity's text.go, written
// again for the browser.
//
// A walk drives a real browser over real screens, so it is a little ahead of the
// person: it can read an element that is not drawn yet, press one before the page
// has loaded, and press a button the screen has shut. A person cannot do any of
// those — reading a missing element throws in the page, and pressing a shut
// button is no press at all (HTML) — so every act here waits for the page to
// offer it, then acts once.
import { expect, type Page } from '@playwright/test';
import { clockStart } from '../scripts/fixture.mjs';

// DEADLINE is how long one act waits for the page to offer it (e2eDeadline).
export const DEADLINE = 8 * 1000;
// STEP is how long it looks away before looking again (e2eStep).
const STEP = 50;

// q is one selector as the page's own `document.querySelector` finds it: the
// expression the walks name an element with.
export const q = (selector: string) => `document.querySelector(${JSON.stringify(selector)})`;
// there is whether one element stands on the page: the reading every wait for a
// drawn element goes through (the Go walks' `!!document.querySelector`).
export const there = (selector: string) => `!!${q(selector)}`;

// ready waits until the document has loaded, and until a screen that reads more
// after its first frame says it has settled (window.hergyPage.settled): a person
// cannot read or press before the page is there, and an answer that comes after
// the first press draws the screen again under it — the teams screen's roles
// came after ＋メンバー, took the cursor out of the name and left the new member
// with no role (TEAMS-C1, main ba91188).
export async function ready(page: Page): Promise<void> {
  const deadline = Date.now() + DEADLINE;
  const loaded = `document.readyState==='complete'&&!(window.hergyPage&&window.hergyPage.settled===false)`;
  while (Date.now() < deadline) {
    try {
      if ((await page.evaluate(loaded)) === true) return;
    } catch {
      // A page being replaced has no document to ask yet.
    }
    await page.waitForTimeout(STEP);
  }
  throw new Error('the page never finished loading');
}

// ask asks the page one expression, once, and returns what it made of it: the
// value, or why it has none yet. An expression that throws is one of those
// answers — an element that is not drawn yet throws when it is read — so the
// throw is what a caller waits past, not what ends the walk (e2eAsk). The
// driver's own failure is the same kind of "not yet": a page that is being
// replaced has no execution context to read (r-3c41).
async function ask(page: Page, script: string): Promise<{ value: unknown; why: string }> {
  const wrapping =
    `(function(){try{return JSON.stringify({v:(${script})});}` +
    `catch(e){return JSON.stringify({e:String((e&&e.message)||e)});}})()`;
  let text: unknown;
  try {
    text = await page.evaluate(wrapping);
  } catch (e) {
    return { value: undefined, why: (e as Error).message };
  }
  const answer = JSON.parse(text as string) as { v?: unknown; e?: string };
  if (answer.v === undefined && !answer.e) return { value: undefined, why: 'the expression gave nothing' };
  return { value: answer.v, why: answer.e ?? '' };
}

// read waits until the page can give one expression, and returns it. The
// expression is tried again while the page cannot answer it yet, and what the
// page kept saying is the failure's last word.
export async function read<T>(page: Page, script: string): Promise<T> {
  await ready(page);
  const deadline = Date.now() + DEADLINE;
  let why = '';
  while (Date.now() < deadline) {
    const answer = await ask(page, script);
    if (answer.value !== undefined) return answer.value as T;
    why = answer.why;
    await page.waitForTimeout(STEP);
  }
  throw new Error(`the page never gave ${script} (${why || 'the page gave nothing'})`);
}

// readTo is the read a comparison goes through: the expression is read again
// until it is what the walk expects of it, and the walk's own words are the
// expectation's message, as an expect's second argument was.
//
// A walk that reads once and compares what it got asks the page a question it
// may not be ready to answer: the app adds some marks after the press answers —
// a role document's problem is drawn a beat later, so the members in problem
// read "" the first time (TEAMS-V1, main 4f0ec6d, r-291a) — and the walk then
// fails on the run where the machine is a little slower. So no walk compares
// what `read` gave once: it compares a reading that is read again until it
// matches (the gate in tools/check refuses the other shape).
//
// Two waits stand in it, each bounded by DEADLINE: read's own wait for the page
// to give the expression at all (an element that is not drawn throws in the
// page), and the poll's wait for that value to be the one the walk expects. The
// value that came last is what Playwright prints under the walk's words, so a
// failure still says what the page read.
export function readTo<T>(page: Page, script: string, said: string) {
  return expect.poll(() => read<T>(page, script), { timeout: DEADLINE, message: said });
}

// wait waits until one expression is true.
export async function wait(page: Page, script: string): Promise<void> {
  await ready(page);
  const deadline = Date.now() + DEADLINE;
  let why = '';
  while (Date.now() < deadline) {
    const answer = await ask(page, script);
    if (answer.value === true) return;
    why = answer.why;
    await page.waitForTimeout(STEP);
  }
  throw new Error(`never became true: ${script}${why ? ` (${why})` : ''}`);
}

// attempt waits until one script answers with the word it was waiting for, and
// returns the last word the page said.
async function attempt(page: Page, what: string, script: string, want: string): Promise<string> {
  const deadline = Date.now() + DEADLINE;
  let word = '';
  while (Date.now() < deadline) {
    word = (await ask(page, script)).value as string;
    if (word === want) return word;
    await page.waitForTimeout(STEP);
  }
  if (word === 'missing') throw new Error(`no element for ${what}`);
  throw new Error(`never became ${want}: ${what}`);
}

// press presses one element the way a person does, once the screen offers it:
// the element may not be drawn yet, and the screen may have shut it. The element
// is found by the script the walk names it with, and the press is the element's
// own click — or, for a point or a line of the teams figure, a mouse event that
// bubbles to the panel's one listener, because an SVG element has no click() of
// its own (internal/app/e2e_teams_test.go e2eTeamsClick).
export async function press(page: Page, find: string): Promise<void> {
  await ready(page);
  const script =
    `(function(){var el=${find};if(!el){return 'missing';}` +
    `if(el.disabled){return 'shut';}` +
    `if(typeof el.click==='function'){el.click();}else{el.dispatchEvent(new MouseEvent('click',{bubbles:true,cancelable:true,view:window}));}` +
    `return 'pressed';})()`;
  await attempt(page, find, script, 'pressed');
}

// set writes one field's value and fires the input event a person's typing
// fires, once the field is there.
export async function set(page: Page, find: string, value: string): Promise<void> {
  await ready(page);
  const script =
    `(function(){var el=${find};if(!el){return 'missing';}` +
    `el.value=${JSON.stringify(value)};el.dispatchEvent(new Event('input',{bubbles:true}));return 'set';})()`;
  await attempt(page, find, script, 'set');
}

// choose picks one option of a select and fires the change event a person's
// picking fires, once the select is there.
export async function choose(page: Page, find: string, value: string): Promise<void> {
  await ready(page);
  const script =
    `(function(){var el=${find};if(!el){return 'missing';}` +
    `el.value=${JSON.stringify(value)};el.dispatchEvent(new Event('change',{bubbles:true}));return 'picked';})()`;
  await attempt(page, find, script, 'picked');
}

// run runs one script in the page for its effect.
export async function run(page: Page, script: string): Promise<void> {
  await ready(page);
  await page.evaluate(`(function(){${script}})()`);
}

// waitPath waits until the page's address ends at one path, and the document has
// loaded. The address alone is not enough: the band points each screen's link at
// the place it was left when the document loads (nav.js 1.4), so a reading or a
// press before that acts on the addresses the server wrote.
export async function waitPath(page: Page, path: string): Promise<void> {
  const deadline = Date.now() + DEADLINE;
  let at = '';
  while (Date.now() < deadline) {
    at = (await ask(page, 'location.pathname + location.search')).value as string;
    if (at === path) {
      await ready(page);
      return;
    }
    await page.waitForTimeout(STEP);
  }
  throw new Error(`the page never reached ${path} (it is at ${at})`);
}

// viewport sets the window the walk reads the screen at.
export async function viewport(page: Page, width: number, height: number): Promise<void> {
  await page.setViewportSize({ width, height });
}

// ---- the words the server says about time ----------------------------------------------

// now is the moment the run's screens are read at: the moment the serve's clock
// began (d-f978). The lengths a row shows are counted from the serve's own
// clock, which is a few seconds further on by the time the page is read, so the
// candidates below carry a minute's drift — the same minute the Go walks allowed
// (activityLengths).
export const now = new Date(clockStart);

// clock is one moment as the screen writes it (activity.Clock): today reads
// "15:04", another day reads "1/2 15:04".
export function clock(moment: Date, at: Date = now): string {
  const m = parts(moment);
  const n = parts(at);
  const hm = `${m.hour}:${m.minute}`;
  if (m.year === n.year && m.day === n.day && m.month === n.month) return hm;
  return `${Number(m.month)}/${Number(m.day)} ${hm}`;
}

// sinceAt is when a state began, as the row reads it (activity.SinceAt).
export function sinceAt(moment: Date, lower: boolean, at: Date = now): string {
  return lower ? `${clock(moment, at)} より前` : `${clock(moment, at)} から`;
}

// length is how long a state has lasted, as a person reads it (activity.Length).
export function length(ms: number, lower: boolean): string {
  return markLower(durationText(ms), lower);
}

// lengths is what a row may write for a state that began at one moment: the
// words for that length and for a minute more, because the screen is read a
// moment after the fixture was counted from (activityLengths).
export function lengths(ms: number, lower: boolean): string[] {
  return [length(ms, lower), length(ms + 60_000, lower)];
}

function durationText(ms: number): string {
  if (ms >= 3_600_000) {
    const hours = Math.floor(ms / 3_600_000);
    const minutes = Math.floor((ms % 3_600_000) / 60_000);
    if (minutes === 0) return count(hours, '1 時間', '%d 時間');
    return `${count(hours, '1 時間', '%d 時間')} ${count(minutes, '1 分', '%d 分')}`;
  }
  if (ms >= 60_000) return count(Math.floor(ms / 60_000), '1 分', '%d 分');
  return count(Math.max(Math.floor(ms / 1000), 1), '1 秒', '%d 秒');
}

function count(n: number, one: string, many: string): string {
  return n === 1 ? one : many.replace('%d', String(n));
}

function markLower(text: string, lower: boolean): string {
  return lower ? `${text}以上` : text;
}

function parts(at: Date): Record<string, string> {
  const f = new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Tokyo',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });
  const out: Record<string, string> = {};
  for (const { type, value } of f.formatToParts(at)) out[type] = value;
  return out;
}

// ---- reading what a save wrote ---------------------------------------------------------

// changedLines is the 1-based numbers of the lines that differ between two texts,
// with the line each became (e2eRolesChangedLines): how a save is held to the one
// line it was meant to write.
export function changedLines(before: string, after: string): Map<number, string> {
  const b = before.split('\n');
  const a = after.split('\n');
  const out = new Map<number, string>();
  for (let i = 0; i < Math.max(a.length, b.length); i++) {
    const l = i < b.length ? b[i] : '';
    const r = i < a.length ? a[i] : '';
    if (l !== r) out.set(i + 1, r);
  }
  return out;
}

// diff is what one file gained and lost, counted by the text of the line: a save
// that writes one line in the middle of a list shifts every line under it, so a
// line-by-line reading would call the shift a change (e2eTeamsDiff).
export function diff(before: string, after: string): { added: string[]; removed: string[] } {
  const count = (s: string) => {
    const m = new Map<string, number>();
    for (const l of s.split('\n')) m.set(l, (m.get(l) ?? 0) + 1);
    return m;
  };
  const bm = count(before);
  const am = count(after);
  const added: string[] = [];
  const removed: string[] = [];
  for (const [l, n] of am) for (let i = 0; i < n - (bm.get(l) ?? 0); i++) added.push(l);
  for (const [l, n] of bm) for (let i = 0; i < n - (am.get(l) ?? 0); i++) removed.push(l);
  added.sort();
  removed.sort();
  return { added, removed };
}
