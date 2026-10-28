// profiles/view.tsx — 一覧のタブ（spec 4 章、r-c683）。
//
// 行と、行の中の面（プロジェクト / グローバル）を描く。class と data-* は CSS
// と e2e が読むので、移す前のマークアップから 1 つも変えない（webui-preact-design
// 5 章 6）。ここは state.ts の signal を読んで描き、押し込みは自分の onClick /
// onInput から actions.ts の関数を直接呼ぶ（5 章 1）。DOM は自分では書かない
// （ドラッグの途中の class だけは、動かしている間の印として付ける）。
import { useEffect, useRef } from '../lib/preact.js';
import {
  rows, chips, shownRows, filterKind, query, groupName, currentProjectName, GROUPS,
  changedOf, saveBlocked, rowUnsaved, classOf, otherKinds, BASE_KINDS, labelOf,
  focusAsk, scrollAsk, GROUPED, panelByKey,
} from './state.js';
import {
  pressNew, pressFilter, pressQuery, pressToggle, pressSetKind, pressKindOther,
  pressArgAdd, pressArgDel, pressEnvAdd, pressEnvDel, pressSave, pressDup, pressCopy,
  pressDiscard, pressDel, pressDelNo, pressDelYes, pressRename, pressLayer, inputField,
  reorderArgs,
} from './actions.js';
import type { Place } from '../lib/api.js';
import type { ChangeSet, Panel as PanelState, Problem, ProfileChip, Row } from './types.js';

// inputValue is what a field holds after a person typed in it. A field is an
// input, a textarea or a select; anything else has no value to read.
function inputValue(e: Event): string {
  const t = e.target;
  return t instanceof HTMLInputElement || t instanceof HTMLTextAreaElement || t instanceof HTMLSelectElement ? t.value : '';
}

// checkedOf is whether a box stands ticked after a person pressed it.
function checkedOf(e: Event): boolean {
  return e.target instanceof HTMLInputElement ? e.target.checked : false;
}

// chipsURL is where a filter's link points: the same kind and search the state
// holds, so a middle press and a copied address mean the same list.
function chipsURL(kind: string): string {
  const params = new URLSearchParams();
  if (kind) params.set('kind', kind);
  if (query.value) params.set('q', query.value);
  const search = params.toString();
  return '/profiles' + (search ? '?' + search : '');
}

// Toolbar is the list's own: ＋プロファイルを追加, the kind filters and the
// search. The chips' numbers are the rows', so a save that moves a kind's number
// moves the chip with it.
export function Toolbar() {
  return (
    <div class="toolbar">
      <button class="btn primary" data-act="new" onClick={pressNew}>＋ プロファイルを追加</button>
      <div class="filters">
        {chips.value.map((chip) => (
          <a class={'fchip' + (chip.name === filterKind.value ? ' on' : '')} data-act="kind-filter" data-k={chip.name} href={chipsURL(chip.name)} aria-pressed={chip.name === filterKind.value ? 'true' : 'false'} onClick={(e: Event) => { e.preventDefault(); pressFilter(chip.name); }}>
            {chip.name ? <><i class={'sw ' + chip.kindClass}></i>{chip.name}</> : 'すべて'}<span class="n">{chip.count}</span>
          </a>
        ))}
      </div>
      <input id="q" class="search" type="search" placeholder="プロファイル名" value={query.value} autocomplete="off" onInput={(e: Event) => pressQuery(inputValue(e))} />
    </div>
  );
}

// PersonChip is one person who uses a profile: the team, the member and the
// role, and where the assign tab puts the cursor (P11).
function PersonChip({ c }: { c: ProfileChip }) {
  const inner = c.worker
    ? <><span class="wk">ワーカー</span><span class="r">{c.role}</span></>
    : <>{c.who}<span class="g">{c.role}</span></>;
  return (
    <a class="uchip" href={'/profiles/assign#' + c.def + '.' + c.id} data-act="goto" data-g={c.def} data-k={c.id}>
      <span class="g">{c.def}</span>{inner}
    </a>
  );
}

// PlacePills is the row's プロジェクト column (spec 4 章): the project's name, or
// グローバル. A project that overrides a global name carries the crossed-out
// グローバル, and the global row says which projects override it.
function PlacePills({ row }: { row: Row }) {
  if (!GROUPED) {
    return <>{(row.places || []).map((place) => <span class={'pill ' + place.state}>{place.label === 'プロジェクト' ? currentProjectName() : place.label}</span>)}</>;
  }
  const globalGroup = (GROUPS || []).find((g) => g.global);
  const over = (globalGroup && globalGroup.overridden_by) || {};
  return (
    <>
      <span class="pill on">{row.group === 'global' ? 'グローバル' : groupName(row.group)}</span>
      {row.group && row.group !== 'global' && (over[row.name] || []).includes(row.group) ? <span class="pill shadow">グローバル</span> : ''}
      {row.group === 'global' ? (over[row.name] || []).map((p) => <span class="pill hq-ov">{groupName(p)} で上書き</span>) : ''}
    </>
  );
}

// Row is one profile: the summary a person reads and presses, and the detail it
// opens (P1: under itself, inside the view).
export function Row({ row }: { row: Row }) {
  const detail = useRef<HTMLDivElement>(null);
  const sum = useRef<HTMLDivElement>(null);

  // A row a person pressed into opens in front of them (d-d475), and the caret
  // goes back to the line they pressed (5 章 5). Opening from the address is not
  // a press, and does not move the page.
  useEffect(() => {
    const ask = scrollAsk.value;
    if (!ask || ask.key !== row.key || !row.open) return;
    if (detail.current) detail.current.scrollIntoView({ block: 'nearest' });
    if (sum.current) sum.current.focus({ preventScroll: true });
  }, [scrollAsk.value]);

  const first = row.panels[0];
  const name = row.isNew && first ? first.draft.name : row.name;
  const kind = row.isNew && first ? first.draft.kind : row.kind;
  const kindClass = row.isNew && first ? classOf(first.draft.kind) : row.kindClass;
  return (
    <div class={'row' + (row.open ? ' open' : '') + (rowUnsaved(row) ? ' unsaved' : '')} data-key={row.key} data-name={row.name} data-group={row.group || ''} data-kind={row.isNew ? 'claude' : kind}>
      <div class="sum" role="button" tabindex={0} aria-expanded={row.open ? 'true' : 'false'} data-act="toggle" ref={sum} onClick={() => pressToggle(row.key)} onKeyDown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); pressToggle(row.key); } }}>
        <span class="chev"></span>
        <span class="c-name">
          <span class="nm">{name ? name : <span class="none">—</span>}</span>
          {row.isNew ? <span class="tag">新規</span> : ''}
          {row.err > 0 ? <span class="sev err">問題 {row.err}</span> : row.warn > 0 ? <span class="sev warn">警告 {row.warn}</span> : ''}
        </span>
        <span class="c-kind">{kind ? <><i class={'sw ' + kindClass}></i>{kind}</> : ''}</span>
        <span class="c-place">{row.isNew && (row.places || []).length === 0 ? <span class="pill pending">{GROUPED ? groupName(row.group) : labelOf(first ? first.file : 'project')}</span> : <PlacePills row={row} />}</span>
        <span class="c-args">{row.args ? row.args : <span class="none">—</span>}</span>
        <span class="c-env">{(row.env && row.env.length) ? [row.env.map((value) => <span>{value}</span>), row.more > 0 ? <span class="more">+{row.more}</span> : ''] : <span class="none">—</span>}</span>
        <span class="c-users">{row.users > 0 ? <><b>{row.users}</b><span>{row.roles}</span></> : <span class="none">0</span>}</span>
      </div>
      <div class="detail" ref={detail} hidden={!row.open}>
        {row.open ? <>
          {(row.chips && row.chips.length) ? <div class="usage"><span class="lbl">使っている人</span>{row.chips.map((c) => <PersonChip c={c} />)}</div> : ''}
          <div class={'panels' + (row.panels.length > 1 ? ' two' : '')}>{row.panels.map((panel) => <Panel key={panel.key} panel={panel} />)}</div>
        </> : ''}
      </div>
    </div>
  );
}

// badOf is what the check named, read the way markBad read it: a field, an
// argument's number, or an environment variable's key and value.
function badOf(panel: PanelState): { name: boolean; kind: boolean; args: Set<number>; env: Map<number, { key?: boolean; value?: boolean }> } {
  const out = { name: false, kind: false, args: new Set<number>(), env: new Map<number, { key?: boolean; value?: boolean }>() };
  (panel.problems || []).forEach((p) => {
    const at = p.at === 'line' ? '' : (p.at || '');
    const parts = at.split('.');
    if (parts[0] === 'name') out.name = true;
    else if (parts[0] === 'kind') out.kind = true;
    else if (parts[0] === 'args') out.args.add(Number(parts[1]));
    else if (parts[0] === 'env') {
      const i = Number(parts[1]);
      const was = out.env.get(i) || {};
      out.env.set(i, { ...was, [parts[2] === 'value' ? 'value' : 'key']: true });
    }
  });
  return out;
}

// Panel is one layer's face of a profile: what the file holds, the draft, the
// problems the check answered, and the save.
function Panel({ panel }: { panel: PanelState }) {
  const changed = changedOf(panel);
  const bad = badOf(panel);
  const nameInput = useRef<HTMLInputElement>(null);
  const confirmButton = useRef<HTMLButtonElement>(null);
  const hints = panel.renameUsers > 0 && panel.saved && panel.draft.name && panel.draft.name !== panel.saved.name;

  // Focus goes where a person just pressed (5 章 5): a new row's name, the
  // copy's name (all of it, so typing replaces it), the 削除する they asked for.
  useEffect(() => {
    const ask = focusAsk.value;
    if (!ask || ask.key !== panel.key) return;
    if (ask.field === 'name' && nameInput.current) {
      nameInput.current.focus();
      if (ask.select) nameInput.current.select();
    }
    if (ask.field === 'del-yes' && confirmButton.current) confirmButton.current.focus();
  }, [focusAsk.value]);

  const saved = panel.saved;
  const acts = (
    <>
      {saved ? <button class="btn" data-act="dup" onClick={() => pressDup(panel.key)}>複製</button> : ''}
      {saved && !panel.both ? <button class="btn" data-act="copy" onClick={() => pressCopy(panel.key)}>{panel.otherLabel}にコピー</button> : ''}
      {saved ? <button class="btn danger" data-act="del" onClick={() => pressDel(panel.key)}>削除</button> : <button class="btn danger" data-act="discard" onClick={() => pressDiscard(panel.key)}>取り消し</button>}
    </>
  );
  return (
    <section class={'panel' + (panel.shadowed ? ' shadowed' : '')} data-key={panel.key} data-file={panel.file} data-group={panel.group || ''} data-saved={saved ? '1' : undefined} data-users={panel.users} data-rename-users={panel.renameUsers} data-hash={panel.hash}>
      <header class="ph">
        {panel.layerSwitch
          ? <div class="seg small">{(['project', 'global'] as Place[]).map((x) => <button class={panel.file === x ? 'on' : ''} data-act="pick-layer" data-x={x} onClick={() => pressLayer(panel.key, x)}>{labelOf(x)}</button>)}</div>
          : <span class="ln">{GROUPED && panel.group && panel.group !== 'global' ? groupName(panel.group) : panel.label}</span>}
        <code class="path">{panel.path}</code>
        {panel.both ? <span class={'state ' + (panel.shadowed ? 'off' : 'on')}>{panel.shadowed ? '隠れている' : '効いている'}</span> : ''}
      </header>
      <div class="field"><label>プロファイル名</label><div>
        <input class={'in-name' + (changed.name ? ' changed' : '') + (bad.name ? ' bad' : '')} data-f="name" value={panel.draft.name} spellcheck={false} autocomplete="off" ref={nameInput} onInput={(e: Event) => inputField(panel.key, 'name', 0, inputValue(e))} />
        <label class="rename" hidden={!hints}><input type="checkbox" data-f="rename" checked={panel.rename} onChange={(e: Event) => pressRename(panel.key, checkedOf(e))} /><span>{hints ? `使っている ${panel.renameUsers} 人の指定も ${panel.draft.name} に書き換える` : ''}</span></label>
        <ul class="vm" data-for="name">{problemsOf(panel, 'name')}</ul>
      </div></div>
      <div class="field"><label>kind</label><div>
        <div class={'seg' + (changed.kind ? ' changed' : '') + (bad.kind ? ' bad' : '')} data-f="kind">
          {BASE_KINDS.map((k) => <button class={panel.draft.kind === k ? 'on' : ''} data-act="set-kind" data-k={k} aria-pressed={panel.draft.kind === k ? 'true' : 'false'} onClick={() => pressSetKind(panel.key, k)}><i class={'sw ' + classOf(k)}></i>{k}</button>)}
          {kindSelect(panel)}
        </div>
        <ul class="vm" data-for="kind">{problemsOf(panel, 'kind')}</ul>
      </div></div>
      <div class="field"><label>引数</label><div>
        <Args panel={panel} changed={changed} bad={bad} />
        <ul class="vm" data-for="args">{problemsOf(panel, 'args')}</ul>
      </div></div>
      <div class="field"><label>環境変数</label><div>
        <div class="env">
          {panel.draft.env.map((pair, i) => <EnvRow key={'env' + i} index={i} pair={pair} changed={changed.env[i] || {}} bad={bad.env.get(i) || {}} panelKey={panel.key} />)}
          <button class="add" data-act="env-add" aria-label="環境変数を足す" onClick={() => pressEnvAdd(panel.key)}>＋</button>
        </div>
        <ul class="vm" data-for="env">{problemsOf(panel, 'env')}</ul>
      </div></div>
      <footer class="pf">
        <button class="btn primary" data-act="save" disabled={saveBlocked(panel)} onClick={() => pressSave(panel.key)}>保存</button><span class="dirty">{saved && changed.count ? `${changed.count} か所` : ''}</span><span class="vcount">{counts(panel)}</span>{panel.savedLine ? <span class="ok">保存しました</span> : ''}{panel.refusal ? <span class="sev err refusal">{panel.refusal}</span> : ''}
        <span class="acts">{panel.confirming
          ? <>
            <button class="btn danger solid" data-act="del-yes" ref={confirmButton} onClick={() => pressDelYes(panel.key)}>削除する{panel.users ? `（${panel.users} 人が使っている）` : ''}</button>
            <button class="btn" data-act="del-no" onClick={() => pressDelNo(panel.key)}>やめる</button>
          </>
          : acts}</span>
      </footer>
    </section>
  );
}

// Args is the argument chips and the press that adds one. A dragged chip's order
// is the order the save writes (spec 4 章): the drop works it out from the
// panel's own draft, not from where the elements stand (5 章 2).
function Args({ panel, changed, bad }: { panel: PanelState; changed: ChangeSet; bad: ReturnType<typeof badOf> }) {
  const drop = (event: DragEvent): void => {
    const found = event.target instanceof Element ? event.target.closest('.arg') : null;
    const target = found instanceof HTMLElement ? found : null;
    const holder = event.currentTarget instanceof HTMLElement ? event.currentTarget : null;
    const dragged = holder ? holder.querySelector('.dragging') : null;
    const source = dragged instanceof HTMLElement ? dragged : null;
    if (!target || !source || target === source) return;
    event.preventDefault();
    const draft = (panelByKey(panel.key) || { draft: { args: [] } }).draft;
    const order = draft.args.slice();
    const from = Number(source.dataset.i), to = Number(target.dataset.i);
    const rect = target.getBoundingClientRect();
    if (!(from in order) || !(to in order)) return;
    const [moved] = order.splice(from, 1);
    const at = event.clientX < rect.left + rect.width / 2 ? (to > from ? to - 1 : to) : (to >= from ? to : to + 1);
    order.splice(at, 0, moved);
    source.classList.remove('dragging');
    source.removeAttribute('draggable');
    reorderArgs(panel.key, order);
  };
  return (
    <div class="args" onDragOver={(e: DragEvent) => { if (e.target instanceof Element && e.target.closest('.arg')) e.preventDefault(); }} onDrop={drop}>
      <span class="bin">{panel.draft.kind}</span>
      {panel.draft.args.map((arg, i) => <ArgChip key={'arg' + i} index={i} arg={arg} changed={changed.args[i] || false} bad={bad.args.has(i)} panelKey={panel.key} />)}
      <button class="add" data-act="arg-add" aria-label="引数を足す" onClick={() => pressArgAdd(panel.key)}>＋</button>
    </div>
  );
}

function problemsOf(panel: PanelState, field: string) {
  const list = (panel.problems || []).filter((p) => (p.at === 'line' ? '' : (p.at || '')).split('.')[0] === field);
  return list.map((p) => <li><span class={'sev ' + (p.sev === 'err' ? 'err' : 'warn')}>{p.sev === 'err' ? '問題' : '警告'}</span><span>{p.text}</span></li>);
}

function counts(panel: PanelState) {
  const errs = (panel.problems || []).filter((p: Problem) => p.sev === 'err').length;
  const warns = (panel.problems || []).length - errs;
  return [errs ? <span class="sev err">問題 {errs}</span> : '', warns ? <span class="sev warn">警告 {warns}</span> : ''];
}

function kindSelect(panel: PanelState) {
  const rest = otherKinds().filter((k) => !BASE_KINDS.includes(k));
  const kind = panel.draft.kind;
  if (kind && !BASE_KINDS.includes(kind) && !rest.includes(kind)) rest.push(kind);
  const chosen = BASE_KINDS.includes(kind) ? '' : kind;
  return (
    <span class="sel">
      <select data-f="kind-other" aria-label="ほかの kind" value={chosen} onChange={(e: Event) => pressKindOther(panel.key, inputValue(e))}>
        <option value="">ほか</option>
        {rest.map((k) => <option value={k}>{k}</option>)}
      </select>
    </span>
  );
}

// ArgChip is one argument: the grip that moves it, the field, and the × that
// takes it away.
function ArgChip({ index, arg, changed, bad, panelKey }: { index: number; arg: string; changed: boolean; bad: boolean; panelKey: string }) {
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => {
    const ask = focusAsk.value;
    if (ask && ask.key === panelKey && ask.field === 'arg' && ask.index === index && input.current) input.current.focus();
  }, [focusAsk.value]);
  const size = Math.max(String(arg).length, 3);
  return (
    <span class={'arg' + (String(arg).startsWith('-') ? ' flag' : '') + (changed ? ' changed' : '') + (bad ? ' bad' : '')} data-i={index} onPointerDown={(e: PointerEvent) => { if (e.target instanceof Element && e.target.closest('.grip') && e.currentTarget instanceof HTMLElement) e.currentTarget.setAttribute('draggable', 'true'); }} onDragStart={(e: DragEvent) => { if (e.currentTarget instanceof HTMLElement) e.currentTarget.classList.add('dragging'); }} onDragEnd={(e: DragEvent) => { if (e.currentTarget instanceof HTMLElement) { e.currentTarget.classList.remove('dragging'); e.currentTarget.removeAttribute('draggable'); } }}>
      <span class="grip" aria-hidden="true">⋮</span>
      <input data-f="arg" data-i={index} value={arg} size={size} spellcheck={false} autocomplete="off" ref={input} onInput={(e: Event) => inputField(panelKey, 'arg', index, inputValue(e))} />
      <button class="x" data-act="arg-del" data-i={index} aria-label={`${arg} を消す`} onClick={() => pressArgDel(panelKey, index)}>×</button>
    </span>
  );
}

// EnvRow is one environment variable: the key, the value and the ×.
function EnvRow({ index, pair, changed, bad, panelKey }: { index: number; pair: { key: string; value: string }; changed: { key?: boolean; value?: boolean }; bad: { key?: boolean; value?: boolean }; panelKey: string }) {
  const key = useRef<HTMLInputElement>(null);
  const val = useRef<HTMLTextAreaElement>(null);
  useEffect(() => {
    const ask = focusAsk.value;
    if (ask && ask.key === panelKey && ask.field === 'ek' && ask.index === index && key.current) key.current.focus();
  }, [focusAsk.value]);
  useEffect(() => {
    if (!val.current) return;
    val.current.style.height = 'auto';
    val.current.style.height = `${val.current.scrollHeight + 2}px`;
  }, [pair.value]);
  return (
    <div class="envrow" data-i={index}>
      <input class={(changed.key ? 'changed' : '') + (bad.key ? ' bad' : '')} data-f="ek" data-i={index} value={pair.key} spellcheck={false} autocomplete="off" ref={key} onInput={(e: Event) => inputField(panelKey, 'ek', index, inputValue(e))} />
      <span class="eq">=</span>
      <textarea class={(changed.value ? 'changed' : '') + (bad.value ? ' bad' : '')} data-f="ev" data-i={index} rows={1} spellcheck={false} value={pair.value} ref={val} onInput={(e: Event) => inputField(panelKey, 'ev', index, inputValue(e))}></textarea>
      <button class="x" data-act="env-del" data-i={index} aria-label={`${pair.key} を消す`} onClick={() => pressEnvDel(panelKey, index)}>×</button>
    </div>
  );
}

// List is the tab: the toolbar, the column line, the rows in their groups, and
// the one line a list with nothing in it carries.
export function List() {
  const visible = shownRows.value;
  return (
    <>
      <Toolbar />
      <div class="list">
        <div class="head"><span></span><span>プロファイル名</span><span>kind</span><span>プロジェクト</span><span>引数</span><span>環境変数</span><span>使っている人</span></div>
        <div id="rows">
          {rows.value.length === 0 ? <p class="none">プロファイルはありません</p> : ''}
          <p class="none" id="no-match" hidden={rows.value.length === 0 || visible.length > 0}>合うプロファイルはありません</p>
          {groupedRows(visible).map((one) => ('head' in one ? <GroupHead group={one.head} /> : <Row key={one.row.key} row={one.row} />))}
        </div>
      </div>
    </>
  );
}

// groupedRows puts a group's head where its first row stands (spec 4 章).
function groupedRows(list: Row[]): ({ head: string } | { row: Row })[] {
  if (!GROUPED) return list.map((row) => ({ row }));
  const out: ({ head: string } | { row: Row })[] = [];
  let last: string | null = null;
  list.forEach((row) => {
    const group = row.group || '';
    if (group !== last) { out.push({ head: group }); last = group; }
    out.push({ row });
  });
  return out;
}

function GroupHead({ group }: { group: string }) {
  const g = (GROUPS || []).find((x) => (x.global ? 'global' : (x.project && x.project.path)) === group);
  if (!g) return null;
  if (g.global) return <div class="hq-gh global"><span class="hq-gn">グローバル</span><span class="hq-gp">{g.display || ''}</span><span class="hq-gc">どのプロジェクトからも使える。同じ名前があれば、そのプロジェクトのほうが使われる</span></div>;
  const count = ((g.list && g.list.items) || []).length;
  return <div class="hq-gh"><span class="hq-gn">{g.project ? g.project.name : ''}</span><span class="hq-gp">{g.project ? g.project.display : ''}</span><span class="hq-gc">プロファイル <b>{count}</b></span></div>;
}
