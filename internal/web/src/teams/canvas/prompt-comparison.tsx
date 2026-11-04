import { useEffect, useMemo, useRef, useState } from '../../lib/preact.js';
import { call, problemText } from '../../lib/api.js';
import type { Schemas } from '../../lib/api.js';
import { resource } from '../../lib/resource.js';
import { CurrentPrompt, OriginLegend, OriginMarks, originLines } from './prompt-origins.js';
import type { OriginLine } from './prompt-origins.js';
import { promptComparisonInput } from '../state.js';
import type { Rec } from '../state.js';

type View = 'recorded' | 'current' | 'diff';
type Answer = Schemas['PromptComparison'];

export function PromptComparison({ r, member, close }: {r: Rec; member: string; close: () => void}) {
 const dialog = useRef<HTMLDialogElement>(null);
 const [running, setRunning] = useState('');
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
 const lines = useMemo(() => answer?.current !== null && answer?.current !== undefined ? originLines(answer.current, answer.segments) : null, [answer]);
 const busy = reading.loading.value;
 const error = reading.error.value;
 return <dialog class="prompt-comparison" ref={dialog} aria-labelledby="prompt-comparison-title" onKeyDown={(event) => event.stopPropagation()} onCancel={(event) => {event.preventDefault(); close();}}>
  <header><h2 id="prompt-comparison-title">プロンプト · {r.name.value} / {member}</h2><button class="btn sm" type="button" onClick={close}>閉じる</button></header>
  <div class="prompt-controls">
   {!!answer?.runs.length && <label>実行中のチーム <select value={running || answer.selected?.team || ''} onChange={(event) => { initialized.current = false; setRunning(event.currentTarget.value); }}>
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
 return <>{!lines && <p role="alert">本文と由来が一致しないため、印を表示できません。再取得してください。</p>}{answer.recorded.text===answer.current && <p>差はありません。</p>}{answer.diff.map((line,i) => <div key={i} class={`prompt-diff-line ${line.kind}`} data-new-line={line.new || undefined}>
  <span class="prompt-line-number" aria-label="記録の行">{line.old || ''}</span><span class="prompt-line-number" aria-label="今の行">{line.new || ''}</span><span aria-label={line.kind==='add' ? '追加' : line.kind==='remove' ? '削除' : '変更なし'}>{line.kind==='add' ? '+' : line.kind==='remove' ? '−' : ' '}</span><OriginMarks segments={line.new ? lines?.[line.new - 1]?.segments || [] : []}/><div><pre>{line.text}</pre>{!line.text.endsWith('\n') && <span class="prompt-eof">末尾に改行はありません</span>}</div>
 </div>)}</>;
}
