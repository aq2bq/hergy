import { promptClock } from './prompt-clock.js';
// The member panel opens a read-only comparison; full text is never folded.
// ⑤ r-ba9b: the column and the dialog read through one owner, and an older
// answer that arrives after a newer one is not shown.
import { expect, test } from 'bun:test';
import { layout, serve, settle, later } from './page.js';
import { checkAnswer, click, docText, island, memberSegments } from './teams-page.js';

test('member comparison retains full recorded text and rejects a stale response', async () => {
 history.replaceState({}, '', '/teams?open=core');
 layout('teams-data', island());
 const full=docText(memberSegments)+'\n\t final  ';
 const old=later();let count=0;
 const answer={runs:[],selected:null,status:'recorded',reason:'',recorded:{text:full,command:'up',time:'2026-11-04T00:00:00Z'},current:full,segments:[...memberSegments,{kind:"template",text:"\n\t final  ",origin:{kind:"hergy"}}],problems:[],diff:[],project:'/example',executable:'hergy',language:'ja',team:'core',member:'lead',draft:false};
 const asked=serve((method,url,body)=>{
  if(url.startsWith('/api/team/prompt-comparison')) {count++;return count===1 ? old.promise : answer;}
  if(url.startsWith('/api/team/check') && body.part==='team')return checkAnswer(memberSegments);
  if(url.startsWith('/api/team/check'))return {problems:[]};
  if(url.startsWith('/api/role'))return {items:[]};
 });
 await promptClock();
 await import('../src/teams/main');await settle();await settle();
 click(document.querySelector('.tdef[data-name="core"] .cv-n.member[data-cv-n="lead"]'));await settle();
 click(document.querySelector('.tdef[data-name="core"] [data-cv-side="prompt"]'));await settle();await settle();await settle();
 // The first read is out (the test holds its answer): no text stands yet.
 const panel=document.querySelector('.cv-topology-prompt');
 expect(panel.querySelectorAll('.cv-topology-section').length).toBe(0);
 // 再取得 reads again while the first request is still out: the newer answer
 // stands, and the older one that arrives after it is not shown.
 click(panel.querySelector('.cv-prompt button'));await settle();
 click([...document.querySelectorAll('.prompt-comparison button')].find(x=>x.textContent==='再取得'));await settle();await settle();
 const sections=[...document.querySelectorAll('.cv-topology-section')];
 expect(sections.map(x=>x.dataset.kind)).toEqual(['roster','herdr','turns']);
 for(const section of sections) expect([...section.querySelectorAll('pre')].map(x=>x.textContent).join('')).toBe(memberSegments.filter(x=>x.kind===section.dataset.kind).map(x=>x.text).join(''));
 expect(panel.textContent).not.toContain('TEAM-1');
 click([...document.querySelectorAll('.prompt-tabs button')].find(x=>x.textContent==='渡した文'));await settle();
 expect(document.querySelector('.prompt-full').textContent).toBe(full);
 expect(document.querySelectorAll('.prompt-body details,.prompt-body .cv-fold').length).toBe(0);
 old.resolve({...answer,recorded:{...answer.recorded,text:'stale'}});await settle();await settle();
 expect(document.querySelector('.prompt-full').textContent).toBe(full);
 expect(document.querySelector('.prompt-context').textContent).toContain('受信の確認ではありません');
 expect(asked.filter(x=>x.url.includes('prompt-comparison')).every(x=>x.body.member==='lead')).toBe(true);
});
