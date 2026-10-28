// teams/canvas/shared.ts — 構成図の部品が共有する読みと計算（spec-teams B7、d-a575）。
//
// 図・属性の列・YAML のプレビュー・ドラッグは、同じ「選んだもの」「点の場所」
// 「点の箱」を読む。その読みをここに 1 つずつ置き、部品は描くことだけを持つ。
import type { CanvasPick, Rec } from '../state.js';
import { memberOf } from '../state.js';
import type { Box, Positions, Spot } from '../place.js';

// NAME is the pattern a member's name has to match: the field beside the figure
// and the figure itself say why a name the server would refuse is not taken.
export const NAME = /^[a-z][a-z0-9_-]{0,31}$/;

// CanvasMenu is the blank space's menu as the record keeps it: where the press
// was, the place a point would go, and whether only workers can be added there.
export type CanvasMenu = { x: number; y: number; ax: number | null; ay: number | null; workersOnly: boolean };

// CanvasTyped is a member's name while a person is typing it: the letters the
// draft cannot hold yet, and the words the field says beside them (r-4fd7).
export type CanvasTyped = { id: string; value: string; msgs: string[] };

// menuOf and typedOf read the record's two slots that hold the part's own
// short-lived state. The record names their shape now, so no cast is needed.
export const menuOf = (r: Rec): CanvasMenu | null => r.menu.value;
export const setMenu = (r: Rec, menu: CanvasMenu | null): void => { r.menu.value = menu; };
export const typedOf = (r: Rec): CanvasTyped | null => r.typed.value;
export const setTyped = (r: Rec, typed: CanvasTyped | null): void => { r.typed.value = typed; };

// livePick is the pick the draft still holds: a point the draft no longer has
// is no pick, and the panel would show an attribute of nothing.
export function livePick(r: Rec, sel: CanvasPick | null): CanvasPick | null {
  if (!sel) return null;
  const t = r.draft.value;
  if (sel.type === 'node' && sel.id !== 'human' && !memberOf(t, sel.id)) return null;
  if (sel.type === 'worker' && !t.workers[sel.i]) return null;
  if (sel.type === 'edge' && !sel.k.split('|').every((x) => x === 'human' || memberOf(t, x))) return null;
  return sel;
}
export const selOf = (r: Rec): CanvasPick | null => livePick(r, r.sel.value);
export const sideOf = (r: Rec): string => r.side.value || 'attr';

// layoutOf is every point's place as this row's drawing has it: the places a
// person put points down at, and an automatic place for the rest. It is a
// computed on the record, so a change to the draft or to a place draws the
// figure again (webui-components 1 章).
export const layoutOf = (r: Rec): Positions => r.layout.value;

// putPlace puts one point down where a person put it (a double press's menu, or
// a drag that was let go).
export function putPlace(r: Rec, id: string, at: Spot): void {
  r.placed.value = { ...r.placed.value, [id]: [at[0], at[1]] };
}

// renamePlace carries the place of a member the person renamed.
export function renamePlace(r: Rec, from: string, to: string): void {
  const { [from]: was, ...rest } = r.placed.value;
  r.placed.value = was ? { ...rest, [to]: was } : rest;
}

// edgePt is where a line leaves a point's box on its way to another point.
export const edgePt = (a: Spot, box: Box, b: Spot): Spot => {
  const dx = b[0] - a[0], dy = b[1] - a[1];
  if (!dx && !dy) return a;
  const k = Math.min((box.w / 2 + 3) / Math.abs(dx || 1e-6), (box.h / 2 + 3) / Math.abs(dy || 1e-6));
  return [a[0] + dx * k, a[1] + dy * k];
};

// pair is the key a turn's two ends stand under, whichever way it goes.
export const pair = (a: string, b: string): string => [a, b].sort().join('|');
