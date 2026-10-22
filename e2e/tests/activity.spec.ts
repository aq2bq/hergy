// ACTIVITY-C1 and ACTIVITY-P4 — checks/activity.md's 大 items, walked in a
// browser. They are internal/app e2eActivityItemAtAGlance and e2eActivityAgain,
// test for test and assertion for assertion, over the same real-scale fixture
// (testdata/teams-v2 with testdata/activity over it: a made-up herdr and gy, 4
// running teams and 55 open questions) and the serve a person runs.
//
// The run's clock and zone are its own (r-f647): the serve reads its moment from
// HERGY_E2E_NOW (d-f978), and the fixture's times are counted back from the same
// moment, so what the rows say does not depend on when or where the run happens.
import { test, expect } from './fixtures';
import type { Page } from '@playwright/test';
import path from 'node:path';
import { readFileSync, writeFileSync } from 'node:fs';
import { press, q, read, wait } from './helpers';

test.use({ reads: 'activity' });

// drawn is the list having been drawn: rows, or the one line a list with nothing
// to show carries.
const drawn = `document.querySelectorAll('#list .act').length > 0 || !!document.querySelector('#list .gone')`;

// names is the teams the screen lists, in the order it lists them (2.2).
const names = `Array.from(document.querySelectorAll('#list .act')).map(function(a){return a.dataset.team}).join(',')`;

// band is the band's two numbers as the reading leaves them: the 活動 number and
// the 待ち word, or none where they are not written (2.1・2.7).
const band =
  `(function(){var s=document.querySelector('.screens .scr[data-screen="activity"]');` +
  `if(!s) throw new Error('no activity band');` +
  `var n=s.querySelector('.n'),w=s.querySelector('.wait');` +
  `return [n?n.textContent:'none',w?w.textContent:'none'].join('|')})()`;

// row is one row as it reads at a glance: the word of いま, who the row is about,
// and いつから (2.2).
function row(team: string): string {
  const t = JSON.stringify(team);
  return (
    `(function(){var a=document.querySelector('#list .act[data-team='+JSON.stringify(${t})+']');` +
    `if(!a) throw new Error('no row for '+${t});` +
    `return [a.querySelector('.c-st .st').textContent,a.querySelector('.c-who').textContent,a.querySelector('.c-since').textContent].join('|');})()`
  );
}

// open opens the screen on one address and waits for the answer to be drawn.
async function open(page: Page, base: string, query: string): Promise<void> {
  await page.goto(`${base}/activity${query}`);
  await wait(page, drawn);
}

// awaitFor waits for one expression beyond one act's own deadline: the screen
// reads again every ten seconds (2.6), so a change made behind it lands a poll
// later (activityAwait).
async function awaitFor(page: Page, script: string, span: number): Promise<void> {
  const deadline = Date.now() + span;
  while (Date.now() < deadline) {
    const ok = await page
      .evaluate(`(function(){try{return !!(${script});}catch(e){return false;}})()`)
      .catch(() => false);
    if (ok === true) return;
    await page.waitForTimeout(250);
  }
  throw new Error(`never became true within ${span}ms: ${script}`);
}

// agentState writes herdr's agent list again with one agent in one state, its
// change's seq moved up so the reading takes it as the newest (activityAgentState).
function agentState(cfg: string, name: string, state: string): void {
  const at = path.join(cfg, 'activity', 'herdr', 'agents.json');
  const doc = JSON.parse(readFileSync(at, 'utf8')) as {
    result: { agents: Record<string, unknown>[] };
  };
  let found = false;
  for (const agent of doc.result.agents) {
    if (agent.name !== name) continue;
    agent.agent_status = state;
    agent.state_change_seq = Number(agent.state_change_seq) + 1;
    found = true;
  }
  if (!found) throw new Error(`the fixture has no agent ${name}`);
  writeFileSync(at, JSON.stringify(doc));
}

test('ACTIVITY-C1 what a row says at a glance', async ({ page, app }) => {
  // C1: without opening a row, its いま and 誰 alone say that core's lead and
  // migrate's tester are the ones the human is waited by.
  await open(page, app.base, '');
  for (const one of [
    {
      team: 'core',
      state: '人間の番',
      who: '最後のターンlead→人間',
      reading: 'core の lead が人間の番',
    },
    { team: 'migrate', state: '許可待ち', who: 'tester', reading: 'migrate の tester が許可待ち' },
  ]) {
    const parts = (await read<string>(page, row(one.team))).split('|');
    expect(
      [parts[0], parts[1]],
      `${one.team} reads ${parts[0]}|${parts[1]}, want ${one.state}|${one.who} (${one.reading})`,
    ).toEqual([one.state, one.who]);
  }
});

test('ACTIVITY-P4 reading again', async ({ page, app }) => {
  // P4: the screen reads again every ten seconds (2.6), and the re-reading
  // moves only what the answer changed. core-lead is made to work, so core's
  // row turns 仕事中 and the order moves, while the cursor, the scroll, the open
  // rows and the let-out pane lines stay where the person left them. The band's
  // two numbers are the re-reading's own too (2.1): 待ち follows the members
  // waiting, down to nothing.
  await page.setViewportSize({ width: 1440, height: 900 });
  await open(page, app.base, '?open=core,migrate,core-hotfix,research');
  // The person lets migrate's held lines out and stands the cursor on them.
  const tail = q('#list .act[data-team="migrate"] .tail');
  await press(page, `${tail}.querySelector('.th .rest')`);
  await wait(page, `${tail}.querySelector('.term').textContent.split('\\n').length === 8`);
  await page.evaluate(`document.getElementById('tail-migrate').focus();window.scrollTo(0,120)`);
  const before = await read<string>(
    page,
    `(document.activeElement.id||'')+'|'+String(window.scrollY)+'|'+location.search`,
  );
  expect(
    before.startsWith('tail-migrate|120|?open=core,migrate,core-hotfix,research'),
    `the person stands at ${before} before the re-reading`,
  ).toBe(true);

  agentState(app.cfg, 'core-lead', 'working');
  await awaitFor(
    page,
    `(function(){var a=document.querySelector('#list .act[data-team="core"]');` +
      `return !!a && a.querySelector('.c-st .st').textContent === '仕事中'})()`,
    40_000,
  );

  expect(
    await read<string>(page, names),
    'after core started working the rows are, want the working teams last',
  ).toBe('migrate,core-hotfix,core,research');
  const parts = (await read<string>(page, row('core'))).split('|');
  expect(parts, "core's row reads, want 仕事中 and the member working with no いつから").toEqual([
    '仕事中',
    'lead',
    '',
  ]);
  expect(
    await read<string>(
      page,
      `(document.activeElement.id||'')+'|'+String(window.scrollY)+'|'+location.search`,
    ),
    `the re-reading moved the person, was ${before}`,
  ).toBe(before);
  expect(
    await read<string>(
      page,
      `(function(){var a=document.querySelector('#list .act[data-team="migrate"] .tail');` +
        `return String(a.querySelector('.term').textContent.split('\\n').length)+'|'+a.querySelector('.th').textContent})()`,
    ),
    'the let-out pane reads after the re-reading',
  ).toBe('8|画面の最後▾ 上の行をしまう');
  // A working team's opened part is the right column alone (2.5).
  expect(
    await read<string>(
      page,
      `String(!!document.querySelector('#list .act[data-team="core"] .ask'))`,
    ),
    'a team that started working still shows the left column',
  ).toBe('false');

  // P4（R2）: the band's two numbers are the re-reading's own (2.1). core-lead
  // works now, so the members waiting for a human or a permission fall from
  // two to one, 活動 staying at four.
  expect(
    await read<string>(page, band),
    'after the re-reading the band stands, want 活動 4 and 待ち 1',
  ).toBe('4|待ち 1');
  // P4（R2）: with the last member waiting gone, 待ち falls off the band (2.1).
  // The walk then puts the fixture back as the walks after it expect it.
  agentState(app.cfg, 'migrate-tester', 'working');
  await awaitFor(page, `${band} === '4|none'`, 40_000);
  agentState(app.cfg, 'migrate-tester', 'blocked');
  await page.goto(`${app.base}/activity`);
  await awaitFor(
    page,
    `(function(){var l=document.querySelector('#list .act[data-team="migrate"] .c-since .len');return !!l && l.textContent.length > 0})()`,
    30_000,
  );
});
