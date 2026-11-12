import { term } from './terms.generated.js';
import { useEffect, useRef, useState } from './preact.js';
import { blocks } from './prose-blocks.js';
import { blockRange, transferDraft, saveTransfer } from './prose-transfer.js';
import type { TransferDocument, TransferSource, TransferTarget } from './prose-transfer.js';
import { problemText } from './api.js';

type Choice = { target?: string; at?: number; move?: boolean; children?: boolean };
let dragged: { source: string; open: (choice: Choice) => void; children: boolean } | null = null;
export function dropTransfer(event: DragEvent, target: string, at: number) {
 if (!dragged) return;
 event.preventDefault(); event.stopPropagation();
 if (dragged.source === target) return;
 dragged.open({ target, at, move: !event.altKey, children: dragged.children }); dragged = null;
}
export function transferKeys() {
 const mac = /Mac/.test(navigator.platform);
 return `${mac ? '⇧' : 'Shift'} を押しながらつかむと子見出しも含める。${mac ? '⌥' : 'Alt'} を押しながら放すと写す。`;
}
export function TransferGrip({ source, index: _index, open }: { source: TransferSource; index: number; open: (choice: Choice) => void }) {
 const label = `つかんで移す。${transferKeys()}`;
 return <button type="button" class="prose-grip" draggable title={label} aria-label={label} onClick={() => open({})} onDragStart={(event) => {
  dragged = { source: source.id, open, children: event.shiftKey };
  if (event.dataTransfer) { event.dataTransfer.effectAllowed = 'copyMove'; event.dataTransfer.setData('application/x-hergy-heading', source.id); }
 }} onDragEnd={() => { dragged = null; }}><svg viewBox="0 0 16 20" aria-hidden="true"><path d="M5 4h1M10 4h1M5 10h1M10 10h1M5 16h1M10 16h1" /></svg></button>;
}
type Prepared = { source: string; destination: string; next: {source: string; destination: string} };
function ChangedText({ text, start, end, removed = false }: { text: string; start: number; end: number; removed?: boolean }) {
 const mark = useRef<HTMLElement>(null);
 useEffect(() => { const node = mark.current; if (node?.parentElement) node.parentElement.scrollTop = node.offsetTop - 40; }, []);
 return <pre>{text.slice(0, start)}<mark ref={mark} class={removed ? 'removed' : 'inserted'} aria-label={removed ? '削除する範囲' : '追加した範囲'}>{text.slice(start, end)}</mark>{text.slice(end)}</pre>;
}
export function TransferMenu({ source, index, target: initialTarget, at: initialAt, move: initialMove, children: initialChildren, close }: { source: TransferSource; index: number; close: () => void } & Choice) {
 const dialog = useRef<HTMLDialogElement>(null), request = useRef(0);
 const [original] = useState(source.text());
 const [targets, setTargets] = useState<TransferTarget[]>([]), [target, setTarget] = useState<TransferDocument | null>(null);
 const [selected, setSelected] = useState(initialTarget || ''), [at, setAt] = useState(initialAt ?? -1);
 const [children, setChildren] = useState(!!initialChildren), [move, setMove] = useState(!!initialMove);
 const [prepared, setPrepared] = useState<Prepared | null>(null), [busy, setBusy] = useState(false), [error, setError] = useState(''), [done, setDone] = useState('');
 const parts = blocks(original), [start, end] = blockRange(parts, index, children);
 const destination = prepared?.destination ?? target?.text() ?? '';
 const choice = targets.find((x) => x.id === selected);
 const groups = [...new Set(targets.map((t) => `${t.kind === 'team' ? term('team-definition') : term('role')} · ${t.group}`))];
 const insertion = at < 0 ? destination.length : at;
 useEffect(() => { dialog.current?.showModal(); source.targets().then((items) => setTargets(items.filter((x) => x.id !== source.id))).catch((err) => setError(problemText(err, '宛先を読めませんでした'))); }, []);
 useEffect(() => {
  const seq = ++request.current;
  setTarget(null);
  const choice = targets.find((x) => x.id === selected);
  if (choice) choice.load().then((doc) => { if (seq === request.current) setTarget(doc); }).catch((err) => { if (seq === request.current) setError(problemText(err, '宛先を読めませんでした')); });
 }, [selected, targets]);
 useEffect(() => {
  const warn = (event: BeforeUnloadEvent) => { if (prepared || busy) { event.preventDefault(); event.returnValue = ''; } };
  window.addEventListener('beforeunload', warn); return () => window.removeEventListener('beforeunload', warn);
 }, [prepared, busy]);
 function undo() {
  if (!prepared || !target) return;
  source.set(prepared.source); target.set(prepared.destination); setPrepared(null);
 }
 function cancel() { if (busy) return; undo(); close(); }
 function prepare() {
  if (!target) return;
  if (source.text() !== original) { setError('元の文書が変わりました。閉じて選び直してください'); return; }
  const destination = target.text();
  try {
   const next = transferDraft(original, destination, start, end, at < 0 ? destination.length : at, move);
   setPrepared({ source: original, destination, next }); source.set(next.source); target.set(next.destination); setError('');
  } catch (err) { setError(problemText(err, '下書きを作れませんでした')); }
 }
 async function persist() {
  if (!prepared || !target || busy) return;
  setBusy(true); setError('');
  try {
   let detail = '';
   const result = await saveTransfer(target, source, move, (err) => { detail = problemText(err, '保存に失敗しました'); });
   if (result === 'destination-failed') { setError('宛先を保存できませんでした。元のファイルは削除していません。下書きを元に戻し、宛先の競合や検査結果を確認してください。' + detail); }
   else {
    if (result === 'source-failed') source.set(prepared.source);
    setPrepared(null);
    setDone(result === 'source-failed' ? `${choice?.label || target.label} は保存しましたが、元を保存できませんでした。本文は両方のファイルに残っています。元の文書の競合や検査結果を確認してください。` : `${parts[index].heading} を ${choice?.label || target.label}へ${move ? '移して' : '写して'}保存しました。`);
   }
  } finally { setBusy(false); }
 }
 return <dialog class="prose-transfer" ref={dialog} aria-label="見出しを写す・移す" onCancel={(e) => { e.preventDefault(); cancel(); }}>
  <h2>見出しを写す・移す</h2><p>{source.label} / {parts[index].heading}</p>
  {done ? <p role="status">{done}</p> : <>
   <fieldset disabled={busy || !!prepared}>
    <label>操作<select aria-label="操作" value={move ? 'move' : 'copy'} onChange={(e) => setMove(e.currentTarget.value === 'move')}><option value="copy">写す（元に残す）</option><option value="move">移す</option></select></label>
    <label>宛先<select aria-label="宛先" value={selected} onChange={(e) => { setSelected(e.currentTarget.value); setAt(-1); setError(''); }}><option value="">選んでください</option>{groups.map((group) => <optgroup label={group}>{targets.filter((t) => `${t.kind === 'team' ? term('team-definition') : term('role')} · ${t.group}` === group).map((t) => <option value={t.id}>{t.label}</option>)}</optgroup>)}</select></label>
    <label>挿入位置<select aria-label="挿入位置" value={at} disabled={!target} onChange={(e) => setAt(Number(e.currentTarget.value))}><option value={-1}>末尾</option>{target ? blocks(destination).map((part) => <option value={part.start}>{'　'.repeat(Math.max(0, part.depth - 1)) + (part.start === 0 ? `題（${part.heading}）の前` : part.heading + ' の前')}</option>) : null}</select></label>
    <label class="transfer-children"><input type="checkbox" checked={children} onChange={(e) => setChildren(e.currentTarget.checked)} />子見出しを含める</label>
   </fieldset>
   <p>選んだ範囲の空白と改行はそのまま写し、境目に空行がないときだけ補います。{choice?.kind === 'role' ? `共有${term('role')}への保存は、その文書を読むすべての${term('team')}に${term('apply')}されます。` : ''}</p>
   {choice?.otherProject ? <p class="transfer-notice">{`宛先は、いま開いている${term('project')}とは別の場所です：`}{choice.label}</p> : null}
   {target && at === 0 && destination ? <p class="transfer-notice">題の前へ挿入すると、文書の題が変わります。</p> : null}
   {prepared ? <div class={'transfer-preview' + (move ? '' : ' copy')}>
    {move ? <div><b>元から削除する範囲</b><ChangedText text={prepared.source} start={start} end={end} removed /></div> : null}
    <div><b>宛先に追加した範囲</b><ChangedText text={prepared.next.destination} start={insertion} end={insertion + prepared.next.destination.length - prepared.destination.length} /></div>
   </div> : <pre class="transfer-excerpt">{original.slice(start, end)}</pre>}
   {error ? <p role="alert">{error}</p> : null}
  </>}
  <footer>
   <button class="btn" disabled={busy} onClick={cancel}>{done ? '閉じる' : '取り消して閉じる'}</button>
   {!done ? <span class="transfer-submit">{prepared ? <><button class="btn" disabled={busy} onClick={undo}>下書きをやめる</button><button class="btn primary" disabled={busy} onClick={() => void persist()}>{busy ? '保存中…' : move ? '宛先から順に保存' : '写した先を保存'}</button></> : <button class="btn primary" disabled={!target} onClick={prepare}>{`下書きに${term('apply')}`}</button>}</span> : null}
  </footer>
 </dialog>;
}
