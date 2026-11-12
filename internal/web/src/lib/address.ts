// lib/address.ts — 開いている行の住所（⑥-2, r-3d54 / d-a143）。
//
// 画面の行を URL に書く形と、URL から読む形の持ち主。チーム定義・ロール・
// プロファイル・アクティビティの 4 画面は、このモジュールを通してだけ
// `open` を読み書きする。
//
// 書く形: `open=名前|置き場所`。置き場所は、プロジェクトを並べた表示では
// プロジェクトのパスか `global`、一プロジェクト表示では `project` か `global`。
// 名前だけの値は書かない（writeOpen が投げる）。
//
// 読む形: 置き場所のある値は、その置き場所の行だけを開く（`project` は、その
// 画面の今のプロジェクトを指す）。置き場所の無い古い `open=名前` は、その名前の
// 行を両方（今のプロジェクトとグローバルの行）開く（d-a143 とその補足）。その
// どちらにも無い名前は、その名前の最初の行を開く（古いリンクの読み方）。どの値が
// どの行を指すかは rowsFor 1 つが決め、画面は行の名前と置き場所を渡すだけ。
import { term } from './terms.generated.js';
export type Ref = Readonly<{ name: string; group: string }>;
const addressBrand: unique symbol = Symbol('RowAddress');
export type RowAddress = Ref & { readonly [addressBrand]: true };

// Only a complete row address can be written. Old links remain readable.
export function rowAddress(name: string, group: string): RowAddress {
  if (!group) throw new Error(`${term('place')}の無い行は URL に書けません: ${name}`);
  return Object.freeze({ name, group, [addressBrand]: true as const });
}

// token joins a row's name and the place it stands in for the address. A name
// alone is the old value: it names every row of that name.
export const token = (name: string, group?: string): string => (group ? `${name}|${group}` : name);

// parseOpen reads one `open` value: comma separated refs, each a name and, after
// the last |, the place it stands in. No | is the old value (no place).
export function parseOpen(value: string | null | undefined): Ref[] {
  return String(value || '').split(',').filter(Boolean).map((one) => {
    const i = one.lastIndexOf('|');
    if (i < 0) return { name: one, group: '' };
    return { name: one.slice(0, i), group: one.slice(i + 1) };
  });
}

export const readOpen = (params: URLSearchParams): Ref[] => parseOpen(params.get('open'));

// writeOpen writes the refs a screen has open. A ref with no place would name
// every row of that name: a screen always knows the place it draws, so it never
// writes one.
export function writeOpen(params: URLSearchParams, refs: readonly RowAddress[]): void {
  const tokens = refs.map((one) => {
    if (!one.group) throw new Error(`${term('place')}の無い行は URL に書けません: ${one.name}`);
    return token(one.name, one.group);
  });
  if (tokens.length) params.set('open', tokens.join(','));
  else params.delete('open');
}

// placeGroup is the place a screen writes in its one-project view.
export const placeGroup = (place: 'project' | 'global'): string => place;

// groupPlace is the place a read ref stands for: `global`, this screen's project
// (`project` or the project's own path), or null for a group this screen does not
// draw.
export function groupPlace(group: string, current: string): 'project' | 'global' | null {
  if (group === 'global') return 'global';
  if (group === 'project' || (current !== '' && group === current)) return 'project';
  return null;
}

// AddressRow is one row as an address names it: the name it shows and the place
// it stands in, written the way writeOpen writes it (a project's path or
// `global`, or `project`/`global` on a one-project screen).
export type AddressRow = Ref;

// rowsFor is the rows one address names. A value with a place names the rows at
// that place alone. The old value, a name with no place, names this project's row
// and the global one; a name neither holds keeps the old reading, the first row
// of that name (d-a143 and its note).
export function rowsFor<T>(ref: Ref, rows: T[], of: (row: T) => AddressRow, current: string): T[] {
  const named = rows.filter((row) => of(row).name === ref.name);
  if (ref.group) {
    const place = groupPlace(ref.group, current);
    return named.filter((row) => {
      const group = of(row).group;
      if (group === ref.group) return true;
      if (place === 'project') return group === 'project' || group === current;
      if (place === 'global') return group === 'global';
      return false;
    });
  }
  const both = named.filter((row) => {
    const group = of(row).group;
    return group === current || group === 'project' || group === 'global';
  });
  return both.length ? both : named.slice(0, 1);
}
