// teams/canvas/prompt.tsx — the prompt a member (or a worker) is handed
// （spec-teams B7、r-4cf9、n-56f7、d-b87a）。
//
// The lines are hergy's own, cut into the segments r-4cf9 names and where each
// came from (spec-boundary 5). The screen lays them out and says where a folded
// document is read; it builds no line.
import type { ComponentChildren, JSX } from 'preact';
import { promptBlockedOf, promptsOf, workerPrompt } from '../state.js';
import type { Member, PromptSegment, Rec, Worker, WorkerPrompt as WorkerPromptText } from '../state.js';

// lineAtMaker reads which line an offset of the whole prompt stands on.
function lineAtMaker(full: string): (off: number) => number {
  const starts = [0];
  for (let i = 0; i < full.length; i++) if (full[i] === '\n') starts.push(i + 1);
  return (off) => {
    let lo = 0, hi = starts.length - 1;
    while (lo < hi) { const mid = (lo + hi + 1) >> 1; if (starts[mid] <= off) lo = mid; else hi = mid - 1; }
    return lo;
  };
}

// Block is one segment's lines: the segment and the half-open range [a, b) of
// the whole prompt it stands for.
type Block = { seg: PromptSegment; a: number; b: number };

function promptBlocks(segments: PromptSegment[]): { lines: string[]; blocks: Block[] } {
  const full = segments.map((x) => x.text).join('');
  const lines = full.split('\n');
  const at = lineAtMaker(full);
  const out: Block[] = [];
  let off = 0;
  segments.forEach((seg) => {
    const len = seg.text.length;
    if (len) {
      let a = at(off), b = at(off + len - 1) + 1;
      // The blank lines a segment starts with belong to the block before it:
      // a folded document starts at its first line, not at the gap above it.
      const lead = (seg.text.match(/^\n+/) || [''])[0].length;
      if (lead && out.length && a === out[out.length - 1].b && at(off + lead - 1) + 1 > a) {
        out[out.length - 1].b = at(off + lead - 1) + 1;
        a = out[out.length - 1].b;
      }
      if (out.length && a < out[out.length - 1].b) a = out[out.length - 1].b;
      if (a < b) out.push({ seg, a, b });
    }
    off += len;
  });
  // The blank lines between two segments belong to the block before them:
  // every line of the prompt is held by one block, so a fold's range covers
  // them and nothing is left between two blocks.
  for (let i = 1; i < out.length; i++) if (out[i].a > out[i - 1].b) out[i - 1].b = out[i].a;
  if (out.length) out[out.length - 1].b = Math.max(out[out.length - 1].b, lines.length);
  return { lines, blocks: out };
}

// PLine marks the heading marks of a prompt line (## 名簿).
function PLine({ text }: { text: string }): JSX.Element | string {
  const m = String(text).match(/^(#{1,6} )(.*)$/);
  if (m) return <><span class="cv-mm">{m[1]}</span><span class="cv-ph">{m[2]}</span></>;
  return text;
}

type PLinesProps = { lines: string[]; a: number; b: number; slot?: (t: string) => ComponentChildren };

function PLines({ lines, a, b, slot }: PLinesProps): JSX.Element[] {
  return lines.slice(a, b).map((t, k) => (
    <div class="yl" data-ln={a + k}>
      <span class="lno">{a + k + 1}</span>
      <span class="yt">{slot ? slot(t) : <PLine text={t} />}</span>
    </div>
  ));
}

const prange = (a: number, b: number): string => (b - a > 1 ? `${a + 1}–${b}` : `${a + 1}`);
const pcount = (lines: string[], a: number, b: number): number => lines.slice(a, b).filter((x) => x !== '').length;

function TemplateChip({ seg }: { seg: PromptSegment }): JSX.Element {
  const o = seg.origin;
  if (o.kind === 'team') return <><span class="cv-ptpl own">ひな形: チームの prompt:</span><button class="cv-go" type="button" data-cv-act="to-yaml">YAML で直す</button></>;
  if (o.kind === 'member') return <><span class="cv-ptpl own">ひな形: メンバー {o.name || ''} の prompt:</span><button class="cv-go" type="button" data-cv-act="to-yaml">YAML で直す</button></>;
  return <><span class="cv-ptpl">ひな形: hergy の既定</span></>;
}

const PLABEL: Record<string, string> = { roster: '名簿', turns: 'ターンを渡す相手', workers: 'ワーカーの追加' };

// d-20ac: a definition with a problem is not given a prompt, by hergy up or by
// this column. The words are the screen's own; the problem itself is the
// server's, under the canvas.
const PROMPT_BLOCKED = 'このチーム定義には問題があるので、hergy はプロンプトを組みません（hergy up も止まります）。下の validate の欄の問題を直すと、ここに出ます。';

export function MemberPrompt({ r, m }: { r: Rec; m: Member }): JSX.Element {
  const hit = (promptsOf(r) || []).find((x) => x.name === m.name);
  let body: ComponentChildren;
  if (promptBlockedOf(r)) body = <span class="cv-note">{PROMPT_BLOCKED}</span>;
  else if (!hit) body = <span class="cv-note">プロンプトを読んでいます。</span>;
  else {
    const { lines, blocks } = promptBlocks(hit.segments || []);
    let head = false, labelled = '';
    body = blocks.map(({ seg, a, b }) => {
      const o = seg.origin;
      if (seg.kind === 'role_doc') {
        return <div class="cv-fold" data-a={a} data-b={b} data-doc="role" data-role={o.name || ''} data-place={o.place || ''}><span class="cv-range">{prange(a, b)}</span><b>ロールの文書</b><code>{o.path || ''}</code><span>{pcount(lines, a, b)} 行</span><a href={`/roles?open=${encodeURIComponent(o.name || '')}`}>ロールの画面で見る</a></div>;
      }
      if (seg.kind === 'team_rules') {
        return <div class="cv-fold" data-a={a} data-b={b} data-doc="team"><span class="cv-range">{prange(a, b)}</span><b>チームのルール</b><span>{pcount(lines, a, b)} 行</span><button class="cv-go" type="button" data-cv-act="to-agree">ルールのタブで見る</button></div>;
      }
      if (seg.kind === 'herdr') {
        return <div class="cv-fold cv-fixed" data-a={a} data-b={b}><span class="cv-range">{prange(a, b)}</span><details><summary>連絡のしかた<span> · hergy の決まった文 · どのチームでも同じ</span></summary><PLines lines={lines} a={a} b={b} /></details></div>;
      }
      let label: ComponentChildren = '';
      // The heading of a section ("## 名簿") stands in the template just
      // before the section's own segment: the label goes above the heading,
      // and the segment that follows does not say it a second time.
      const heading = (lines.slice(a, b).find((x) => /^##\s/.test(x)) || '').replace(/^##\s*/, '');
      if (seg.kind === 'template' || seg.kind === 'actor') {
        if (!head) { head = true; label = <div class="cv-pbl"><b>冒頭</b><TemplateChip seg={seg} /></div>; }
        const named = Object.values(PLABEL).find((x) => x === heading);
        if (named) { label = <>{label}<div class="cv-pbl"><b>{named}</b></div></>; labelled = named; }
      } else if (PLABEL[seg.kind]) {
        label = labelled === PLABEL[seg.kind] ? '' : <div class="cv-pbl"><b>{PLABEL[seg.kind]}</b></div>;
        labelled = PLABEL[seg.kind];
      }
      return <>{label}<PLines lines={lines} a={a} b={b} /></>;
    });
  }
  return <details class="cv-sec cv-prompt"><summary class="cv-sl">プロンプト（このチーム定義から渡る行）</summary><div class="cv-pr">{body}</div></details>;
}

export function WorkerPromptSection({ r, w, i }: { r: Rec; w: Worker; i: number }): JSX.Element {
  if (promptBlockedOf(r)) return <details class="cv-sec cv-prompt"><summary class="cv-sl">プロンプト（hire のときに渡る文）</summary><span class="cv-note">{PROMPT_BLOCKED}</span></details>;
  if (!w.purpose) return <details class="cv-sec cv-prompt"><summary class="cv-sl">プロンプト（hire のときに渡る文）</summary><span class="cv-note">目的を書くと、hire のときに渡る文が出ます。</span></details>;
  const pv: WorkerPromptText | null = workerPrompt(r, i);
  let body: ComponentChildren;
  if (!pv) body = <span class="cv-note">プロンプトを読んでいます。</span>;
  else if (pv.err) body = <span class="cv-note">{pv.err}</span>;
  else {
    const { lines, blocks } = promptBlocks(pv.segments || []);
    const full = `${r.name.value}-${pv.name}`;
    // A worker's name and task are chosen at hire, so the lines show the slots
    // they stand in (<ワーカー名> / <依頼>) instead.
    const slotText = (t: string): ComponentChildren => {
      const out: ComponentChildren[] = [];
      let s = String(t);
      for (;;) {
        const a = s.indexOf(full), b = s.indexOf(pv.task);
        const at = a < 0 ? b : b < 0 ? a : Math.min(a, b);
        if (at < 0) { out.push(s); return out; }
        out.push(s.slice(0, at));
        if (at === a) {
          out.push(`${r.name.value}-`);
          out.push(<span class="cv-slot" data-v={pv.name}>{'<ワーカー名>'}</span>);
          s = s.slice(at + full.length);
        } else {
          out.push(<span class="cv-slot" data-v={pv.task}>{'<依頼>'}</span>);
          s = s.slice(at + pv.task.length);
        }
      }
    };
    const agreeAt = blocks.findIndex(({ seg, a, b }) => seg.kind === 'template' && lines.slice(a, b).some((x) => x.startsWith('## 依頼')));
    let herdrSeen = false;
    body = blocks.map(({ seg, a, b }, n) => {
      if (seg.kind === 'herdr') {
        herdrSeen = true;
        return <div class="cv-fold cv-fixed" data-a={a} data-b={b}><span class="cv-range">{prange(a, b)}</span><details><summary>連絡のしかた<span> · hergy の決まった文 · どのワーカーでも同じ</span></summary><PLines lines={lines} a={a} b={b} slot={slotText} /></details></div>;
      }
      let label: ComponentChildren = '';
      if (n === 0) label = <div class="cv-pbl"><b>冒頭</b><span class="cv-ptpl">ひな形: hergy の既定</span></div>;
      else if (n === agreeAt) label = <div class="cv-pbl"><b>依頼</b><span>hire の --task</span></div>;
      else if (herdrSeen) label = <div class="cv-pbl"><b>報告</b><span>追加したメンバーへ</span></div>;
      return <>{label}<PLines lines={lines} a={a} b={b} slot={slotText} /></>;
    });
  }
  return <details class="cv-sec cv-prompt"><summary class="cv-sl">プロンプト（hire のときに渡る文）</summary><div class="cv-pr">{body}</div></details>;
}
