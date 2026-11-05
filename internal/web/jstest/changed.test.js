// The one owner of 「変えたか」 (r-1e9a): it compares the value the save writes,
// and the field table only names what is lost.
import { describe, expect, test } from 'bun:test';
import { ROLE_DOC, PROFILE, TEAM, dirty, lost, same, snapshot } from '../src/lib/changed';

const role = (over) => ({ name: 'builder', title: '設計', text: '# 設計\n', ...(over || {}) });

describe('changed', () => {
  test('the same value is not a change', () => {
    expect(dirty(ROLE_DOC, role(), role())).toBe(false);
    expect(lost(ROLE_DOC, role(), role())).toEqual([]);
  });
  test('one place changed is a change, and lost names it', () => {
    expect(dirty(ROLE_DOC, role(), role({ text: '# 設計\n直した。\n' }))).toBe(true);
    expect(lost(ROLE_DOC, role(), role({ text: '# 設計\n直した。\n' }))).toEqual(['本文']);
  });
  test('changed and put back is not a change', () => {
    const from = snapshot(role());
    const to = role();
    to.text = '# 設計\n直した。\n';
    to.text = from.text;
    expect(dirty(ROLE_DOC, from, to)).toBe(false);
  });
  test('a name only is a change, and lost says the file moves with it', () => {
    const to = role({ name: 'reviewer' });
    expect(dirty(ROLE_DOC, role(), to)).toBe(true);
    expect(lost(ROLE_DOC, role(), to)).toEqual(['名前（保存先のファイルも変わる）']);
  });
  test('a field outside the table is a change, counted as そのほかの変更', () => {
    const shape = { fields: ROLE_DOC.fields };
    const to = { ...role(), extra: '表に無い欄' };
    expect(dirty(shape, role(), to)).toBe(true);
    expect(lost(shape, role(), to)).toEqual(['そのほかの変更']);
  });
  test('a new thing (no saved value) is a change', () => {
    expect(dirty(ROLE_DOC, null, role())).toBe(true);
    expect(lost(ROLE_DOC, null, role())).toContain('本文');
  });
  test('the team and profile writes compare their own fields', () => {
    const team = () => ({ description: 'd', turns: [['human', 'lead']], members: [{ name: 'lead', role: 'lead', profile: 'coder' }], workers: [] });
    expect(dirty(TEAM, team(), team())).toBe(false);
    const moved = team();
    moved.members[0].profile = 'other';
    expect(lost(TEAM, team(), moved)).toEqual(['メンバー']);
    const p = () => ({ name: 'coder', kind: 'claude', args: ['--model', 'x'], env: [{ key: 'A', value: '1' }] });
    expect(dirty(PROFILE, p(), p())).toBe(false);
    type: expect(lost(PROFILE, p(), { ...p(), args: [] })).toEqual(['引数']);
  });
  test('a field with no value is the same as the field being absent', () => {
    expect(dirty(ROLE_DOC, { ...role(), extra: undefined }, role())).toBe(false);
    expect(same({ a: 1, b: undefined }, { a: 1 })).toBe(true);
  });
  test('snapshot is a copy: changing it does not change the original', () => {
    const from = role();
    const copy = snapshot(from);
    copy.text = 'other';
    expect(from.text).toBe('# 設計\n');
    expect(same(from, copy)).toBe(false);
  });
});
