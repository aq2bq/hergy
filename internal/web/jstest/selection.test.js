// The table (r-3d54): every event the owner names leaves the pick and the right
// column where SIDE_RULES says, from each column the screen can start at.
import { describe, expect, test } from 'bun:test';
import { samePick, select, createSelection } from '../src/teams/selection';
import { SIDE_RULES } from '../src/teams/selection-table';

const SIDES = ['attr', 'yaml', 'prompt'];
const BEFORE = { type: 'node', id: 'impl' };
const ONE = { type: 'node', id: 'lead' };
const TYPED = { id: 'lead', value: 'le', msgs: [] };

// rec is one record with the signals the owner writes and reads.
const rec = (side) => {
 const r = { typed: { value: TYPED }, draft: { value: { description: '', members: [{ name: 'lead' }, { name: 'impl' }], workers: [{ purpose: 'a' }], turns: [] } } };
 const selection = createSelection(() => { r.typed.value = null; });
 Object.assign(r, { selection, sel: selection.pick, side: selection.side });
 select(r, { how: 'field', what: BEFORE }); select(r, { how: 'side', side }); r.typed.value = TYPED;
 return r;
};

// EVENTS is one event per row of the table, as a caller hands it over.
const EVENTS = [
  { how: 'press', what: 'member', event: { how: 'press', what: ONE } },
  { how: 'press', what: 'worker', event: { how: 'press', what: { type: 'worker', i: 0 } } },
  { how: 'press', what: 'human', event: { how: 'press', what: { type: 'node', id: 'human' } } },
  { how: 'press', what: 'edge', event: { how: 'press', what: { type: 'edge', k: 'lead|impl' } } },
  { how: 'let', event: { how: 'let' } },
  { how: 'reset', event: { how: 'reset' } },
  { how: 'field', event: { how: 'field', what: ONE } },
  { how: 'side', event: { how: 'side', side: 'prompt' } },
  { how: 'yaml', event: { how: 'yaml' } },
];

describe('the table', () => {
  for (const one of EVENTS) {
    const rule = SIDE_RULES.find((row) => row.how === one.how && (!row.what || row.what.includes(one.what)));
    const label = `${one.how}${one.what ? ' ' + one.what : ''}`;
    test(`${label} is a row of the table`, () => {
      expect(rule).toBeDefined();
    });
    for (const from of SIDES) {
      test(`${label} from ${from}`, () => {
        const r = rec(from);
        select(r, one.event);
        const wantSide = rule.side === 'keep' ? from : rule.side === 'pressed' ? 'prompt' : rule.side;
        expect(r.side.value).toBe(wantSide);
        const wantPick = one.how === 'press' ? one.event.what : one.how === 'field' ? ONE : one.how === 'side' || one.how === 'yaml' ? BEFORE : null;
        expect(r.sel.value).toEqual(wantPick);
        // A pick that changes drops the half-typed name; a column does not.
        const picked = one.how === 'press' || one.how === 'let' || one.how === 'reset' || one.how === 'field';
        expect(r.typed.value).toEqual(picked ? null : TYPED);
      });
    }
  }
  test('a column the table does not know is left alone', () => {
    const r = rec('yaml');
    select(r, { how: 'side', side: 'nope' });
    expect(r.side.value).toBe('yaml');
  });
});

describe('samePick', () => {
  test('the same thing, field by field', () => {
    expect(samePick({ type: 'node', id: 'lead' }, { type: 'node', id: 'lead' })).toBe(true);
    expect(samePick({ type: 'worker', i: 1 }, { type: 'worker', i: 1 })).toBe(true);
    expect(samePick({ type: 'edge', k: 'a|b' }, { type: 'edge', k: 'a|b' })).toBe(true);
  });
  test('a different thing, or a different kind of thing', () => {
    expect(samePick({ type: 'node', id: 'lead' }, { type: 'node', id: 'impl' })).toBe(false);
    expect(samePick({ type: 'worker', i: 0 }, { type: 'worker', i: 1 })).toBe(false);
    expect(samePick({ type: 'edge', k: 'a|b' }, { type: 'edge', k: 'a|c' })).toBe(false);
    expect(samePick({ type: 'node', id: 'lead' }, { type: 'edge', k: 'lead|impl' })).toBe(false);
  });
  test('nothing, and nothing against something', () => {
    expect(samePick(null, null)).toBe(true);
    expect(samePick(null, { type: 'node', id: 'lead' })).toBe(false);
    expect(samePick({ type: 'node', id: 'lead' }, null)).toBe(false);
  });
});

test('a side change preserves the selected object and a caller cannot mutate it', () => {
 const r = rec('attr');
 const before = r.sel.value;
 select(r, { how: 'side', side: 'yaml' });
 expect(r.sel.value).toBe(before);
 const pick = { type: 'node', id: 'lead' }; select(r, { how: 'press', what: pick });
 pick.id = 'outside'; expect(r.sel.value.id).toBe('lead');
});
