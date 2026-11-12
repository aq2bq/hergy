// Changes are answered only by the editor that owns the saved value and draft.
import { describe, expect, test } from 'bun:test';
import { editRole, editProfile, editTeam } from '../src/lib/editing';
import { resourceIdentity } from '../src/lib/resource-identity';
const doc = (over = {}) => ({ name: 'builder', title: '設計', text: '# 設計\n', ...over });
const role = (input = {}) => editRole(resourceIdentity('role', { name: 'builder', place: 'project' }), { saved: doc(), draft: doc(), hash: 'h1', ...input });
describe('editing changes', () => {
 test('the same saved and draft value is not a change', () => {
  const e = role(); expect(e.dirty.value).toBe(false); expect(e.lost.value).toEqual([]); e.dispose();
 });
 test('one edited field is a change, and lost names it', () => {
  const e = role(); e.change(doc({ text: '直した。' })); expect(e.dirty.value).toBe(true); expect(e.lost.value).toEqual(['本文']); e.dispose();
 });
 test('changing and restoring the value clears the change', () => {
  const e = role(); e.change(doc({ text: '直した。' })); e.change(doc()); expect(e.dirty.value).toBe(false); e.dispose();
 });
 test('a name alone is a change and names its moved file', () => {
  const e = role(); e.change(doc({ name: 'reviewer' })); expect(e.dirty.value).toBe(true); expect(e.nameChanged.value).toBe(true); expect(e.lost.value).toEqual(['名前（保存先のファイルも変わる）']); e.dispose();
 });
 test('a new resource is a change', () => {
  const profile = { name: 'new', kind: 'claude', args: [], env: [] };
  const e = editProfile(resourceIdentity('profile', { place: 'project' }), { saved: null, draft: profile, hash: '' }); expect(e.dirty.value).toBe(true); e.dispose();
  const missingRole = role({ saved: null }); expect(missingRole.dirty.value).toBe(false); missingRole.dispose();
 });
 test('team and profile editors compare the complete value their save writes', () => {
  const team = { description: 'd', turns: [['human', 'lead']], members: [{ name: 'lead', role: 'lead', profile: 'coder' }], workers: [] };
  const e = editTeam(resourceIdentity('team', { name: 'sample', place: 'project' }), { name: 'sample', team, hash: 'h1', agreement: null, agreementHash: '' });
  expect(e.team.dirty.value).toBe(false); e.act([{ kind: 'memberProfile', member: 'lead', profile: 'other' }]); expect(e.team.lost.value).toEqual(['メンバー']); e.dispose();
  const profile = { name: 'coder', kind: 'claude', args: ['--model', 'x'], env: [{ key: 'A', value: '1' }] };
  const p = editProfile(resourceIdentity('profile', { name: 'coder', place: 'project' }), { saved: profile, draft: profile, hash: 'p1' });
  expect(p.dirty.value).toBe(false); p.change({ ...profile, args: [] }); expect(p.lost.value).toEqual(['引数']); p.dispose();
 });
 test('input outside the written document does not create a change', () => {
  const e = role({ draft: { ...doc(), extra: undefined } }); expect(e.dirty.value).toBe(false); e.dispose();
 });
 test('the read projection cannot modify the private resource', () => {
  const e = role(); const read = e.read.value; read.draft.text = 'outside'; expect(e.dirty.value).toBe(false); expect(e.take().draft.text).toBe('# 設計\n'); e.dispose();
 });
});
