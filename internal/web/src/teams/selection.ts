// teams/selection.ts — 選んだもの（r.sel）と右の列（r.side）の持ち主（r-3d54,
// n-045a）。図・属性・YAML・プロンプト・検査の欄・画面の押し込みは、何が起きた
// かをここへ渡し、2 つを書くのは select だけにする。
//
// 規則（遷移の表。行き先の一覧は selection-table.ts の SIDE_RULES）:
//
// | 出来事 | 選ぶもの | 後の右の列 | 理由 |
// |---|---|---|---|
// | 点を押す（メンバー・ワーカー・人間） | その点 | そのまま | 点を見比べている間に列が飛ぶと読めない（r-f0be） |
// | 点をキーボード（Enter・Space）で選ぶ | その点 | そのまま | 押すのと同じ（r-f0be） |
// | 線を押す・線をキーボードで選ぶ | その線 | 属性 | 線にできること（両方向のターンの表示と外す）が属性にだけある（attr.tsx の edge、act.ts の del） |
// | 線を引いて離す（既にある・新しく足す） | その線 | 属性 | 同上。足したターンをすぐ確かめられる |
// | 空白を押す | なし | そのまま | 列は人が置いたものを動かさない |
// | Esc | なし | そのまま | 同上 |
// | 点をドラッグして離す | なし | そのまま | 同上 |
// | 外す（メンバー・線・ワーカー） | なし | そのまま | 同上 |
// | 足す（メンバー・ワーカー）・複製 | 新しい点 | 属性 | 直後に決める欄（名前・目的）が属性にある（focus は ⑦） |
// | 名前を変える（使える名前のとき） | 同じ点（新しい名前） | 属性 | 名前の欄は属性にある |
// | 検査の行を押す（点・ワーカー） | その点・ワーカー | 属性 | 直す欄が属性にある |
// | 検査の行を押す（名前の欄） | そのまま | そのまま | 欄へ focus するだけで、選び直しではない |
// | YAML の行を押す（説明） | なし | 属性 | 説明の欄へ focus する |
// | YAML の行を押す（点・ワーカー・線） | そのもの | 属性 | 直す欄が属性にある |
// | チームの説明を編集 | なし | 属性 | 説明は選択なしの属性に出る |
// | 保存・削除・元に戻すのあと | なし | 属性 | 図を引き直すので、選択は成り立たない（d-a575） |
// | 右の列の見出し（属性・YAML・プロンプト） | そのまま | 押した列 | 列を変えるのはこの押し込みだけ |
// | YAML へ | そのまま | YAML | 同上 |
//
// 検査の行・YAML の行が属性へ移る理由は、要求では確かめていなかった。上の理由は
// コードから読める（直す欄が属性にある）。
import type { CanvasPick, Rec } from './state.js';
import { nextSide } from './selection-table.js';
import type { PickWhat, Side } from './selection-table.js';

// PickEvent is what happened, as a caller saw it. The caller does not say which
// column to show: the table here does.
export type PickEvent =
  | { how: 'press'; what: CanvasPick }         // 点・線を押す／キー／離す／線を引いて離す
  | { how: 'let' }                             // 空白・Esc・ドラッグ・外す・保存/削除/元に戻すのあと
  | { how: 'field'; what: CanvasPick | null }  // 足す・複製・名前の変更・検査の行・YAML の行・説明を編集
  | { how: 'side'; side: Side }                // 右の列の見出し
  | { how: 'yaml' };                           // YAML へ

// select writes the pick and the right column, the one place both are written.
// A pick that changes drops the half-typed name (the field was about the old
// pick); a column that changes leaves it, and the pick, alone.
export function select(r: Rec, e: PickEvent): void {
  if (e.how === 'side') {
    if (isSide(e.side)) r.side.value = e.side;
    return;
  }
  if (e.how === 'yaml') { r.side.value = 'yaml'; return; }
  const pick = e.how === 'let' ? null : e.what;
  r.sel.value = pick;
  r.typed.value = null;
  const next = nextSide(e.how, whatOf(pick), sideOf(r));
  if (next !== 'pressed') r.side.value = next;
}

export const isSide = (value: string): value is Side => value === 'attr' || value === 'yaml' || value === 'prompt';

// whatOf is the table's word for a pick: the human is its own kind of node.
function whatOf(pick: CanvasPick | null): PickWhat {
  if (!pick) return null;
  if (pick.type === 'edge') return 'edge';
  if (pick.type === 'worker') return 'worker';
  return pick.id === 'human' ? 'human' : 'member';
}

// pickOf is the pick the draft still holds: a point the draft no longer has is
// no pick, and the panel would show an attribute of nothing.
export function pickOf(r: Rec): CanvasPick | null {
  const pick = r.sel.value;
  if (!pick) return null;
  const t = r.draft.value;
  const hasMember = (name: string): boolean => (t.members || []).some((m) => m.name === name);
  if (pick.type === 'node' && pick.id !== 'human' && !hasMember(pick.id)) return null;
  if (pick.type === 'worker' && !(t.workers || [])[pick.i]) return null;
  if (pick.type === 'edge' && !pick.k.split('|').every((x) => x === 'human' || hasMember(x))) return null;
  return pick;
}

// sideOf is the right column as the screen reads it: a value the table does not
// know is the attribute column, where the screen starts.
export function sideOf(r: Rec): Side {
  return isSide(r.side.value) ? r.side.value : 'attr';
}

// samePick says whether two picks name the same thing, field by field (the
// screen used to compare their JSON).
export function samePick(a: CanvasPick | null, b: CanvasPick | null): boolean {
  if (!a || !b) return a === b;
  if (a.type === 'node') return b.type === 'node' && a.id === b.id;
  if (a.type === 'worker') return b.type === 'worker' && a.i === b.i;
  return b.type === 'edge' && a.k === b.k;
}

// startSelection is the value a record starts with: nothing picked, the
// attribute column.
export const startSelection = (): { sel: null; side: Side } => ({ sel: null, side: 'attr' });
