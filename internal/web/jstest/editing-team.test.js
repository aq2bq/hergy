import { expect, test } from 'bun:test';
import { later, serve } from './page.js';
import { editTeam, restoreTeam } from '../src/lib/editing';
import { resourceIdentity } from '../src/lib/resource-identity';
import { Refusal } from '../src/lib/api';
const team = (description = 'old') => ({ description, members: [{ name: 'lead', role: 'lead', profile: 'first' }], workers: [], turns: [['human', 'lead'], ['lead', 'human']] });
const id = (name = 'sample', project = '/work/alpha') => resourceIdentity('team', { name, place: 'project', project });
const open = () => editTeam(id(), { team: team(), hash: 't1', agreement: { name: '', title: 'sample', text: 'old rules' }, agreementHash: 'a1', name: 'sample' });
const answer = (description = 'sent', name = 'sample') => ({ name, hash: 't2', text: 'yaml', path: `${name}/team.yaml`, doc: team(description), items: [{ name, effective: 'project', places: { project: { hash: 't2', path: `${name}/team.yaml`, doc: { ...team(description), agreement: { hash: 'a2', path: `${name}/agreement.md`, has: true, doc: { name: '', title: name, text: 'sent rules' } } } } } }] });
for (const checkFirst of [false, true]) test(`new team name refusal survives validation, check first ${checkFirst}`, async () => {
 const pending = [], message = 'その名前は既にあります。別のチーム定義名を指定してください';
 serve((method) => { if (method === 'PUT') return Promise.reject(new Refusal(422, message, { at: 'name', text: message })); const one = later(); pending.push(one); return one.promise; });
 const e = editTeam(id(''), { name: 'taken', team: team(), hash: '', agreement: null, agreementHash: '', how: 'new', seeded: true });
 const checking = e.checkTeam();
 const answerChecks = async (checking) => { pending.forEach((one) => one.resolve({ problems: [] })); await checking; };
 if (checkFirst) await answerChecks(checking);
 await e.save('team');
 if (!checkFirst) await answerChecks(checking);
 expect(e.team.read.value.refusal).toBe(message);
 await answerChecks(e.checkTeam()); expect(e.team.read.value.refusal).toBe(message);
 e.changeName('available'); await answerChecks(e.checkTeam());
 expect(e.team.read.value.refusal).toBe(''); e.dispose();
});
for (const part of ['team', 'agreement']) for (const checkFirst of [false, true]) test(`${part} write refusal survives validation, check first ${checkFirst}`, async () => {
 const pending = [], message = '書き込み先を確認してください';
 serve((method) => { if (method === 'PUT') return Promise.reject(new Refusal(422, message, { at: 'doc', text: message })); const one = later(); pending.push(one); return one.promise; });
 const e = open(), editor = part === 'team' ? e.team : e.agreement;
 const checking = part === 'team' ? e.checkTeam() : e.checkAgreement();
 const answerChecks = async () => { pending.forEach((one) => one.resolve({ problems: [] })); await checking; };
 if (checkFirst) await answerChecks();
 await e.save(part);
 if (!checkFirst) await answerChecks();
 expect(editor.read.value.refusal).toBe(message); e.dispose();
});
for (const typed of [false, true]) test(`team save keeps sent basis and later input ${typed}`, async () => {
 const pending = later(); const asked = serve(() => pending.promise); const e = open();
 e.act([{ kind: 'description', text: 'sent' }]); const saving = e.save('team');
 if (typed) e.act([{ kind: 'description', text: 'later' }]);
 pending.resolve(answer()); await saving;
 expect(asked[0].body.hash).toBe('t1'); expect(asked[0].body.doc.ops).toEqual([{ kind: 'description', text: 'sent' }]);
 expect(e.team.read.value.saved.description).toBe('sent'); expect(e.team.read.value.draft.description).toBe(typed ? 'later' : 'sent');
 expect(e.changes.value.list).toEqual(typed ? [['c', '説明: later']] : []);
 expect(e.agreement.read.value.hash).toBe('a1'); e.dispose();
});
for (const typed of [false, true]) test(`folder save carries a later name into visible rename input ${typed}`, async () => {
 const pending = later(); serve(() => pending.promise);
 const e = editTeam(id(''), { name: 'sent', team: team(), hash: '', agreement: null, agreementHash: '', how: 'new', seeded: true });
 const saving = e.save('team'); if (typed) e.changeName('later');
 pending.resolve(answer('sent', 'sent')); await saving;
 expect(e.read.value.name).toBe('sent'); expect(e.read.value.renameDraft).toBe(typed ? 'later' : null);
 expect(e.read.value.fresh).toBe(false); e.dispose();
});
test('new and copied folders restore with both files and reject another project or vanished saved file', () => {
 const e = editTeam(id(''), { name: 'copy', team: team(), hash: '', agreement: null, agreementHash: '', how: 'copy', base: id('source'), seeded: true });
 e.act([{ kind: 'description', text: 'typed' }]); e.agreement.change({ name: '', title: '', text: 'typed rules' });
 e.teamYaml.change('raw draft'); const carry = e.take();
 const restored = restoreTeam(id(''), carry);
 expect(restored.team.read.value.draft.description).toBe('typed'); expect(restored.agreement.read.value.draft.text).toBe('typed rules');
 expect(restored.teamYaml.read.value.draft).toBe('raw draft'); expect(restored.read.value.base.name).toBe('source');
 expect(restoreTeam(id('', '/work/beta'), carry)).toBeUndefined();
 const saved = open(); expect(restoreTeam(id(''), saved.take())).toBeUndefined();
 e.dispose(); restored.dispose(); saved.dispose();
});
test('team and agreement retain original check results when draft and original answers arrive backwards', async () => {
 const pending = []; serve(() => { const one = later(); pending.push(one); return one.promise; });
 const e = open(); e.act([{ kind: 'description', text: 'typed' }]); const checking = e.checkTeam();
 pending[1].resolve({ problems: [{ sev: 'err', text: 'existing' }, { sev: 'err', text: 'new' }, { sev: 'err', at: 'node:lead', text: 'node' }] });
 pending[0].resolve({ problems: [{ sev: 'err', text: 'existing' }] }); await checking;
 expect(e.grew.value.map((p) => p.text)).toEqual(['new']); expect(e.team.read.value.baseProblems.map((p) => p.text)).toEqual(['existing']);
 e.agreement.change({ name: '', title: '', text: 'typed rules' }); const rules = e.checkAgreement();
 pending[3].resolve({ problems: [{ sev: 'err', text: 'existing rules' }] }); pending[2].resolve({ problems: [{ sev: 'err', text: 'existing rules' }] }); await rules;
 expect(e.agreementBlocked.value).toBe(false); e.dispose();
});
test('a copied agreement seeds its original text before the person types', async () => {
 serve(() => ({ problems: [], doc: { ...team(), agreement: { path: 'copy/agreement.md', hash: 'source', has: true, doc: { name: '', title: 'copy', text: 'source rules\n' } } }, draft: { path: 'copy/team.yaml', hash: '', writes: [] } }));
 const e = editTeam(id(''), { name: 'copy', team: team(''), hash: '', agreement: null, agreementHash: '', how: 'copy', base: id('source') });
 await e.checkTeam(); expect(e.agreement.read.value.draft.text).toBe('source rules\n'); e.dispose();
});
test('an allowed save keeps the sent figure when the response cannot build its graph', async () => {
 serve(() => ({ doc: { members: null, turns: null, workers: null, broken: true }, hash: 't2', text: 'saved yaml' }));
 const e = open(); e.act([{ kind: 'addMember', member: 'added', role: 'lead', profile: 'first' }]); await e.save('team');
 expect(e.team.read.value.saved.members.map((m) => m.name)).toEqual(['lead', 'added']); e.dispose();
});
test('the two file representations cannot restore each other even at the same hash', () => {
 const e = editTeam(id(), { name: 'sample', team: team(), hash: 'same', agreement: { name: '', title: '', text: 'same' }, agreementHash: 'same', teamText: 'same', agreementText: 'same' });
 e.agreementYaml.change('rules typed');
 expect(e.teamYaml.matches(e.agreementYaml.take())).toBe(false); expect(e.teamYaml.restore(e.agreementYaml.take())).toBe(false); e.dispose();
});
for (const typed of [false, true]) test(`agreement save keeps team edits separate and retains late input ${typed}`, async () => {
 const pending = later(); const asked = serve(() => pending.promise); const e = open();
 e.act([{ kind: 'description', text: 'team draft' }]); e.agreement.change({ name: '', title: '', text: 'sent rules' });
 const saving = e.save('agreement'); if (typed) e.agreement.change({ name: '', title: '', text: 'later rules' });
 pending.resolve({ doc: { name: '', title: '', text: 'sent rules' }, text: 'sent rules', hash: 'a2', path: 'sample/agreement.md' }); await saving;
 expect(asked[0].body).toMatchObject({ part: 'agreement', hash: 'a1', doc: { ops: [{ kind: 'description', text: 'team draft' }], doc: { text: 'sent rules' } } });
 expect(e.team.read.value.hash).toBe('t1'); expect(e.team.read.value.saved.description).toBe('old');
 expect(e.agreement.read.value.saved.text).toBe('sent rules'); expect(e.agreement.read.value.draft.text).toBe(typed ? 'later rules' : 'sent rules'); e.dispose();
});
for (const part of ['team', 'agreement']) test(`${part} raw save synchronizes only its file and preserves both late drafts`, async () => {
 const pending = later(); const asked = serve(() => pending.promise); const e = open();
 const raw = part === 'team' ? e.teamYaml : e.agreementYaml;
 raw.change('sent raw'); const saving = raw.save(); raw.change('later raw');
 if (part === 'team') e.act([{ kind: 'description', text: 'later document' }]);
 else e.agreement.change({ name: '', title: '', text: 'later document' });
 pending.resolve({ doc: part === 'team' ? team('sent document') : { name: '', title: '', text: 'sent document' }, text: 'sent raw', hash: 'new hash', path: `sample/${part}` }); await saving;
 expect(asked[0].body).toMatchObject({ part, text: 'sent raw', hash: part === 'team' ? 't1' : 'a1' });
 expect(raw.read.value.saved).toBe('sent raw'); expect(raw.read.value.draft).toBe('later raw');
 const edited = part === 'team' ? e.team : e.agreement;
 expect(part === 'team' ? edited.read.value.saved.description : edited.read.value.saved.text).toBe('sent document');
 expect(part === 'team' ? edited.read.value.draft.description : edited.read.value.draft.text).toBe('later document');
 expect((part === 'team' ? e.agreement : e.team).read.value.hash).toBe(part === 'team' ? 'a1' : 't1'); e.dispose();
});
test('the visible rename input carried after a new-folder save survives a band visit', async () => {
 const pending = later(); serve(() => pending.promise);
 const e = editTeam(id(''), { name: 'sample', team: team(), hash: '', agreement: null, agreementHash: '', how: 'new', seeded: true });
 const saving = e.save('team'); e.changeName('later name'); pending.resolve(answer()); await saving;
 const carry = e.take(); const freshReading = editTeam(id(), { name: 'sample', team: team('sent'), hash: 't2', agreement: { name: '', title: 'sample', text: 'sent rules' }, agreementHash: 'a2' });
 freshReading.restore(carry); expect(freshReading.read.value.renameDraft).toBe('later name');
 e.dispose(); freshReading.dispose();
});

test('figure marks and the change list come from the same private saved basis', () => {
 const before = {description: 'old', members: [{name: 'lead', role: 'lead', profile: 'first'}], workers: [{from: 'lead', purpose: 'work', profile: 'first'}], turns: [['human', 'lead']]};
 const e = editTeam(id(), {name: 'sample', team: before, hash: 'h1', agreement: null, agreementHash: ''});
 expect(e.changes.value.members.lead).toBe('');
 e.act([{kind: 'memberProfile', member: 'lead', profile: 'second'}, {kind: 'addMember', member: 'new', role: 'helper', profile: 'first'}, {kind: 'workerProfile', from: 'lead', purpose: 'work', profile: 'second'}, {kind: 'addWorker', from: 'lead', purpose: 'new', profile: 'first'}, {kind: 'addTurn', from: 'lead', to: 'human'}]);
 const changes = e.changes.value;
 expect(changes.members).toEqual({lead: 'changed', new: 'new'});
 expect(changes.workers.map((w) => w.added)).toEqual([false, true]);
 expect(changes.turns).toEqual([{from: 'human', to: 'lead', added: false}, {from: 'lead', to: 'human', added: true}]);
 expect(changes.list).toEqual([['c', 'lead のプロファイル first → second'], ['a', 'メンバー new（helper · first）'], ['c', 'ワーカー work（lead · second）'], ['a', 'ワーカー new（lead · first）'], ['a', 'ターン lead → 人間']]);
 e.replaceOperations([]);
 expect(e.changes.value.list).toEqual([]);
 expect(e.changes.value.members).toEqual({lead: ''}); e.dispose();
});

test('figure marks retain first-member identity when an invalid document repeats a name', () => {
 const before = {description: '', members: [{name: 'lead', role: 'lead', profile: 'first'}], workers: [], turns: []};
 const e = editTeam(id(), {name: 'sample', team: before, hash: 'h1', agreement: null, agreementHash: ''});
 e.team.change({...before, members: [...before.members, {name: 'lead', role: 'other', profile: 'second'}]});
 expect(e.changes.value.members.lead).toBe('');
 expect(e.changes.value.list).toEqual([['c', 'lead のロール lead → other'], ['c', 'lead のプロファイル first → second']]); e.dispose();
});
