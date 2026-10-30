// lib/conflict.tsx — 保存を断られたときの知らせ（r-3c77、d-a34a）。
//
// ファイルがこの画面で読んだあとにほかのところで変わっていたら、保存は断られ、
// その答えにファイルの今の様子と、この保存が書き込もうとした文が入っている
// （internal/web/changed.go）。画面は書いていた内容を捨てずに、この部品を保存の
// 帯のすぐ上に出す。人が選べるのは 3 つ:
//
//   見比べる                    ファイルの今の文と、この画面で読んだ文を YAML の行で並べる（読むだけ）
//   書いていた内容で上書きする   見たハッシュを付けて同じ保存を送り直す
//   元に戻す                    ファイルの今の文を読み込み、取り消すで書いていた内容に戻せる
//
// ファイルが消えていれば、上書きの代わりに 書いていた内容で作り直して保存する。
// どの画面もこの 1 つの部品を出し、押されたときの保存と読み込みだけを自分で持つ。
import { useState } from './preact.js';

// Changed is the file a refusal is about, as the server answered it: where it
// stands, whether it is there, the hash of the text it holds now, that text, the
// text the save would have written over it, the hour it was written, and the file
// as the kind reads it (what 元に戻す puts in place).
export type Changed = {
  path: string;
  exists: boolean;
  hash?: string;
  text?: string;
  changed?: string;
  doc?: unknown;
};

// Conflict is one refusal that names its file, with the sentence a person reads.
export type Conflict = { text: string; file: Changed };

const str = (v: unknown): string => (typeof v === 'string' ? v : '');

// conflictOf reads a refusal as the file it is about, or null when the refusal
// is about something else (a name that is taken, a problem in the draft).
export function conflictOf(err: unknown): Conflict | null {
  if (typeof err !== 'object' || err === null || !('json' in err)) return null;
  const json = (err as { json?: unknown }).json;
  if (typeof json !== 'object' || json === null) return null;
  const one = json as { text?: unknown; file?: unknown };
  const file = one.file;
  if (typeof one.text !== 'string' || typeof file !== 'object' || file === null) return null;
  const f = file as Record<string, unknown>;
  if (typeof f.path !== 'string' || typeof f.exists !== 'boolean') return null;
  return {
    text: one.text,
    file: {
      path: f.path,
      exists: f.exists,
      hash: str(f.hash),
      text: str(f.text),
      changed: str(f.changed),
      doc: f.doc,
    },
  };
}

// Row is one line of the compare: the file's line and the draft's line. A row
// with one side empty is a line only that side holds.
type Row = { n: number; file: string | null; draft: string | null };

// rowsOf is the lines the two texts do not share, in order: the file's text on
// the left and the draft's on the right. The compare reads them side by side and
// nothing else: no side is chosen here (r-3c77).
export function rowsOf(file: string, draft: string): Row[] {
  const a = lines(file), b = lines(draft);
  const lcs: number[][] = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0));
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      lcs[i][j] = a[i] === b[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1]);
    }
  }
  const out: Row[] = [];
  let i = 0, j = 0;
  let n = 1;
  while (i < a.length || j < b.length) {
    if (i < a.length && j < b.length && a[i] === b[j]) { i++; j++; n++; continue; }
    if (j < b.length && (i === a.length || lcs[i][j + 1] >= lcs[i + 1][j])) { out.push({ n, file: null, draft: b[j] }); j++; continue; }
    out.push({ n, file: a[i], draft: null });
    i++;
    n++;
  }
  return out;
}

function lines(text: string): string[] {
  const all = (text || '').replace(/\n$/, '');
  return all === '' ? [] : all.split('\n');
}

// ConflictNotice is the notice above a panel's save band: the sentence the
// server wrote, the compare, and the three choices. The draft stays where it is
// while this stands (d-a34a).
//
// 見比べる reads the file as it is now beside the text this screen read: the
// lines that differ are the file's own change, which is what a person needs
// before writing over it. What they wrote is on the screen above, untouched
// (r-3c77).
export function ConflictNotice({ c, mine, busy, undone, onOverwrite, onRevert, onUndo }: {
  c: Conflict;
  mine: string;
  busy?: boolean;
  undone?: boolean;
  onOverwrite: () => void;
  onRevert: () => void;
  onUndo: () => void;
}) {
  const [shown, setShown] = useState(false);
  if (undone) {
    return (
      <section class="conflict" data-conflict={c.file.path} data-state="undone" role="status">
        <p class="cf-text">ファイルの今の内容を読み込みました</p>
        <div class="cf-acts"><button class="btn sm" data-act="cf-undo" onClick={onUndo}>取り消す</button></div>
      </section>
    );
  }
  const rows = shown ? rowsOf(c.file.text || '', mine) : [];
  return (
    <section class="conflict" data-conflict={c.file.path} data-exists={c.file.exists ? '1' : '0'} role="alert">
      <p class="cf-text">{c.text}</p>
      {shown ? (
        <div class="cf-compare">
          <p class="cf-note">書いていた内容は、この画面に残っています。違う行だけを並べます</p>
          {rows.length === 0 ? <p class="cf-none">この画面で読んだときと、違う行はありません</p> : (
            <table>
              <thead><tr><th>行</th><th>ファイルの今の内容</th><th>この画面で読んだ内容</th></tr></thead>
              <tbody>
                {rows.slice(0, 200).map((row) => (
                  <tr class={row.file === null ? 'cf-add' : row.draft === null ? 'cf-del' : 'cf-both'}><th>{row.n}</th><td>{row.file ?? ''}</td><td>{row.draft ?? ''}</td></tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      ) : null}
      <div class="cf-acts">
        <button class="btn sm" data-act="cf-compare" aria-pressed={shown} onClick={() => setShown(!shown)}>{shown ? '見比べるのをやめる' : '見比べる'}</button>
        <button class="btn primary sm" data-act="cf-overwrite" disabled={!!busy} onClick={onOverwrite}>{c.file.exists ? '書いていた内容で上書きする' : '書いていた内容で作り直して保存する'}</button>
        <button class="btn sm" data-act="cf-revert" disabled={!!busy} onClick={onRevert}>元に戻す</button>
      </div>
    </section>
  );
}
