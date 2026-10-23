// CANVAS-PROMPT — the prompt a member and a worker are handed, read from the
// canvas's attribute column. The lines are hergy's own segments (r-4cf9): when
// every line the team definition carries is put back together with the lines the
// folded documents hold on their own screens, they are `up --dry --json`'s prompt
// line for line (ac-5bfa). A worker's name and request are chosen at hire, so
// the walk puts the placeholders' values back and compares it with `hire --dry`.
import { test, expect } from './fixtures';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { hergyBin } from '../scripts/fixture.mjs';
import { readTo, there, wait } from './helpers';

test.use({ reads: 'teams' });

const SLOT_NAME = 'zz-worker-name-0';
const SLOT_TASK = 'zzTaskPlaceholder0';

// plan runs hergy in the test's own project with the settings the fixture gave
// its serve, and returns the JSON answer.
function plan(app: { proj: string; env: Record<string, string> }, args: string[]): any {
  return JSON.parse(execFileSync(hergyBin, args, { cwd: app.proj, env: { ...process.env, ...app.env }, encoding: 'utf8' }));
}

const real = (s: string) => (s === ' ' ? '' : s);

// owner reads the Markdown one folded document stands on: a role's screen, or
// the rule tab of the team the prompt was read from.
async function ownerRole(page: any, base: string, role: string, place: string): Promise<string[]> {
  await page.goto(`${base}/roles?open=${encodeURIComponent(role)}`);
  await wait(page, there('.panel[data-l] .yprev .ybox .yl'));
  await page.waitForTimeout(900);
  const lines = await page.$$eval(`.panel[data-l="${place}"]:not(.shadowed) .yprev .ybox .yl:not(.ghost) .yt`, (els) => els.map((e) => (e.textContent === ' ' ? '' : e.textContent || '')));
  while (lines.length && lines[lines.length - 1] === '') lines.pop();
  return lines;
}

test('CANVAS-PROMPT the lines a member is handed', async ({ page, app }) => {
  for (const team of ['core', 'docs', 'research'] as const) {
  const up = plan(app, ['up', team, '--dry', '--json']);
  await page.goto(`${app.base}/teams?open=${team}`);
  await wait(page, there(`.tdef[data-name="${team}"] .panel.team-p .cv-canvas svg`));

  // The team's own rules, once, for every fold of it.
  await page.click(`.tdef[data-name="${team}"] .dtab[data-t=agree]`);
  await wait(page, there(`.tdef[data-name="${team}"] .panel.agree-p .yprev .ybox .yl`));
  // The check that draws the excluded rules as ghost lines answers after the
  // first draw; wait for the ghost line the prompt carries (CORE-8).
  // The prompt carries the rules that go into it: a rule the file keeps out
  // (in_prompt: false) stands on the owner's screen as a ghost line, and is not
  // a line of the prompt. Read the owner's screen without its ghost lines.
  await page.waitForTimeout(900);
  const agree = await page.$$eval(`.tdef[data-name="${team}"] .panel.agree-p .yprev .ybox .yl:not(.ghost) .yt`, (els) => els.map((e) => (e.textContent === ' ' ? '' : e.textContent || '')));
  while (agree.length && agree[agree.length - 1] === '') agree.pop();
  await page.click(`.tdef[data-name="${team}"] .dtab[data-t=shape]`);
  await wait(page, there(`.tdef[data-name="${team}"] .panel.team-p .cv-canvas svg`));

  const rolePages = new Map<string, string[]>();
  for (const m of up.members as Array<{ name: string; prompt: string; segments: Array<{ kind: string; origin: { kind: string; name?: string } }> }>) {
    const short = m.name.slice(team.length + 1);
    await page.click(`.tdef[data-name="${team}"] .cv-n.member[data-cv-n="${short}"]`);
    await wait(page, there(`.tdef[data-name="${team}"] .cv-side .cv-prompt`));
    await wait(page, `document.querySelectorAll('.tdef[data-name="${team}"] .cv-prompt .yl[data-ln]').length > 0`);
    const seen = await page.$$eval(`.tdef[data-name="${team}"] .cv-prompt`, (els) => {
      const box = els[0];
      const lines: Record<string, string> = {};
      box.querySelectorAll('.yl[data-ln]').forEach((el) => { lines[(el as HTMLElement).dataset.ln || ''] = el.querySelector('.yt')?.textContent || ''; });
      const folds = [...box.querySelectorAll('.cv-fold[data-a]')].map((el) => ({
        a: Number((el as HTMLElement).dataset.a), b: Number((el as HTMLElement).dataset.b),
        doc: (el as HTMLElement).dataset.doc || '', role: (el as HTMLElement).dataset.role || '', place: (el as HTMLElement).dataset.place || '',
      }));
      const tpl = [...box.querySelectorAll('.cv-ptpl')].map((e) => e.textContent || '');
      return { lines, folds, tpl };
    });
    const want = m.prompt.split('\n');
    while (want.length && want[want.length - 1] === '') want.pop();

    // Every shown line is the prompt's own line at that number.
    for (const [i, text] of Object.entries(seen.lines)) {
      expect(real(text), `${m.name}: line ${Number(i) + 1}`).toBe(real(want[Number(i)] ?? ''));
    }
    // Every line is shown once, or inside one folded document.
    const count = new Array(want.length).fill(0);
    Object.keys(seen.lines).forEach((i) => { if (count[+i] !== undefined) count[+i]++; });
    for (const f of seen.folds) if (f.doc) for (let i = f.a; i < f.b; i++) count[i]++;
    const off = count.map((c, i) => [c, i]).filter(([c]) => c !== 1);
    expect(off, `${m.name}: every line sits in one place`).toEqual([]);

    // A folded role or team document holds the owner screen's own Markdown.
    for (const f of seen.folds) {
      if (!f.doc) continue;
      let lines = agree;
      if (f.doc === 'role') {
        const key = f.role + '.' + f.place;
        if (!rolePages.has(key)) rolePages.set(key, await ownerRole(page, app.base, f.role, f.place));
        lines = rolePages.get(key)!;
        await page.goto(`${app.base}/teams?open=${team}`);
        await wait(page, there(`.tdef[data-name="${team}"] .panel.team-p .cv-canvas svg`));
        await page.click(`.tdef[data-name="${team}"] .cv-n.member[data-cv-n="${short}"]`);
        await wait(page, there(`.tdef[data-name="${team}"] .cv-side .cv-prompt`));
        await wait(page, `document.querySelectorAll('.tdef[data-name="${team}"] .cv-prompt .yl[data-ln]').length > 0`);
      }
      const body = want.slice(f.a, f.b);
      while (body.length && body[body.length - 1] === '') body.pop();
      expect(lines, `${m.name}: the folded ${f.doc} at lines ${f.a + 1}–${f.b} is the owner's Markdown`).toEqual(body);
    }
    const firstT = m.segments.find((x) => x.kind === 'template')?.origin || { kind: 'hergy' };
    const wantTpl = firstT.kind === 'team' ? 'ひな形: チームの prompt:'
      : firstT.kind === 'member' ? `ひな形: メンバー ${firstT.name || ''} の prompt:` : 'ひな形: hergy の既定';
    expect(seen.tpl.join(' '), `${m.name}: the template comes from`).toContain(wantTpl);
  }
  }
});

test('CANVAS-PROMPT a worker is handed the placeholders back', async ({ page, app }) => {
  await page.goto(`${app.base}/teams?open=core`);
  await wait(page, there('.tdef[data-name="core"] .panel.team-p .cv-canvas svg'));
  await page.click('.tdef[data-name="core"] .cv-n.worker');
  await wait(page, there('.tdef[data-name="core"] .cv-side [data-cv-f=w-purpose]'));
  const from = await page.$eval('.tdef[data-name="core"] .cv-side [data-cv-f=w-from]', (el) => (el as HTMLSelectElement).value);
  const purpose = await page.$eval('.tdef[data-name="core"] .cv-side [data-cv-f=w-purpose]', (el) => (el as HTMLInputElement).value);
  await wait(page, there('.tdef[data-name="core"] .cv-prompt .yl'));
  const shown = await page.$$eval('.tdef[data-name="core"] .cv-prompt', (els) => {
    const out: string[] = [];
    els[0].querySelectorAll('.yl[data-ln]').forEach((el) => {
      const c = el.querySelector('.yt')!.cloneNode(true) as HTMLElement;
      c.querySelectorAll('.cv-slot').forEach((s) => { (s as HTMLElement).textContent = (s as HTMLElement).dataset.v || ''; });
      out.push(c.textContent || '');
    });
    return out;
  });
  while (shown.length && shown[shown.length - 1] === '') shown.pop();
  const hire = plan(app, ['hire', SLOT_NAME, '--from', from, '--purpose', purpose, '--task', SLOT_TASK, '--team', 'core', '--dry', '--json']);
  const want = hire.members[0].prompt.split('\n');
  while (want.length && want[want.length - 1] === '') want.pop();
  expect(shown, 'the worker prompt, with the placeholders put back, is hire --dry').toEqual(want);
});

test('CANVAS-PROMPT a definition with a problem says why, and gives the lines once it is fixed', async ({ page, app }) => {
  // d-20ac: a definition hergy will not build a prompt for shows the reason and
  // no lines; when the last problem goes, the lines with the unsaved ops appear.
  const blocked = 'このチーム定義には問題があるので、hergy はプロンプトを組みません';
  await page.goto(`${app.base}/teams?open=core`);
  await wait(page, there('.tdef[data-name="core"] .panel.team-p .cv-canvas svg'));
  await page.click('.tdef[data-name="core"] [data-cv-act=m-add]');
  await wait(page, there('.tdef[data-name="core"] .cv-side [data-cv-f=m-name]'));
  const name = await page.$eval('.tdef[data-name="core"] .cv-side [data-cv-f=m-name]', (el) => (el as HTMLInputElement).value);

  // (a) the new member hands the turn to no one: the lead's column says why and
  // holds no line.
  await page.click('.tdef[data-name="core"] .cv-n.member[data-cv-n="lead"]');
  await wait(page, there('.tdef[data-name="core"] .cv-prompt'));
  await readTo<string>(page, `document.querySelector('.tdef[data-name="core"] .cv-prompt').textContent`, 'the blocked column reads').toContain(blocked);
  await readTo<number>(page, `document.querySelectorAll('.tdef[data-name="core"] .cv-prompt .yl').length`, 'the blocked column holds lines').toBe(0);

  // (b) both directions are drawn (not saved): the lead's roster and its turns
  // name the new member, and the lines are there.
  const leadSel = '.tdef[data-name="core"] .cv-side select[data-cv-f=turn-add]';
  await page.selectOption(leadSel, name);
  await wait(page, `!!document.querySelector('.cv-toast')`);
  await page.waitForTimeout(500);
  await page.click(`.tdef[data-name="core"] .cv-n.member[data-cv-n="${name}"]`);
  await wait(page, there('.tdef[data-name="core"] .cv-side select[data-cv-f=turn-add]'));
  await page.selectOption('.tdef[data-name="core"] .cv-side select[data-cv-f=turn-add]', 'lead');
  await wait(page, `!!document.querySelector('.cv-toast')`);
  await page.waitForTimeout(500);
  await page.click('.tdef[data-name="core"] .cv-n.member[data-cv-n="lead"]');
  await wait(page, `document.querySelectorAll('.tdef[data-name="core"] .cv-prompt .yl').length > 0`);
  const text = await page.$eval('.tdef[data-name="core"] .cv-prompt', (el) => el.textContent || '');
  expect(text, 'the roster does not name the new member').toContain(`core-${name}`);
  expect(text, 'the turns do not name the new member with its role').toContain(`core-${name}（ロール`);
  expect(text, 'the blocked words stayed after the problem went').not.toContain(blocked);
});
