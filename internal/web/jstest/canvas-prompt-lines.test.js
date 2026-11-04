// The member panel opens a read-only comparison; full text is never folded.
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
 await import('../src/teams/main');await settle();
 click(document.querySelector('.tdef[data-name="core"] .cv-n.member[data-cv-n="lead"]'));await settle();
 const sections=[...document.querySelectorAll('.cv-topology-section')];
 expect(sections.map(x=>x.dataset.kind)).toEqual(['roster','herdr','turns']);
 for(const section of sections) expect([...section.querySelectorAll('pre')].map(x=>x.textContent).join('')).toBe(memberSegments.filter(x=>x.kind===section.dataset.kind).map(x=>x.text).join(''));
 expect(document.querySelector('.cv-topology-prompt').textContent).not.toContain('TEAM-1');
 const trigger=document.querySelector('.cv-prompt button');click(trigger);await settle();
 click([...document.querySelectorAll('.prompt-comparison button')].find(x=>x.textContent==='閉じる'));await settle();
 click(trigger);await settle();
 click([...document.querySelectorAll('.prompt-tabs button')].find(x=>x.textContent==='渡した文'));await settle();
 expect(document.querySelector('.prompt-full').textContent).toBe(full);
 expect(document.querySelectorAll('.prompt-body details,.prompt-body .cv-fold').length).toBe(0);
 old.resolve({...answer,recorded:{...answer.recorded,text:'stale'}});await settle();
 expect(document.querySelector('.prompt-full').textContent).toBe(full);
 expect(document.querySelector('.prompt-context').textContent).toContain('受信の確認ではありません');
 expect(asked.filter(x=>x.url.includes('prompt-comparison')).every(x=>x.body.member==='lead')).toBe(true);
});
