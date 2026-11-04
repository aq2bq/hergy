import { useEffect, useRef, useState } from './preact.js';
import { blocks } from './prose-blocks.js';
import { blockRange, transferDraft, saveTransfer } from './prose-transfer.js';
import type { TransferDocument, TransferSource, TransferTarget } from './prose-transfer.js';
import { problemText } from './api.js';

type Choice = { target?: string; at?: number; move?: boolean; children?: boolean };
let dragged: { source: string; open: (choice: Choice) => void; children: boolean } | null = null;
export function dropTransfer(event: DragEvent, target: string, at: number) {
 if (!dragged || dragged.source === target) return;
 event.preventDefault(); event.stopPropagation();
 dragged.open({ target, at, move: !event.altKey, children: dragged.children }); dragged = null;
}
export function TransferGrip({ source, index: _index, open }: { source: TransferSource; index: number; open: (choice: Choice) => void }) {
 const label = 'つかんで移す（Altで写す・Shiftで子見出しを含める）';
 return <button type="button" class="btn sm prose-grip" draggable title={label} aria-label={label} onClick={() => open({})} onDragStart={(event) => {
  dragged = { source: source.id, open, children: event.shiftKey };
  if (event.dataTransfer) { event.dataTransfer.effectAllowed = 'copyMove'; event.dataTransfer.setData('text/plain', 'hergy-heading'); }
 }} onDragEnd={() => { dragged = null; }}><svg viewBox="0 0 16 20" aria-hidden="true"><path d="M5 4h1M10 4h1M5 10h1M10 10h1M5 16h1M10 16h1" /></svg></button>;
}
type Prepared = { source: string; destination: string; next: {source: string; destination: string} };
export function TransferMenu({ source, index, target: initialTarget, at: initialAt, move: initialMove, children: initialChildren, close }: { source: TransferSource; index: number; close: () => void } & Choice) {
 const dialog = useRef<HTMLDialogElement>(null), request = useRef(0);
 const [original] = useState(source.text());
 const [targets, setTargets] = useState<TransferTarget[]>([]), [target, setTarget] = useState<TransferDocument | null>(null);
 const [selected, setSelected] = useState(initialTarget || ''), [at, setAt] = useState(initialAt ?? -1);
 const [children, setChildren] = useState(!!initialChildren), [move, setMove] = useState(!!initialMove);
 const [prepared, setPrepared] = useState<Prepared | null>(null), [busy, setBusy] = useState(false), [error, setError] = useState(''), [done, setDone] = useState('');
 const parts = blocks(original), [start, end] = blockRange(parts, index, children);
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
    setDone(result === 'source-failed' ? '宛先は保存しましたが、元を保存できませんでした。本文は両方のファイルに残っています。元の文書の競合や検査結果を確認してください。' : move ? '宛先、元の順に保存しました。' : '写した先を保存しました。');
   }
  } finally { setBusy(false); }
 }
 return <dialog class="prose-transfer" ref={dialog} aria-label="見出しを写す・移す" onCancel={(e) => { e.preventDefault(); cancel(); }}>
  <h2>見出しを写す・移す</h2><p>{source.label} / {parts[index].heading}</p>
  {done ? <p role="status">{done}</p> : <>
   <fieldset disabled={busy || !!prepared}>
    <label>操作<select aria-label="操作" value={move ? 'move' : 'copy'} onChange={(e) => setMove(e.currentTarget.value === 'move')}><option value="copy">写す（元に残す）</option><option value="move">移す</option></select></label>
    <label><input type="checkbox" checked={children} onChange={(e) => setChildren(e.currentTarget.checked)} />子見出しを含める</label>
    <label>宛先<select aria-label="宛先" value={selected} onChange={(e) => { setSelected(e.currentTarget.value); setAt(-1); setError(''); }}><option value="">選んでください</option>{targets.map((t) => <option value={t.id}>{t.label}</option>)}</select></label>
    <label>挿入位置<select aria-label="挿入位置" value={at} disabled={!target} onChange={(e) => setAt(Number(e.currentTarget.value))}><option value={-1}>末尾</option>{target ? blocks(target.text()).map((part) => <option value={part.start}>{part.start === 0 ? '先頭' : part.heading + ' の前'}</option>) : null}</select></label>
   </fieldset>
   <p>空白と改行をそのまま写します。共有ロールへの保存は、その文書を読むすべてのチームに反映されます。</p>
   {prepared ? <div class="transfer-preview"><div><b>元の下書き</b><pre>{prepared.next.source}</pre></div><div><b>宛先の下書き</b><pre>{prepared.next.destination}</pre></div></div> : <pre class="transfer-excerpt">{original.slice(start, end)}</pre>}
   {error ? <p role="alert">{error}</p> : null}
   <footer>{prepared ? <><button class="btn primary" disabled={busy} onClick={() => void persist()}>{busy ? '保存中…' : move ? '宛先から順に保存' : '写した先を保存'}</button><button class="btn" disabled={busy} onClick={undo}>元に戻す</button></> : <button class="btn primary" disabled={!target} onClick={prepare}>下書きに反映</button>}</footer>
  </>}
  <button class="btn" disabled={busy} onClick={cancel}>{done ? '閉じる' : '取り消して閉じる'}</button>
 </dialog>;
}
