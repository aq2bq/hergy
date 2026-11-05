import { useEffect, useMemo, useRef, useState } from '../../lib/preact.js';
import { call, problemText } from '../../lib/api.js';
import type { Schemas } from '../../lib/api.js';
import { resource } from '../../lib/resource.js';
import { CurrentPrompt, OriginLegend, OriginMarks, originLines, sectionAt, sectionSpans } from './prompt-origins.js';
import type { OriginLine } from './prompt-origins.js';
import { dkey, promptComparisonInput } from '../state.js';
import type { Rec } from '../state.js';
import { runOf, setRun } from './prompt-run.js';

type View = 'recorded' | 'current' | 'diff';
type Answer = Schemas['PromptComparison'];

export function PromptComparison({ r, member, close }: {r: Rec; member: string; close: () => void}) {
 const dialog = useRef<HTMLDialogElement>(null);
 const key = dkey(r);
 // The team the person picked stands in the shared signal: the prompt column
 // builds with the same name (r-0b69).
 const [running, setRunning] = useState(runOf(key));
 const [view, setView] = useState<View>('diff');
 const initialized = useRef(false);
 const reading = useMemo(() => resource(async (signal: AbortSignal, payload: Schemas['PromptComparisonRequest']) => {
  const answer = await call<Answer>('POST', '/api/team/prompt-comparison', payload, {signal});
  if (!answer.json) throw new Error('本文の応答がありません');
  return answer.json;
 }), [r, member]);
 const payload = JSON.stringify({...promptComparisonInput(r), member, running_team: running});
 const load = () => reading.run(JSON.parse(payload)).then((answer) => {
  if (answer && !initialized.current) { setView(answer.recorded ? 'diff' : 'current'); initialized.current = true; }
 });
 useEffect(() => {
  const trigger = document.activeElement as HTMLElement | null;
  dialog.current?.showModal();
  return () => { reading.abort(); dialog.current?.close(); trigger?.focus(); };
 }, [reading]);
 useEffect(() => { void load(); return () => reading.abort(); }, [reading, payload]);
 const answer = reading.value.value;
 // A pick that is gone from the runs is let go, and a single running team is
 // taken the way the server took it, so the prompt column reads the same name.
 useEffect(() => {
  if (!answer) return;
  if (runOf(key) && !answer.runs.some(run => run.team === runOf(key))) { setRun(key, ''); setRunning(''); return; }
  if (!runOf(key) && answer.selected) setRun(key, answer.selected.team);
 }, [answer, key]);
 const lines = useMemo(() => answer?.current !== null && answer?.current !== undefined ? originLines(answer.current, answer.segments) : null, [answer]);
 const busy = reading.loading.value;
 const error = reading.error.value;
 return <dialog class="prompt-comparison" ref={dialog} aria-labelledby="prompt-comparison-title" onKeyDown={(event) => event.stopPropagation()} onCancel={(event) => {event.preventDefault(); close();}}>
  <header><h2 id="prompt-comparison-title">プロンプト · {r.name.value} / {member}</h2><button class="btn sm" type="button" onClick={close}>閉じる</button></header>
  <div class="prompt-controls">
   {!!answer?.runs.length && <label>実行中のチーム <select value={running || answer.selected?.team || ''} onChange={(event) => { initialized.current = false; setRunning(event.currentTarget.value); setRun(key, event.currentTarget.value); }}>
    <option value="">チームを選ぶ</option>{answer.runs.map((run) => <option key={run.team} value={run.team}>{run.team} · {run.project}</option>)}
   </select></label>}
   <button class="btn sm" type="button" disabled={busy} onClick={() => {void load();}}>再取得</button>
  </div>
  <nav class="prompt-tabs" aria-label="プロンプトの表示">{([['recorded','渡した文'],['current','今の文'],['diff','差']] as const).map(([key,label]) => <button class="btn sm" type="button" aria-pressed={view===key} onClick={() => {initialized.current = true; setView(key);}}>{label}</button>)}</nav>
  {busy ? <p role="status">本文を読んでいます。</p> : error ? <p role="alert">{problemText(error, '本文を読めませんでした')}</p> : answer && <>
   <div class="prompt-context">
    {answer.reason && <p role="status">{answer.reason}</p>}
    {answer.recorded && <><p>最後に記録された本文 · {answer.recorded.command} · <time dateTime={answer.recorded.time}>{answer.recorded.time}</time></p><p>反映・送り直しの変更理由の前置きは記録に含まれません。送信後の記録であり、受信の確認ではありません。送信の途中や記録に失敗した場合は、実際の送信より古い記録です。</p></>}
    <p>{answer.draft ? '未保存の編集を含む' : '保存済みの定義'} · {answer.team} · {answer.language}</p>
    <p class="prompt-environment" title={`${answer.project} · ${answer.executable}`}>生成条件: {answer.project} · {answer.executable}</p>
    {answer.problems.map((problem) => <p class={problem.sev==='err' ? 'err' : ''}>{problem.text}</p>)}
   </div>
   {view !== 'recorded' && <OriginLegend diff={view === 'diff'} />}
   <div class="prompt-body" tabIndex={0} aria-label={view==='recorded' ? '渡した文の全文' : view==='current' ? '今の文の全文' : '本文の差'}>
    {view==='recorded' && (answer.recorded ? <FullText text={answer.recorded.text}/> : <p>渡した文を表示できません。</p>)}
    {view==='current' && (answer.current!==null ? <CurrentPrompt text={answer.current} lines={lines}/> : <p>定義に問題があるため、今の文を組み立てられません。</p>)}
    {view==='diff' && <Difference answer={answer} lines={lines}/>}
   </div>
  </>}
 </dialog>;
}
function FullText({text}: {text: string}) {
 return <><pre class="prompt-full">{text}</pre>{text && !text.endsWith('\n') && <p class="prompt-eof">末尾に改行はありません</p>}</>;
}
function Difference({answer, lines}: {answer: Answer; lines: OriginLine[] | null}) {
 if (!answer.recorded || answer.current===null) return <p>両方の本文がそろわないため、比較できません。</p>;
 // The band follows the current text's lines: unchanged and added rows stand
 // on them, and a removed row between them stays in the band without a mark
 // (r-04fc, d-2f04). The first and last rows of the band are the rows the
 // current text still has.
 const spans = lines ? sectionSpans(lines).map(span => {
  const present = answer.diff.flatMap(line => (line.new && line.new - 1 >= span.start && line.new - 1 < span.end ? [line.new - 1] : []));
  return { ...span, first: present.length ? Math.min(...present) : -1, last: present.length ? Math.max(...present) : -1 };
 }) : [];
 // gap is the band a removed row stands inside: the nearest rows the current
 // text still has above and below it, both in one band.
 const gap = (() => {
  const before: number[] = [], after: number[] = new Array(answer.diff.length).fill(-1);
  let seen = -1;
  answer.diff.forEach((line, i) => { before[i] = seen; if (line.new) seen = line.new - 1; });
  seen = -1;
  for (let i = answer.diff.length - 1; i >= 0; i--) { after[i] = seen; if (answer.diff[i].new) seen = answer.diff[i].new - 1; }
  return answer.diff.map((line, i) => {
   if (line.new || before[i] < 0 || after[i] < 0) return undefined;
   return spans.find(span => span.first >= 0 && before[i] >= span.first && after[i] <= span.last && after[i] === before[i] + 1);
  });
 })();
 return <>{!lines && <p role="alert">本文と由来が一致しないため、印を表示できません。再取得してください。</p>}{answer.recorded.text===answer.current && <p>差はありません。</p>}{answer.diff.map((line,i) => {
  const at = line.new ? line.new - 1 : -1;
  const span = at >= 0 ? sectionAt(spans, at) : gap[i];
  return <div key={i} class={`prompt-diff-line ${line.kind}`} data-new-line={line.new || undefined} data-section={span?.kind} data-section-start={span && at >= 0 && span.first === at ? '' : undefined} data-section-end={span && at >= 0 && span.last === at ? '' : undefined}>
   <span class="prompt-line-number" aria-label="記録の行">{line.old || ''}</span><span class="prompt-line-number" aria-label="今の行">{line.new || ''}</span><span aria-label={line.kind==='add' ? '追加' : line.kind==='remove' ? '削除' : '変更なし'}>{line.kind==='add' ? '+' : line.kind==='remove' ? '−' : ' '}</span><OriginMarks segments={line.new ? lines?.[line.new - 1]?.segments || [] : []}/><div><pre>{line.text}</pre>{!line.text.endsWith('\n') && <span class="prompt-eof">末尾に改行はありません</span>}</div>
  </div>;
 })}</>;
}
