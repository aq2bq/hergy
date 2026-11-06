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
// 読む形: 置き場所の無い古い `open=名前` は、その名前の行を両方（プロジェクトの
// 行とグローバルの行）開く（d-a143）。置き場所のある値は、その置き場所の行だけを
// 開く。`project` は、その画面の今のプロジェクトを指す。
export type Ref = { name: string; group: string };

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
export function writeOpen(params: URLSearchParams, refs: Ref[]): void {
  const tokens = refs.map((one) => {
    if (!one.group) throw new Error(`置き場所の無い行は URL に書けません: ${one.name}`);
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
