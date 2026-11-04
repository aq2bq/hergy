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
  await page.getByRole('button',{name:'プロンプトを読む',exact:true}).click();
  const panel=page.getByRole('dialog',{name:'プロンプト · core / lead',exact:true});
  await expect(panel.getByText('最後に記録された本文',{exact:false})).toBeVisible();
  await panel.getByRole('button',{name:'渡した文',exact:true}).click();
  await expect.poll(() => panel.locator('.prompt-full').textContent()).toBe(plan.members.find((m:{name:string})=>m.name==='core-lead').prompt);
  await page.screenshot({path:info.outputPath(`${theme}-recorded.png`)});
  await panel.getByRole('button',{name:'今の文',exact:true}).click();
  await expect.poll(() => panel.locator('.prompt-full').textContent()).toBe(current.members.find((m:{name:string})=>m.name==='core-lead').prompt);
  await expect(panel.locator('.prompt-body details')).toHaveCount(0);
  await page.screenshot({path:info.outputPath(`${theme}-current.png`)});
  await panel.getByRole('button',{name:'差',exact:true}).click();
  await expect(panel.locator('.prompt-diff-line.remove')).not.toHaveCount(0);
  await expect(panel.locator('.prompt-diff-line.remove')).toContainText('core-impl-a');
  await page.screenshot({path:info.outputPath(`${theme}-diff.png`)});
  await panel.locator('.prompt-body').focus();
  await page.keyboard.press('Delete');
  await expect(panel).toBeVisible();
  await panel.getByRole('button',{name:'閉じる',exact:true}).click();
  await expect(page.locator('.cv-n.member[data-cv-n="lead"]')).toBeVisible();
  await expect(page.getByRole('button',{name:'プロンプトを読む',exact:true})).toBeFocused();
  await page.screenshot({path:info.outputPath(`${theme}-entry.png`)});
 });
}
