// ACTIVITY-C1 and ACTIVITY-P4 — walked in a browser (spec-activity 2,
// spec 6 章, d-8a45). Only what the browser alone can see is compared here
// (d-3e43): who the screen says is waiting for the human before a team is
// picked, and the ten-second re-reading leaving the person's pick, scroll and
// address where they were.
//
// The run's clock and zone are its own (r-f647): the serve reads its moment from
// HERGY_E2E_NOW (d-f978), and the fixture's times are counted back from the same
// moment, so what the rows say does not depend on when or where the run happens.
import { test } from './fixtures';
import type { Page } from '@playwright/test';
import path from 'node:path';
import { readFileSync, writeFileSync } from 'node:fs';
import { press, q, read, readTo, wait } from './helpers';

test.use({ reads: 'activity' });

// drawn is the board having been drawn: one team's figure, or the one line a
// screen with nothing running carries.
const drawn = `document.querySelectorAll('.hq-tile').length > 0 || !!document.querySelector('#list .gone')`;

// band is the band's two numbers as the reading leaves them: the 活動 number and
// the 待ち word, or none where they are not written (2.1・2.7).
const band =
  `(function(){var s=document.querySelector('.screens .scr[data-screen="activity"]');` +
  `if(!s) throw new Error('no activity band');` +
  `var n=s.querySelector('.n'),w=s.querySelector('.wait');` +
  `return [n?n.textContent:'none',w?w.textContent:'none'].join('|')})()`;

// waiting is one team's row in the first tier as it reads at a glance: the badge
// and the member herdr knows (spec 6 章).
const waiting = (team: string): string =>
  `(function(){var a=document.querySelector('.hq-at[data-pick='+JSON.stringify(${JSON.stringify(team)})+']');` +
  `if(!a) throw new Error('no waiting row for '+${JSON.stringify(team)});` +
  `return [a.querySelector('.st').textContent,a.querySelector('.hq-aw code').textContent].join('|');})()`;

// state is one team's figure's badge (spec 6 章).
const state = (team: string): string =>
  `(function(){var a=document.querySelector('.hq-tile[data-team='+JSON.stringify(${JSON.stringify(team)})+']');` +
  `if(!a) throw new Error('no figure for '+${JSON.stringify(team)});` +
  `return a.querySelector('.st').textContent;})()`;

// open opens the screen on one address and waits for the board.
async function open(page: Page, base: string, query: string): Promise<void> {
  await page.goto(`${base}/activity${query}`);
  await wait(page, drawn);
}

// awaitFor waits for one expression beyond one act's own deadline: the screen
// reads again every ten seconds (2.6), so a change made behind it lands a poll
// later.
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
// change's seq moved up so the reading takes it as the newest.
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

test('ACTIVITY-C1 who the screen says is waiting', async ({ page, app }) => {
  // C1: before a team is picked, the first tier says whose answer is waited by
  // the human: core's lead handed the turn over, and migrate's tester waits for
  // a permission.
  await open(page, app.base, '');
  await readTo<string>(page, waiting('core'), 'core reads, want 人間の番 with its herdr name').toBe(
    '人間の番|core-lead',
  );
  await readTo<string>(page, waiting('migrate'), 'migrate reads, want 許可待ち with its herdr name').toBe(
    '許可待ち|migrate-tester',
  );
  await readTo<string>(page, state('core'), "core's figure reads, want 人間の番").toBe('人間の番');
});

test('ACTIVITY-P4 reading again', async ({ page, app }) => {
  // P4: the screen reads again every ten seconds (2.6), and the re-reading
  // moves only what the answer changed: the picked team and the scroll stay
  // where the person left them, and the band's two numbers are the re-reading's
  // own (2.1).
  await page.setViewportSize({ width: 1440, height: 900 });
  await open(page, app.base, '');
  const pickedBefore = await read<string>(page, `document.querySelector('.hq-tile.sel').dataset.team`);
  await page.evaluate(
    `(function(){var t=[...document.querySelectorAll('.hq-tile')].find(function(x){return !x.classList.contains('sel')});t.querySelector('.hq-th').click()})()`,
  );
  await wait(page, `document.querySelector('.hq-tile.sel').dataset.team !== ${JSON.stringify(pickedBefore)}`);
  await page.evaluate('window.scrollTo(0, 120)');
  // stands is the three things the re-reading must leave alone: the picked
  // team, the scroll and the address.
  const stands = `document.querySelector('.hq-tile.sel').dataset.team+'|'+String(window.scrollY)+'|'+location.search`;
  const before = await read<string>(page, stands);

  agentState(app.cfg, 'core-lead', 'working');
  await awaitFor(page, `document.querySelector('.hq-tile[data-team="core"] .st').textContent === '仕事中'`, 40_000);

  await readTo<string>(page, stands, `the re-reading moved the person, was ${before}`).toBe(before);
  // The band's waiting number follows the members: core-lead no longer waits.
  await readTo<string>(page, band, 'after the re-reading the band stands, want 活動 4 and 待ち 1').toBe('4|待ち 1');
});
