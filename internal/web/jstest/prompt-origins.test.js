import {expect, test} from 'bun:test';
import {originLines} from '../src/teams/canvas/prompt-origins';
const segment=(kind,text,name=kind)=>({kind,text,origin:{kind,name,place:'project',path:`roles/${name}.md`}});

test('origin ranges preserve bytes, inline kinds and newline boundaries without marking template or actor',()=>{
 const segments=[segment('actor','actor\r\n'),segment('template','custom\n'),segment('roster','a'),segment('turns','b\n'),segment('template','\n'),segment('herdr','c\n\n'),segment('workers',''),segment('role_doc','r','one'),segment('role_doc','s','two'),segment('team_rules','t')];
 const text=segments.map(s=>s.text).join('');
 const lines=originLines(text,segments);
 expect(lines.map(x=>x.text).join('')).toBe(text);
 expect(lines.map(x=>x.segments.map(s=>s.kind))).toEqual([[],[],['roster','turns'],[],['herdr'],['herdr'],['role_doc','role_doc','team_rules']]);
 expect(lines.at(-1).segments.map(s=>s.origin.name)).toEqual(['one','two','team_rules']);
 expect(originLines('a\n',[segment('turns','a\n')])).toHaveLength(1);
 expect(originLines('',[])).toEqual([]);
 expect(originLines('wrong',segments)).toBeNull();
});
