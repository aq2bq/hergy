// TEAMS-C4 — walked in a browser: the figure of a team definition is drawn at
// its own size and fits the window (r-e39c, d-943f). A two-member team is no
// longer stretched to the column's width, and a team of eight is laid out in
// rows instead of a ring too tall for the screen: at 1440×900 and 1280×720 the
// whole figure and the check column under it are in the window after the row is
// opened, without the person scrolling. The letters stay readable (a member's
// name is 11px or more), and the figure is never drawn larger than its own units.
import { test } from './fixtures';
import type { Page } from '@playwright/test';
import { mkdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { press, readTo, wait } from './helpers';

test.use({ reads: 'teams' });

// eight is the large team: eight members and ten turns. It is written into this
// walk's own project, so the fixture the other walks read keeps its list.
const eight = `version: 1
description: "注文を 8 人で回す"
members:
  - name: lead
    role: lead
    profile: opus-plan
  - name: analyst
    role: analyst
    profile: opus-deep
  - name: impl-a
    role: builder
    profile: codex-high
  - name: impl-b
    role: builder
    profile: sonnet-fast
  - name: reviewer
    role: reviewer
    profile: fable-review
  - name: scout
    role: scout
    profile: codex-mini
  - name: docs
    role: analyst
    profile: opus-deep
  - name: ops
    role: builder
    profile: codex-high
turns:
  - human -> lead
  - lead -> analyst
  - analyst -> impl-a
  - impl-a -> impl-b
  - impl-b -> reviewer
  - reviewer -> lead
  - lead -> scout
  - scout -> docs
  - docs -> ops
  - ops -> human
`;

function writeEight(proj: string): void {
  const dir = path.join(proj, '.hergy', 'teams', 'eight');
  mkdirSync(dir, { recursive: true });
  writeFileSync(path.join(dir, 'team.yaml'), eight);
}

// shape is one open row's figure as a person sees it: the multiplier it is
// drawn at (drawn width ÷ its own units), the member name's pixels on the
// screen, and where the figure, the save's band and the check column stand.
const shape = (name: string) => `(function(){
  var row=document.querySelector('.tdef[data-name=${JSON.stringify(name)}]');
  if(!row) return null;
  var svg=row.querySelector('.panel.team-p .cv-canvas svg');
  if(!svg) return null;
  var box=svg.getBoundingClientRect(), vw=Number(svg.dataset.vw), vh=Number(svg.dataset.vh);
  var foot=row.querySelector('.panel.team-p footer.pf');
  var head=row.querySelector('.panel.team-p .vlist .vhead');
  var nm=row.querySelector('.cv-n.member .nm');
  return {
    scale: box.width/vw,
    ownW: vw,
    ownH: vh,
    top: box.top,
    bottom: box.bottom,
    footTop: foot.getBoundingClientRect().top,
    checkTop: head.getBoundingClientRect().top,
    font: parseFloat(getComputedStyle(nm).fontSize),
    across: document.documentElement.scrollWidth,
    windowW: window.innerWidth,
    windowH: window.innerHeight
  };
})()`;

// fails lists what a person would find wrong with one row's figure: a
// multiplier above its own units or under the smallest readable size, letters
// under 11px, the figure or the check column out of the window, or the page
// scrolling sideways. The walk reads it again until the list is empty.
const fails = (name: string) => `(function(){
  var s=${shape(name)};
  var bad=[];
  if(!(s.scale<=1.001)) bad.push('drawn at '+s.scale.toFixed(2)+'x, larger than its own units');
  if(s.scale<0.8) bad.push('drawn at '+s.scale.toFixed(2)+'x, under 0.8');
  if(s.font*s.scale<11) bad.push('the member name is '+(s.font*s.scale).toFixed(1)+'px');
  if(s.top<0) bad.push('the figure starts above the window at '+s.top.toFixed(0));
  if(s.bottom>s.footTop) bad.push('the figure ends at '+s.bottom.toFixed(0)+', under the band at '+s.footTop.toFixed(0));
  if(s.checkTop>=s.windowH) bad.push('the check column starts at '+s.checkTop.toFixed(0)+' of '+s.windowH);
  if(s.across>s.windowW) bad.push('the page scrolls sideways: '+s.across+'>'+s.windowW);
  return bad;
})()`;

// opened opens one row the way a person does and waits for its figure.
async function opened(page: Page, name: string): Promise<void> {
  await press(page, `[...document.querySelectorAll('.tdef')].find(function(r){return r.dataset.name===${JSON.stringify(name)}}).querySelector('.sum')`);
  await wait(page, `!!document.querySelector('.tdef[data-name=${JSON.stringify(name)}] .panel.team-p .cv-canvas svg')`);
}

for (const viewport of [{ width: 1440, height: 900 }, { width: 1280, height: 720 }]) {
  test(`TEAMS-C4 the figure fits ${viewport.width}×${viewport.height}`, async ({ page, app }) => {
    writeEight(app.proj);
    await page.setViewportSize(viewport);
    await page.goto(`${app.base}/teams`);
    await wait(page, `!!document.querySelector('.tdef[data-name="draft"]') && !!document.querySelector('.tdef[data-name="eight"]')`);

    for (const name of ['draft', 'eight']) {
      await opened(page, name);
      await readTo<string[]>(page, fails(name), `the figure of ${name} never fit the window`).toEqual([]);
    }
  });
}
