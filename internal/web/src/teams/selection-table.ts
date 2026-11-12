// selection-table.ts — 出来事と、右の列の行き先（r-3d54, n-045a）。
//
// このファイルは表の読み方だけを持つ。規則の中身と理由は selection.ts の冒頭に
// 書く。DOM に触れない葉なので、e2e も同じ表を import して期待を作る（2 つの表を
// 手で写さない）。
export type Side = 'attr' | 'yaml' | 'prompt';
// PickWhat is the kind of thing a pick names. human is the one node the draft
// always has.
export type PickWhat = 'member' | 'worker' | 'edge' | 'human' | null;
export type PickHow = 'press' | 'let' | 'reset' | 'field' | 'side' | 'yaml';
// SideRule is one row of the table: an event (and, when it matters, the thing it
// picked) and where the right column goes from here. 'keep' keeps the column the
// row started at; 'pressed' takes the column of the heading the person pressed.
export type SideRule = { readonly how: PickHow; readonly what?: readonly PickWhat[]; readonly side: 'keep' | 'pressed' | Side };
export const SIDE_RULES: ReadonlyArray<SideRule> = [
  { how: 'press', what: ['member', 'worker', 'human'], side: 'keep' },
  { how: 'press', what: ['edge'], side: 'attr' },
  { how: 'let', side: 'keep' },
  { how: 'reset', side: 'attr' },
  { how: 'field', side: 'attr' },
  { how: 'side', side: 'pressed' },
  { how: 'yaml', side: 'yaml' },
];
// nextSide answers the column the event leaves behind: 'keep' answers the column
// the row started at, and 'pressed' is the column of the heading the person
// pressed (the caller has it). An event the table does not name keeps the column:
// the table never moves a column it did not say to move.
export function nextSide(how: PickHow, what: PickWhat, from: Side): 'pressed' | Side {
  const rule = SIDE_RULES.find((one) => one.how === how && (!one.what || one.what.includes(what)));
  const side = rule ? rule.side : 'keep';
  return side === 'keep' ? from : side;
}
