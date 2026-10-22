// TestE2EActivityItems' ACTIVITY-C1 and TestE2EActivity's ACTIVITY-P4, written
// again in Playwright (d-3c9b). The fixture's moments are counted back from the
// run's own clock, which the e2e binary reads from HERGY_E2E_NOW (d-f978,
// r-f647), so the rows say the same thing whenever and wherever the run happens.
import { test, expect, type Page } from '@playwright/test';
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import {
  go, makeFixture, press, removeRoot, script, startServe, text, wait, watch,
  type Fixture,
} from './harness';

// activityDrawn is the list having been drawn (TestE2EActivity).
const activityDrawn =
  `document.querySelectorAll('#list .act').length > 0 || !!document.querySelector('#list .gone')`;

// activityNames is the teams the screen lists, in the order it lists them.
const activityNames =
  `Array.from(document.querySelectorAll('#list .act')).map(function(a){return a.dataset.team}).join(',')`;

// activityBandScript is the band's two numbers, or none where they are not written.
const activityBandScript =
  `(function(){var s=document.querySelector('.screens .scr[data-screen="activity"]');` +
  `if(!s) throw new Error('no activity band');` +
  `var n=s.querySelector('.n'),w=s.querySelector('.wait');` +
  `return [n?n.textContent:'none',w?w.textContent:'none'].join('|')})()`;

// activityRowScript is one row as it reads at a glance: いま, 誰, いつから.
function activityRowScript(team: string): string {
  const q = JSON.stringify(team);
  return `(function(){var a=document.querySelector('#list .act[data-team='+JSON.stringify(${q})+']');` +
    `if(!a) throw new Error('no row for '+${q});` +
    `return [a.querySelector('.c-st .st').textContent,a.querySelector('.c-who').textContent,a.querySelector('.c-since').textContent].join('|');})()`;
}

async function activityOpen(page: Page, base: string, query: string): Promise<void> {
  await go(page, `${base}/activity${query}`);
  await wait(page, activityDrawn);
}

// activityAgentState writes herdr's agent list again with one agent in one
// state, its change's seq moved up so the reading takes it as the newest.
function activityAgentState(f: Fixture, name: string, state: string): void {
  const at = path.join(f.cfg, 'activity', 'herdr', 'agents.json');
  const doc = JSON.parse(readFileSync(at, 'utf8'));
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

test('ACTIVITY-C1 what a row says at a glance', async ({ page }) => {
  const f = makeFixture('activity');
  const serve = await startServe(f);
  try {
    const said = watch(page, false);
    await activityOpen(page, serve.url, '');
    // C1: without opening a row, its いま and 誰 alone say that core's lead and
    // migrate's tester are the ones the human is waited by.
    for (const one of [
      { team: 'core', state: '人間の番', who: '最後のターンlead→人間' },
      { team: 'migrate', state: '許可待ち', who: 'tester' },
    ]) {
      const parts = (await text(page, activityRowScript(one.team))).split('|');
      expect(parts[0], `${one.team}'s いま`).toBe(one.state);
      expect(parts[1], `${one.team}'s 誰`).toBe(one.who);
    }
    expect(said, 'the screens wrote to the console').toEqual([]);
  } finally {
    await serve.stop();
    removeRoot(f);
  }
});

test('ACTIVITY-P4 reading again', async ({ page }) => {
  const f = makeFixture('activity');
  const serve = await startServe(f);
  try {
    const said = watch(page, false);
    await page.setViewportSize({ width: 1440, height: 900 });
    await activityOpen(page, serve.url, '?open=core,migrate,core-hotfix,research');
    // The person lets migrate's held lines out and stands the cursor on them.
    const tail = `document.querySelector('#list .act[data-team="migrate"] .tail')`;
    await press(page, `${tail}.querySelector('.th .rest')`);
    await wait(page, `(${tail}.querySelector('.term').textContent.split('\\n').length === 8)`);
    await script(page, `(function(){document.getElementById('tail-migrate').focus();window.scrollTo(0,120);})()`);
    const before = await text(page,
      `(document.activeElement.id||'')+'|'+String(window.scrollY)+'|'+location.search`);
    expect(before.startsWith('tail-migrate|120|?open=core,migrate,core-hotfix,research'),
      'the person stands before the re-reading').toBe(true);

    activityAgentState(f, 'core-lead', 'working');
    await wait(page,
      `(function(){var a=document.querySelector('#list .act[data-team="core"]');` +
      `return !!a && a.querySelector('.c-st .st').textContent === '仕事中'})()`,
      40_000, 250);

    expect(await text(page, activityNames),
      'after core started working the rows, want the working teams last').toBe('migrate,core-hotfix,core,research');
    const parts = (await text(page, activityRowScript('core'))).split('|');
    expect(parts[0], "core's いま").toBe('仕事中');
    expect(parts[1], "core's 誰").toBe('lead');
    expect(parts[2], "core's いつから").toBe('');
    expect(await text(page, `(document.activeElement.id||'')+'|'+String(window.scrollY)+'|'+location.search`),
      'the re-reading moved the person').toBe(before);
    expect(await text(page,
      `(function(){var a=document.querySelector('#list .act[data-team="migrate"] .tail');` +
      `return String(a.querySelector('.term').textContent.split('\\n').length)+'|'+a.querySelector('.th').textContent})()`),
      'the let-out pane after the re-reading').toBe('8|画面の最後▾ 上の行をしまう');
    // A working team's opened part is the right column alone.
    expect(await text(page, `String(!!document.querySelector('#list .act[data-team="core"] .ask'))`),
      'a team that started working still shows the left column').toBe('false');

    // P4（R2）: the band's two numbers are the re-reading's own. core-lead works
    // now, so the members waiting for a human or a permission fall from two to
    // one, 活動 staying at four.
    expect(await text(page, activityBandScript),
      'after the re-reading the band').toBe('4|待ち 1');
    // With the last member waiting gone, 待ち falls off the band. The walk then
    // puts the fixture back as the walks after it expect it.
    activityAgentState(f, 'migrate-tester', 'working');
    await wait(page, `${activityBandScript} === '4|none'`, 40_000, 250);
    activityAgentState(f, 'migrate-tester', 'blocked');
    await go(page, `${serve.url}/activity`);
    await wait(page,
      `(function(){var l=document.querySelector('#list .act[data-team="migrate"] .c-since .len');return !!l && l.textContent.length > 0})()`,
      30_000, 250);

    expect(said, 'the screens wrote to the console').toEqual([]);
  } finally {
    await serve.stop();
    removeRoot(f);
  }
});
