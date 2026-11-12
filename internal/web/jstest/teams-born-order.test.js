import { expect, test } from 'bun:test';
import { layout, later, serve } from './page.js';
test('a draft discarded before its initial check answers cannot be inserted again', async () => {
 layout('teams-data', { team: { items: [], files: [] } });
 const pending = []; serve(() => { const one = later(); pending.push(one); return one.promise; });
 const s = await import('../src/teams/state');
 const adding = s.addDraft('new', 'project', '', null, ''); const r = s.recs.value[0];
 s.dropDraft(r);
 for (const one of pending) one.resolve({ problems: [], doc: { description: '', members: [], workers: [], turns: [] }, draft: { hash: '', path: 'team.yaml' } });
 await adding; expect(s.recs.value.includes(r)).toBe(false);
});
