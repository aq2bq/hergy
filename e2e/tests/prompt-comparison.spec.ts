import { test, expect } from './fixtures';
import { ready } from './helpers';
import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import path from 'node:path';
import { hergyBin } from '../scripts/fixture.mjs';

test.use({reads:'teams'});
for (const theme of ['light','dark'] as const) {
 test(`prompt comparison keeps recorded bytes and all topology lines in ${theme}`, async ({page,app}, info) => {
  await page.setViewportSize({width:1440,height:900});
  await page.emulateMedia({colorScheme:theme});
  const yaml=path.join(app.proj,'.hergy/teams/core/team.yaml');
  // Use the CLI's real prompt builder and launch-log format; Herdr membership
  // is the isolated fixture's, not an agent on the developer's machine.
  const plan=JSON.parse(execFileSync(hergyBin,['up','core','--dry','--json','--lang','ja'],{cwd:app.proj,env:{...process.env,...app.env},encoding:'utf8'}));
  const dir=path.join(app.state,'hergy/launches');mkdirSync(path.join(dir,'blobs'),{recursive:true});
  const members=plan.members.map((m: {name:string;prompt:string})=>{
   const hash=createHash('sha256').update(m.prompt).digest('hex');writeFileSync(path.join(dir,'blobs',hash),m.prompt);
   return {name:m.name,short:m.name.replace('core-',''),prompt:hash};
  });
  writeFileSync(path.join(dir,'log.jsonl'),JSON.stringify({time:new Date().toISOString(),command:'up',project:app.proj,layer:'local',definition:'core',team:'core',team_yaml:yaml,members})+'\n');
  const herdr=path.join(app.work,'herdr');mkdirSync(herdr,{recursive:true});
  writeFileSync(path.join(herdr,'tabs.json'),JSON.stringify({result:{tabs:[{tab_id:'t1',label:'core',workspace_id:'w1'}]}}));
  writeFileSync(path.join(herdr,'agents.json'),JSON.stringify({result:{agents:members.map((m:{name:string},i:number)=>({name:m.name,agent:'claude',agent_status:'idle',pane_id:`p${i+1}`,tab_id:'t1',workspace_id:'w1'}))}}));
  const before=readFileSync(yaml,'utf8');
  writeFileSync(yaml,before.replace('  - lead -> impl-a\n',''));
  const current=JSON.parse(execFileSync(hergyBin,['up','core','--dry','--json','--lang','ja'],{cwd:app.proj,env:{...process.env,...app.env},encoding:'utf8'}));
  await page.goto(`${app.base}/teams?open=core`);await ready(page);
  await page.locator('.cv-n.member[data-cv-n="lead"]').click();
  // The prompt is read in its own tab (r-f0be). One owner reads it for the
  // column and the dialog (⑤ r-ba9b): the answer comes with the tab, not again
  // when the dialog opens.
  const response=page.waitForResponse(r=>r.url().endsWith('/api/team/prompt-comparison')&&r.request().method()==='POST');
  await page.locator('.cv-side [data-cv-side="prompt"]').click();
  const comparison=await (await response).json();
  const entry=page.getByRole('button',{name:'全文と差分を読む',exact:true});
  const sections=page.locator('.cv-topology-section');
  await expect(sections).toHaveCount(4);
  await expect(page.locator('.cv-topology-prompt details')).toHaveCount(0);
  await expect.poll(async()=>{
   const button=await entry.boundingBox();const parent=await page.locator('.cv-topology-prompt').boundingBox();
   return !!button && !!parent && button.width<parent.width-20;
  }).toBe(true);
  await entry.click();
  const expectedKinds=expectedOrigins(comparison.segments);
  for(const kind of ['roster','herdr','turns','workers']) {
   await expect.poll(()=>page.locator(`.cv-topology-section[data-kind=${kind}] pre`).allTextContents().then(x=>x.join('')))
    .toBe(comparison.segments.filter((x:{kind:string})=>x.kind===kind).map((x:{text:string})=>x.text).join(''));
  }
  await expect(page.locator('.cv-topology-section[data-kind=role_doc],.cv-topology-section[data-kind=team_rules]')).toHaveCount(0);

  const panel=page.getByRole('dialog',{name:'プロンプト · core / lead',exact:true});
  await expect(panel.getByText('最後に記録された本文',{exact:false})).toBeVisible();
  await panel.getByRole('button',{name:'渡した文',exact:true}).click();
  await expect(panel.locator('.prompt-diff-summary')).toHaveCount(0);
  await expect.poll(() => panel.locator('.prompt-full').textContent()).toBe(plan.members.find((m:{name:string})=>m.name==='core-lead').prompt);
  await page.screenshot({path:info.outputPath(`${theme}-recorded.png`)});
  await expect(panel.locator('.prompt-diff-summary')).toHaveCount(0);
  await panel.getByRole('button',{name:'今の文',exact:true}).click();
  await expect.poll(() => panel.locator('.prompt-line-text').allTextContents().then(x=>x.join(''))).toBe(current.members.find((m:{name:string})=>m.name==='core-lead').prompt);
  await expect(panel.locator('.prompt-body details')).toHaveCount(0);
  await expect.poll(()=>panel.locator('.prompt-current-line').evaluateAll(rows=>rows.map(row=>[...row.querySelectorAll('.prompt-origin')].map(x=>x.getAttribute('data-kind'))))).toEqual(expectedKinds);
  // Each topology section stands as one band from its heading to its last
  // line. A heading from the template is inside its band and keeps no label;
  // a heading the section carries keeps the label it always had (r-04fc).
  const bands = () => panel.locator('.prompt-current-line').evaluateAll((rows) => rows.map((row, i) => ({
   i, text: row.querySelector('.prompt-line-text')!.textContent || '', section: row.getAttribute('data-section'),
   start: row.hasAttribute('data-section-start'), end: row.hasAttribute('data-section-end'),
   marks: row.querySelectorAll('.prompt-origin').length,
  })));
  await expect.poll(async () => {
   const rows = await bands(), bad: string[] = [];
   for (const [kind, heading, marks] of [['roster', '## 名簿\n', 0], ['herdr', '## 連絡\n', 0], ['turns', '## ターンを渡す相手\n', 1], ['workers', '## ワーカーの追加\n', 1]] as const) {
    const inside = rows.filter((row) => row.section === kind);
    if (!inside.length) { bad.push(`${kind}: no rows`); continue; }
    if (!inside[0].start) bad.push(`${kind}: no start`);
    if (!inside[inside.length - 1].end) bad.push(`${kind}: no end`);
    if (inside[inside.length - 1].i - inside[0].i !== inside.length - 1) bad.push(`${kind}: not one band`);
    const head = rows.find((row) => row.text === heading);
    if (!head) { bad.push(`${kind}: no heading`); continue; }
    if (head.section !== kind) bad.push(`${kind}: heading outside the band`);
    if (marks === 0 && head.marks !== 0) bad.push(`${kind}: the template heading carries a label`);
    if (marks === 1 && head.marks < 1) bad.push(`${kind}: the heading lost its label`);
   }
   return bad;
  }).toEqual([]);

  // The copy shows a band, not the top of the text.
  await panel.locator('.prompt-current-line[data-section="roster"]').first().scrollIntoViewIfNeeded();
  await page.screenshot({path:info.outputPath(`${theme}-current.png`)});
  await panel.getByRole('button',{name:'差',exact:true}).click();
  await expect(panel.locator('.prompt-diff-line.remove')).not.toHaveCount(0);
  // The diff says how much it holds and offers one step to its first line, and
  // opening it does not move anything by itself (r-3a3767, n-83f0).
  const counts = await panel.locator('.prompt-diff-line').evaluateAll((rows) => ({
   add: rows.filter((row) => row.classList.contains('add')).length,
   remove: rows.filter((row) => row.classList.contains('remove')).length,
  }));
  await expect(panel.locator('.prompt-diff-summary')).toContainText(`追加 ${counts.add} 行・削除 ${counts.remove} 行`);
  await expect.poll(() => panel.locator('.prompt-body').evaluate((node) => node.scrollTop)).toBe(0);
  await expect.poll(() => panel.locator('.prompt-body').evaluate((node) => {
   const first = node.querySelector('.prompt-diff-line.add, .prompt-diff-line.remove');
   return !!first && first.getBoundingClientRect().top - node.getBoundingClientRect().top >= 8;
  })).toBe(true);
  await page.screenshot({path:info.outputPath(`${theme}-diff-open.png`)});
  const pagePlace = await page.evaluate(() => scrollY);
  await panel.getByRole('button',{name:'最初の差へ',exact:true}).click();
  // The first difference ends up inside the body; a body that cannot scroll
  // that far shows it as low as it goes, which is still in view.
  await expect.poll(() => panel.locator('.prompt-body').evaluate((node) => {
   const first = node.querySelector('.prompt-diff-line.add, .prompt-diff-line.remove');
   const at = first ? first.getBoundingClientRect().top - node.getBoundingClientRect().top : -1;
   return at >= 0 && at < node.clientHeight;
  })).toBe(true);
  await expect.poll(() => page.evaluate(() => scrollY)).toBe(pagePlace);
  await expect(panel.locator('.prompt-diff-line.remove')).toContainText('core-impl-a');
  await expect(panel.locator('.prompt-diff-line.remove .prompt-origin')).toHaveCount(0);
  // A removed row inside a band stays in it (d-2f04). This fixture takes one
  // turn away, so the turns band runs through the removed row without a mark.
  await expect(panel.locator('.prompt-diff-line.remove[data-section="turns"]')).toHaveCount(1);
  await expect.poll(() => panel.locator('.prompt-diff-line').evaluateAll((rows) => {
   const inside = rows.flatMap((row, i) => (row.getAttribute('data-section') === 'turns' ? [i] : []));
   return inside.length > 0 && inside[inside.length - 1] - inside[0] + 1 === inside.length;
  })).toBe(true);
  // Every removed row wears a band exactly when the rows around it share one.
  await expect.poll(() => panel.locator('.prompt-diff-line').evaluateAll((rows) => {
   const bad: string[] = [];
   rows.forEach((row, i) => {
    if (!row.classList.contains('remove')) return;
    let above = i - 1; while (above >= 0 && rows[above].classList.contains('remove')) above -= 1;
    let below = i + 1; while (below < rows.length && rows[below].classList.contains('remove')) below += 1;
    const around = above >= 0 && below < rows.length ? rows[above].getAttribute('data-section') : null;
    const shared = around !== null && rows[below].getAttribute('data-section') === around;
    if (shared && row.getAttribute('data-section') !== around) bad.push(`removed ${i}: lost the band`);
    if (!shared && row.getAttribute('data-section')) bad.push(`removed ${i}: band outside one`);
   });
   return bad;
  })).toEqual([]);
  const firstRoster = panel.locator('.prompt-diff-line[data-section="roster"]').first();
  await expect(firstRoster).toHaveAttribute('data-section-start');
  await expect(firstRoster.locator('pre')).toHaveText('## 名簿');
  await expect.poll(() => panel.locator('.prompt-diff-line[data-section]').count()).toBeGreaterThan(0);
  await expect.poll(()=>panel.locator('.prompt-diff-line[data-new-line]').evaluateAll(rows=>rows.map(row=>({line:Number(row.getAttribute('data-new-line')),kinds:[...row.querySelectorAll('.prompt-origin')].map(x=>x.getAttribute('data-kind'))}))))
   .toEqual(comparison.diff.filter((x:{new:number})=>x.new).map((x:{new:number})=>({line:x.new,kinds:expectedKinds[x.new-1]})));
  await panel.locator('.prompt-diff-line[data-section="turns"]').first().scrollIntoViewIfNeeded();
  await page.screenshot({path:info.outputPath(`${theme}-diff.png`)});
  await panel.locator('.prompt-body').focus();
  await page.keyboard.press('Delete');
  await expect(panel).toBeVisible();
  await panel.getByRole('button',{name:'閉じる',exact:true}).click();
  await expect(page.locator('.cv-n.member[data-cv-n="lead"]')).toBeVisible();
  await expect(page.getByRole('button',{name:'全文と差分を読む',exact:true})).toBeFocused();
  await page.setViewportSize({width:1101,height:900});
  await expect.poll(async()=>{
   const button=await entry.boundingBox();const parent=await page.locator('.cv-topology-prompt').boundingBox();
   return !!button && !!parent && button.width<parent.width-20;
  }).toBe(true);
  await expect(entry).toHaveClass(/btn sm/);
  await page.screenshot({path:info.outputPath(`${theme}-entry.png`)});
 });
}

// Independent character-by-character oracle; newline belongs to its ending line.
function expectedOrigins(segments:{kind:string;text:string}[]) {
 const result:string[][]=[[]];let line=0;
 const marked=new Set(['roster','herdr','turns','workers','role_doc','team_rules']);
 for(const segment of segments) for(const char of segment.text) {
  if(marked.has(segment.kind)&&!result[line].includes(segment.kind))result[line].push(segment.kind);
  if(char==='\n') {line++;result.push([]);}
 }
 if(segments.map(x=>x.text).join('').endsWith('\n'))result.pop();
 return result;
}
