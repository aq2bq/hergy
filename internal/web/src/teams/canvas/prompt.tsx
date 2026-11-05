// teams/canvas/prompt.tsx — the prompt a member (or a worker) is handed
// （spec-teams B7、r-4cf9、n-56f7、d-b87a）。
//
// The lines are hergy's own, cut into the segments r-4cf9 names and where each
// came from (spec-boundary 5). The screen lays them out and says where a folded
// document is read; it builds no line.
import type { ComponentChildren, JSX } from 'preact';
import { useEffect, useMemo, useState } from '../../lib/preact.js';
import { call, problemText } from '../../lib/api.js';
import type { Schemas } from '../../lib/api.js';
import { resource } from '../../lib/resource.js';
import { PromptComparison } from './prompt-comparison.js';
import { topologySections } from './prompt-origins.js';
import { dkey, memberOf, promptBlockedOf, promptComparisonInput, promptsOf, workerPrompt } from '../state.js';
import type { Member, PromptSegment, Rec, Worker, WorkerPrompt as WorkerPromptText } from '../state.js';
import { selOf } from './shared.js';
import { keepRun, runOf } from './prompt-run.js';

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
// d-20ac: a definition with a problem is not given a prompt, by hergy up or by
// this column. The words are the screen's own; the problem itself is the
// server's, under the canvas.
const PROMPT_BLOCKED = 'このチーム定義には問題があるので、hergy はプロンプトを組みません（hergy up も止まります）。下の設定の検査の欄の問題を直すと、ここに出ます。';

// MemberPrompt is the プロンプト tab's member text (r-1af1, r-f0be). It is built
// where the whole-text panel builds it, with the same running team, so the same
// member name stands in both (r-0b69): the comparison API answers the segments
// and the name it used.
export function MemberPrompt({ r, m }: { r: Rec; m: Member }): JSX.Element {
  const [opened, setOpened] = useState(false);
  const key = dkey(r);
  const blocked = promptBlockedOf(r);
  // tick is the last answered check: the text follows unsaved edits at the
  // check's own pace, not at every keystroke (r-0b69).
  const tick = r.check.value;
  const reading = useMemo(() => resource(async (signal: AbortSignal, payload: Schemas['PromptComparisonRequest']) => {
    const answer = await call<Schemas['PromptComparison']>('POST', '/api/team/prompt-comparison', payload, { signal });
    if (!answer.json) throw new Error('本文の応答がありません');
    return answer.json;
  }), [r, m.name]);
  useEffect(() => {
    if (blocked) { reading.abort(); return; }
    void reading.run({ ...promptComparisonInput(r), member: m.name, running_team: runOf(key) });
    return () => reading.abort();
  }, [reading, key, m.name, tick, blocked, runOf(key)]);
  const answer = reading.value.value;
  const failure = reading.error.value;
  // The same follow-up the whole-text panel makes: a picked team that stopped
  // is let go, a single running team is taken (r-0b69).
  useEffect(() => { if (answer) keepRun(key, answer); }, [answer, key]);
  // A text the server could not build with a running team's name falls back to
  // the check's own segments, which stand on the definition's name.
  const fallback = promptsOf(r).find(item => item.name === m.name);
  const segments = answer ? answer.segments : (fallback?.segments || []);
  const shown = answer ? answer.team : r.name.value;
  const name = r.name.value;
  const runs = (answer && answer.runs) || [];
  const note = failure
    // A failed re-read keeps the text it already had: the note names the team
    // that text was built with, not the one that failed (r-0b69).
    ? answer
      ? `${problemText(failure, '本文を読めませんでした')} 直前に作った文は、${answer.selected ? `実行中のチーム ${answer.team}` : `定義名 ${answer.team}`} の名前で作ったものです。`
      : `${problemText(failure, '本文を読めませんでした')} そのため、定義名 ${name} で作っています。`
    : !answer ? ''
    : answer.status === 'error' ? `${answer.reason || 'チームの状態を読めませんでした'} そのため、定義名 ${name} で作っています。`
    : answer.selected ? `実行中のチーム ${shown} の名前で作っています。`
    : runs.length > 1 ? `動いているチームが ${runs.length} つあるため、定義名 ${name} で作っています。全文と差分のパネルでチームを選ぶと、その名前で作ります。`
    : runs.length === 0 ? `この定義から動いているチームが無いため、定義名 ${name} で作っています。`
    : `定義名 ${name} で作っています。`;
  return <section class="cv-sec cv-prompt cv-topology-prompt">
    <h3>このメンバーにトポロジーから渡る文</h3>
    <button class="btn sm cv-prompt-open" type="button" onClick={() => setOpened(true)}>全文と差分を読む</button>
    <p class="cv-note">今の定義から出る文。未保存の編集を含みます。{note}</p>
    {blocked ? <p class="cv-note">{PROMPT_BLOCKED} 記録された本文は確認できます。</p>
      : !answer && !fallback ? <p role="status">プロンプトを読んでいます。</p>
      : topologySections.map(([kind, label]) => {
        const parts = (segments || []).filter(segment => segment.kind === kind && segment.text !== '');
        if (kind === 'workers' && !parts.length) return null;
        return <section class="cv-topology-section" data-kind={kind} key={kind}><h4>{label}</h4>
          {parts.length ? parts.map((segment, i) => <pre class="cv-topology-text" key={i}>{segment.text}</pre>) : <p class="cv-note">この節は今のプロンプトに含まれません。</p>}
        </section>;
      })}
    {opened && <PromptComparison key={m.name} r={r} member={m.name} close={() => setOpened(false)} />}
  </section>;
}

export function WorkerPromptSection({ r, w, i }: { r: Rec; w: Worker; i: number }): JSX.Element {
  if (promptBlockedOf(r)) return <details class="cv-sec cv-prompt" open><summary class="cv-sl">プロンプト（hire のときに渡る文）</summary><span class="cv-note">{PROMPT_BLOCKED}</span></details>;
  if (!w.purpose) return <details class="cv-sec cv-prompt" open><summary class="cv-sl">プロンプト（hire のときに渡る文）</summary><span class="cv-note">目的を書くと、hire のときに渡る文が出ます。</span></details>;
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
  return <details class="cv-sec cv-prompt" open><summary class="cv-sl">プロンプト（hire のときに渡る文）</summary><div class="cv-pr">{body}</div></details>;
}


// PromptView is the プロンプト tab (r-f0be, d-87f5): what hergy hands the thing
// the person picked, and nothing when the pick is not something a prompt is
// built for. The attribute tab no longer carries these.
export function PromptView({ r }: { r: Rec }): JSX.Element {
  const sel = selOf(r);
  const m = sel && sel.type === 'node' && sel.id !== 'human' ? memberOf(r.draft.value, sel.id) : undefined;
  if (m) return <MemberPrompt r={r} m={m} />;
  if (sel && sel.type === 'worker') {
    const w = r.draft.value.workers[sel.i];
    if (w) return <WorkerPromptSection r={r} w={w} i={sel.i} />;
  }
  return <p class="cv-note cv-prompt-none">メンバーかワーカーを選ぶと、そのプロンプトがここに出ます。</p>;
}
