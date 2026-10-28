// lib/hq/sheets.js — 本部の欄と、横断の検索（n-7f3c、spec 2・3・5 章）。
//
// 欄は 3 つ: 並べるプロジェクト、起動、止める（と、起動の途中を見せる欄）。
// どれも本文の後ろに 1 つだけ立ち、外を押すか Esc で閉じる。中身は Preact が描き、
// 押し込みは欄が自分で受ける（webui-preact-design 5 章 1）。
//
// 検索は帯の #hq-q から。並べたプロジェクトとグローバルのチーム定義・
// ワークスペース・メンバー・ロール・規則・プロファイルを一度に読み、/ で開き、
// ↑↓ で選び、Enter で行く（spec 3 章）。
import { html, render as draw, signal, computed, useEffect, useLayoutEffect, useRef, useState } from '../../vendor/preact.js';
import { call, problemText } from '../api.js';
import {
  st, rev, token, projectName, readJSONOr, readGroups, readCore, readRunning,
  noteStarting, definitionDoc, listProject, unlistProject, groupOf, projectOfGroup,
  groupLabel, runningTeam,
} from './state.js';

// ---- 欄の器 -----------------------------------------------------------
// sheetHost is the one element a sheet stands in. It is the part's own: the
// screens' documents have no host of their own any more (webui-components 1 章).
let sheetHost = null;
export const sheetOpen = () => !!sheetHost;

export function closeSheet() {
  if (!sheetHost) return;
  draw(null, sheetHost);
  sheetHost.remove();
  sheetHost = null;
}

// openSheet stands one sheet behind the page: the element the screen scrolls
// under, and the tree inside it.
function openSheet(vnode, side) {
  closeSheet();
  sheetHost = document.createElement('div');
  sheetHost.className = 'hq-sheet ' + (side || 'right');
  sheetHost.setAttribute('role', 'dialog');
  sheetHost.setAttribute('aria-modal', 'false');
  sheetHost.tabIndex = -1;
  document.body.appendChild(sheetHost);
  sheetHost.focus();
  draw(vnode, sheetHost);
  return sheetHost;
}

// Seeded is an input whose value is put in when the seed changes, and never while
// a person types: the field itself holds what was typed (webui-preact-design 5 章 5).
function Seeded({ value, ...rest }) {
  const el = useRef(null);
  useLayoutEffect(() => {
    const field = el.current;
    if (!field) return;
    const want = value == null ? '' : String(value);
    if (field.value !== want) field.value = want;
  });
  return html`<input ...${rest} ref=${el} />`;
}

// spaced is a list of pieces with the separator the screen always used: names as
// <code>lead</code> · <code>analyst</code>.
const spaced = (items) => items.reduce((all, one) => (all.length ? all.concat(' · ', one) : [one]), []);

const Close = () => html`<button type="button" class="hq-x" data-hqx="close" aria-label="閉じる" onClick=${closeSheet}>×</button>`;
const Head = ({ title, label }) => html`<div class="hq-h"><b>${title}</b>${label ? html`<code>${label}</code>` : ''}<${Close} /></div>`;
const Err = ({ text }) => html`<p class="hq-err" data-role="error" hidden=${!text}>${text || ''}</p>`;

const mmdd = (iso) => {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return '';
  const two = (n) => String(n).padStart(2, '0');
  return `${two(d.getMonth() + 1)}/${two(d.getDate())}`;
};

// ---- 並べるプロジェクトの欄（spec 2 章） --------------------------------
function ProjectsSheet() {
  const [error, setError] = useState('');
  const [path, setPath] = useState('');
  const p = st.projects;
  if (!p) return html`<${Head} title="本部に並べるプロジェクト" /><p class="hq-sub">読み込んでいます…</p>`;
  const listed = (p.projects || []).filter((x) => x.listed);
  const candidates = (p.projects || []).filter((x) => !x.listed && x.source === 'launch');
  const current = p.current || {};
  const currentOnly = p.mode !== 'listed';
  const rewrite = (fn, arg) => async () => {
    const no = await fn(arg);
    if (no) { setError(no); return; }
    location.reload();
  };
  const row = (project) => html`<div class="hq-li"><span class="hq-pn">${project.name}</span><span class="hq-pp">${project.display}</span>
      <span class="hq-pc">チーム定義 ${project.definitions || 0} · 動いているチーム ${project.running || 0}</span>
      <span class="hq-act"><button type="button" class="hq-btn sm" data-hq="unlist" data-path=${project.path} onClick=${rewrite(unlistProject, project.path)}>外す</button></span></div>`;
  const cand = (project) => html`<div class="hq-li cand"><span class="hq-pn">${project.name}</span><span class="hq-pp">${project.display}</span>
      <span class="hq-pc">最後の起動 ${mmdd(project.last_launch)} · チーム定義 ${project.definitions || 0}</span>
      <span class="hq-act"><button type="button" class="hq-btn sm" data-hq="list" data-path=${project.path} onClick=${rewrite(listProject, project.path)}>並べる</button></span></div>`;
  // Without projects:, the project serve was started in is not listed yet. It
  // stands in the sheet with 並べる, and listing anything lists it first, so the
  // teams a person is looking at do not fall out of the screen (r-5b96 audit 7).
  const currentRow = currentOnly
    ? html`<div class="hq-li"><span class="hq-pn">${current.name}</span><span class="hq-pp">${current.display}</span><span class="hq-pc">チーム定義 ${current.definitions || 0} · 動いているチーム ${current.running || 0}</span><span class="hq-act"><button type="button" class="hq-btn sm" data-hq="list" data-path=${current.path} onClick=${rewrite(listProject, current.path)}>並べる</button></span></div>`
    : '';
  const unlisted = p.mode === 'listed' && !current.listed
    ? html`<div class="hq-li cand"><span class="hq-pn">${current.name}</span><span class="hq-pp">${current.display}</span><span class="hq-pc">この serve を起こしたプロジェクト（並んでいない）</span><span class="hq-act"><button type="button" class="hq-btn sm" data-hq="list" data-path=${current.path} onClick=${rewrite(listProject, current.path)}>並べる</button></span></div>`
    : '';
  const others = candidates.filter((x) => x.path !== current.path).map(cand);
  const add = async () => {
    const want = path.trim();
    if (!want) { setError('プロジェクトのディレクトリのパスを書いてください'); return; }
    const no = await listProject(want);
    if (no) { setError(no); return; }
    location.reload();
  };
  return html`<${Head} title="本部に並べるプロジェクト" />
      <p class="hq-sub">並べたプロジェクトのチーム定義・ロール・プロファイルと、動いているチームが、どの画面にも出ます。どこで hergy serve を起こしても、同じ並びです。</p>
      ${currentOnly
        ? html`<div class="hq-sec"><span class="hq-lbl">この serve を起こしたプロジェクト（まだ並べていない）</span>${currentRow}</div>`
        : html`<div class="hq-sec"><span class="hq-lbl">並べている ${listed.length}</span>${listed.length ? listed.map(row) : html`<p class="hq-sub">まだありません</p>`}</div>`}
      <div class="hq-sec"><span class="hq-lbl">起動の記録にあって、並べていないプロジェクト</span>${unlisted}${others.length ? others : unlisted ? '' : html`<p class="hq-sub">ありません</p>`}</div>
      <p class="hq-note">並びは <code>~/.config/hergy/config.yaml</code> の <code>projects:</code> に書きます。外しても、そのプロジェクトのファイルは変わりません。${currentOnly ? html` 並べると <code>config.yaml</code> に <code>projects:</code> を作ります。この serve を起こした ${current.name} も一緒に並べます。` : ''}</p>
      <div class="hq-sec hq-foot"><span class="hq-lbl">パスで並べる（まだチームを起こしたことのないプロジェクト）</span><div class="hq-acts"><${Seeded} id="hq-path" class="hq-sel" placeholder="~/src/…" autocomplete="off" aria-label="プロジェクトのパス" value=${path} onInput=${(e) => setPath(e.target.value)} /><button type="button" class="hq-btn sm" data-hq="add" onClick=${add}>並べる</button></div><${Err} text=${error} /></div>`;
}

// ---- 起動の欄（spec 5 章） ---------------------------------------------
// startSheet is the definition and its project, the members, the workspace in the
// spec's order, and the name when one is taken. A global definition chooses its
// project first; its members and workspaces follow the pick (r-5b96 audit 8).
function StartSheet({ definition, place, project }) {
  const isGlobal = (place || 'project') === 'global';
  const [options, setOptions] = useState(null);
  const [chosen, setChosen] = useState('');
  const [picked, setPicked] = useState('');
  const [asName, setAsName] = useState('');
  const [wsName, setWsName] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const take = (next) => {
    setOptions(next);
    setChosen(next.project ? next.project.path : '');
    setPicked(next.last_workspace ? next.last_workspace.id : '');
    setAsName(next.suggested_name || '');
    setWsName(next.new_workspace_name || definition);
  };
  useEffect(() => {
    let alive = true;
    (async () => {
      await readGroups();
      const q = new URLSearchParams({ definition, place: place || 'project' });
      if (project) q.set('project', project);
      const next = await readJSONOr('/api/hq/launch/options?' + q.toString());
      if (!alive) return;
      if (!next) { setError('起動の選択肢を読めませんでした'); setOptions({ broken: true }); return; }
      take(next);
    })();
    return () => { alive = false; };
  }, [definition, place, project]);

  const pickProject = async (path) => {
    setChosen(path);
    setPicked('');
    // The refusal for a missing project is about the pick before this one: it
    // goes as soon as one is picked (r-5b96 audit R2).
    setError('');
    if (!path) {
      setOptions((was) => Object.assign({}, was, { members: [], clash: [], suggested_name: '', new_workspace_name: definition, workspaces: [] }));
      setAsName('');
      return;
    }
    const q = new URLSearchParams({ definition, place: 'global', project: path });
    const next = await readJSONOr('/api/hq/launch/options?' + q.toString());
    if (!next) return;
    setOptions((was) => Object.assign({}, was, {
      members: next.members || [], clash: next.clash || [], suggested_name: next.suggested_name || '',
      new_workspace_name: next.new_workspace_name || definition, workspaces: next.workspaces || [],
    }));
    setPicked(next.last_workspace ? next.last_workspace.id : '');
    setAsName(next.suggested_name || '');
    setWsName(next.new_workspace_name || definition);
  };

  if (!options) {
    return html`<${Head} title="起動する" label=${definition} />${error
      ? html`<p class="hq-err">${error}</p>`
      : html`<p class="hq-sub">読み込んでいます…</p>`}`;
  }

  // membersOf is the people the definition starts: the answer's own list, or,
  // before a project is chosen for a global definition, the definition's own
  // members (they do not depend on the project).
  const membersOf = () => {
    if ((options.members || []).length) return options.members;
    const doc = definitionDoc(definition, chosen);
    return ((doc && doc.members) || []).map((m) => ({ name: m.name, role: m.role, profile: m.profile }));
  };
  const members = membersOf();
  const clash = options.clash || [];
  const same = (as) => {
    const asPart = clash.length > 0 && as ? ` --as ${as}` : '';
    return chosen
      ? html`<code>${projectName(chosen)}</code> で <code>${`hergy up ${definition}${asPart}`}</code> を打つのと同じ`
      : html`選んだプロジェクトで <code>${`hergy up ${definition}${asPart}`}</code> を打つのと同じ`;
  };
  const wsWhy = (w) => (w.why === 'last' ? `前に ${definition} が動いたワークスペース` : w.why === 'project' ? 'プロジェクトの名前で始まるワークスペース' : w.why === 'busy' ? `${w.team} が動いている（1 つのワークスペースにチームは 1 つ）` : 'hergy のチームはいない');
  const start = async () => {
    const proj = isGlobal ? chosen : project;
    if (!proj) { setError('どのプロジェクトで起こすかを選んでください'); return; }
    // Which workspace is picked, and the name a new one takes, are the sheet's
    // state: nothing is read back from the boxes (5 章 2).
    if (!picked) { setError('ワークスペースを選んでください'); return; }
    let workspace;
    if (picked === '__new') {
      const name = wsName.trim();
      if (!name) { setError('新しいワークスペースの名前を書いてください'); return; }
      workspace = { new: name };
    } else workspace = { id: picked };
    const body = { project: proj, definition, place: place || 'project', workspace };
    if (asName.trim()) body.name = asName.trim();
    setBusy(true);
    let answer;
    try {
      answer = await call('POST', '/api/hq/launch', body);
    } catch (e) {
      setBusy(false);
      setError(problemText(e, '起動できませんでした'));
      return;
    }
    setBusy(false);
    const launch = (answer.json || {}).launch;
    // The row says 起動の途中 at once, rather than waiting for the ten-second
    // reading (r-5b96 audit R5).
    if (launch) { noteStarting(launch); runStart(launch); }
  };

  return html`<${Head} title="起動する" label=${definition} />
      <dl class="hq-kv">
        <dt>チーム定義</dt><dd><code>${definition}</code>（${isGlobal ? 'グローバル' : projectName(chosen)}）</dd>
        <dt>プロジェクト</dt><dd>${isGlobal
          ? html`<select id="hq-proj" class="hq-sel" onChange=${(e) => pickProject(e.target.value)}><option value="">選んでください</option>${(options.projects || []).map((x) => (x.available
            ? html`<option value=${x.path} selected=${x.path === chosen}>${x.name}</option>`
            : html`<option value="" disabled>${`${x.name}（${x.reason}）`}</option>`))}</select><div class="hq-sub">グローバルのチーム定義は、どのプロジェクトで起こすかを選びます</div>`
          : html`<code>${projectName(chosen)}</code>`}</dd>
        <dt>メンバー</dt><dd data-role="members">${members.length ? html`${`${members.length} 人: `}${spaced(members.map((m) => html`<code>${m.name}</code>`))}` : '—'}</dd>
      </dl>
      <dl class="hq-kv" data-role="clash">${clash.length
        ? html`<dt>チームの名前</dt><dd><${Seeded} id="hq-as" class="hq-sel" value=${asName} onInput=${(e) => setAsName(e.target.value)} /><p class="hq-sub">${definition} という名前のチームが ${clash.map((c) => projectName(c.project)).join(' · ')} で動いています。この名前で起こします（hergy up --as と同じ）。</p></dd>`
        : ''}</dl>
      <div class="hq-sec"><span class="hq-lbl">ワークスペース</span><div class="hq-ws" data-role="wslist">${isGlobal && !chosen
        ? html`<p class="hq-sub">プロジェクトを選ぶと、ワークスペースが出ます</p>`
        : html`${(options.workspaces || []).map((w) => (w.selectable
          ? html`<label class="hq-wo"><input type="radio" name="hq-ws" value=${w.id} checked=${w.id === picked} onChange=${() => setPicked(w.id)} /><span class="hq-wn">${w.label}</span><span class="hq-wd">${wsWhy(w)}</span></label>`
          : html`<label class="hq-wo no"><input type="radio" name="hq-ws" disabled /><span class="hq-wn">${w.label}</span><span class="hq-wd">${wsWhy(w)}</span></label>`))}<label class="hq-wo"><input type="radio" name="hq-ws" value="__new" checked=${picked === '__new'} onChange=${() => setPicked('__new')} /><span class="hq-wn">新しく作る</span><span class="hq-wd"><${Seeded} id="hq-wsname" class="hq-in" value=${options.new_workspace_name || definition} onInput=${(e) => setWsName(e.target.value)} aria-label="新しいワークスペースの名前" /></span></label>`}</div></div>
      <${Err} text=${error} />
      <div class="hq-acts"><button type="button" class="hq-btn primary" data-hq="go" data-def=${definition} data-place=${place || 'project'} disabled=${busy} onClick=${start}>起動する</button><button type="button" class="hq-btn" data-hqx="close" onClick=${closeSheet}>やめる</button>
      <span class="hq-sub" data-role="same">${same(asName)}</span></div>`;
}

// ---- 起動の途中（spec 5 章） -------------------------------------------
// watched holds every start being watched: one entry per launch id. The watching
// goes on after the sheet is closed (the band's numbers move when it ends), and
// the sheet draws from the last reading.
const watched = signal({});
const watching = {};

function watchLaunch(id) {
  if (watching[id]) return;
  watching[id] = setInterval(async () => {
    const answer = await readJSONOr('/api/hq/launches/' + encodeURIComponent(id));
    if (!answer || !answer.launch) { clearInterval(watching[id]); delete watching[id]; return; }
    const launch = answer.launch;
    watched.value = Object.assign({}, watched.value, { [id]: launch });
    if (launch.phase === 'done' || launch.phase === 'failed' || launch.phase === 'cancelled') {
      clearInterval(watching[id]);
      delete watching[id];
      // The row and the group's count move as the title does, rather than on the
      // next ten-second reading (r-5b96 audit S2).
      readRunning();
      setTimeout(readRunning, 800);
    }
  }, 900);
}

function RunSheet({ id, launch: first }) {
  const steps = useRef(null);
  const launch = watched.value[id] || first;
  const over = launch.phase === 'done' || launch.phase === 'failed' || launch.phase === 'cancelled';
  // A stage is the step in progress, not a finished one: the last stage stands
  // with … only while the start is starting, and is dropped when the next line
  // comes or the start waits or ends (r-5b96 audit 9).
  const all = launch.steps || [];
  const lastStage = all.map((x, i) => (x.kind === 'stage' ? i : -1)).filter((i) => i >= 0).pop();
  const shownSteps = all.filter((x, i) => x.kind !== 'stage' || (launch.phase === 'starting' && i === lastStage));
  useLayoutEffect(() => {
    if (steps.current) steps.current.scrollTop = steps.current.scrollHeight;
  });
  const title = launch.phase === 'done' ? '起動しました'
    : launch.phase === 'failed' ? '起動できませんでした'
      : launch.phase === 'cancelled' ? '起動をやめました' : '起動しています';
  const where = `${launch.project_name} · ワークスペース ${launch.workspace.label || launch.workspace.id}`;
  const abort = async () => {
    try {
      const answer = await call('DELETE', '/api/hq/launches/' + encodeURIComponent(id));
      const next = (answer.json || {}).launch;
      if (next) watched.value = Object.assign({}, watched.value, { [id]: next });
    } catch (e) { /* the reading a moment later says where it stands */ }
    readRunning();
  };
  return html`<div class="hq-h"><b data-role="title">${title}</b><code>${launch.team}</code><${Close} /></div>
      <p class="hq-sub" data-role="where">${over ? where : `${where}。この欄を閉じても、起動は続きます。`}</p>
      <ul class="hq-steps" data-role="steps" ref=${steps}>${shownSteps.map((step) => {
        const now = step.kind === 'stage';
        const mark = now ? 'now' : step.kind === 'done' || step.kind === 'member' ? 'done' : '';
        return html`<li class=${mark}><span>${step.text}</span></li>`;
      })}</ul>
      <div data-role="waitbox">${launch.waiting ? html`<div class="hq-wait"><div class="hq-wh"><span class="hq-on wait">許可待ち</span><b><code>${launch.waiting.herdr}</code> が、起動の途中で確認に答えを待っています</b></div>
          <p class="hq-where">ワークスペース ${launch.waiting.workspace} · タブ ${launch.waiting.tab} · ペイン ${launch.waiting.pane}</p>
          <p>エージェントが、このフォルダを信頼するか、などを聞いています。そのペインを開いて答えてください。確認は続けて出ることがあります。</p>
          <p>答え終わると、hergy は <code>${launch.waiting.herdr}</code> に初期プロンプトを送って続きを進めます。</p></div>` : ''}</div>
      <div class="hq-acts" data-role="endacts">${launch.phase === 'done'
        ? html`<p class="hq-ok">${`起動しました: ${launch.team}（${launch.project_name} · ワークスペース ${launch.workspace.label || launch.workspace.id} · タブ ${launch.tab.label}）`}</p>
        <a class="hq-btn primary" href=${`/activity?open=${encodeURIComponent(token(launch.team, launch.project))}`}>活動で見る</a><button type="button" class="hq-btn" data-hqx="close" onClick=${closeSheet}>閉じる</button>`
        : launch.phase === 'failed'
          ? html`<p class="hq-err">${launch.error || '起動できませんでした'}</p><button type="button" class="hq-btn" data-hqx="close" onClick=${closeSheet}>閉じる</button>`
          : launch.phase === 'cancelled'
            ? html`<p class="hq-sub">起動をやめました。作ったタブとペインは残っています（止めるのはチームを止める欄から）。</p><button type="button" class="hq-btn" data-hqx="close" onClick=${closeSheet}>閉じる</button>`
            : html`<button type="button" class="hq-btn danger" data-hq="abort" data-launch=${id} onClick=${abort}>起動をやめる</button>`}</div>`;
}

export function runStart(launch) {
  watched.value = Object.assign({}, watched.value, { [launch.id]: launch });
  watchLaunch(launch.id);
  openSheet(html`<${RunSheet} id=${launch.id} launch=${launch} />`);
}

// ---- 止める欄（spec 5 章） ---------------------------------------------
function StopSheet({ team }) {
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const t = runningTeam(team);
  if (!t) {
    readRunning();
    return html`<${Head} title="止める" label=${team} /><p class="hq-sub">このチームは動いていません。読み直してから、もう一度お試しください。</p>`;
  }
  const members = (t.members || []).filter((m) => m.live !== false).map((m) => m.name);
  const workers = (t.workers || []).length;
  const stop = async () => {
    setBusy(true);
    try {
      await call('POST', '/api/hq/stop', { team });
    } catch (e) {
      setBusy(false);
      setError(problemText(e, '止められませんでした'));
      return;
    }
    setBusy(false);
    closeSheet();
    // The row goes at once, rather than waiting for the ten-second reading
    // (r-5b96 audit R5).
    await readCore();
  };
  return html`<${Head} title="止める" label=${team} />
      <dl class="hq-kv"><dt>プロジェクト</dt><dd><code>${t.project_name || projectName(t.project)}</code></dd><dt>ワークスペース</dt><dd><code>${t.workspace.label || t.workspace.id}</code></dd>
      <dt>閉じるペイン</dt><dd>${`メンバー ${members.length} 人（`}${spaced(members.map((m) => html`<code>${m}</code>`))}${`）${workers ? `と、${team} が追加したワーカー ${workers} 人のタブ` : ''}`}</dd></dl>
      <p class="hq-sub">${`エージェントを止めてペインを閉じます。会話は残りません。ワークスペース ${t.workspace.label || t.workspace.id} は閉じません。${t.project_name || projectName(t.project)} で `}<code>${`hergy down ${team}`}</code>${' を打つのと同じです。'}</p>
      <${Err} text=${error} />
      <div class="hq-acts"><button type="button" class="hq-btn danger solid" data-hq="stopgo" data-team=${team} disabled=${busy} onClick=${stop}>止める</button><button type="button" class="hq-btn" data-hqx="close" onClick=${closeSheet}>やめる</button></div>`;
}

// ---- 開く -------------------------------------------------------------
export function startSheet(definition, place, project) {
  openSheet(html`<${StartSheet} definition=${definition} place=${place || 'project'} project=${project || ''} />`);
}

export function stopSheet(team) {
  openSheet(html`<${StopSheet} team=${team} />`);
}

export function projectsSheet() {
  openSheet(html`<${ProjectsSheet} />`, 'left');
}

// ---- 横断の検索（spec 3 章） -------------------------------------------
// KINDS is the order the result groups stand in. A rule's group shows six rows
// and the rest as ほか N 件; every other group shows five.
const KINDS = [
  ['live', '動いているチーム'], ['def', 'チーム定義'], ['ws', 'ワークスペース'], ['member', 'メンバー'],
  ['role', 'ロール'], ['rule', '規則'], ['profile', 'プロファイル'],
];
const CAP = (kind) => (kind === 'rule' ? 6 : 5);

let index = null;
let indexTeams = null;

// index builds the rows the search reads: names, missions, rules' ids and texts,
// kinds and args — the fields api.md 3.5 names (spec 3 章).
function buildIndex() {
  const out = [];
  for (const g of groupOf('teams')) {
    const group = projectOfGroup(g), label = groupLabel(g);
    for (const item of (g.list ? g.list.items : [])) {
      const places = item.places || {};
      const eff = item.effective || (places.project ? 'project' : 'global');
      const doc = (places[eff] || {}).doc || {};
      const href = `/teams?open=${encodeURIComponent(token(item.name, group))}`;
      out.push({ kind: 'def', group, project: label, label: item.name, sub: doc.mission || '', text: `${item.name} ${doc.mission || ''}`, href });
      for (const m of doc.members || []) {
        out.push({ kind: 'member', group, project: label, label: `${item.name} の ${m.name}`, sub: `ロール ${m.role || ''} · プロファイル ${m.profile || ''}`, text: `${m.name} ${item.name}-${m.name} ${m.role || ''} ${m.profile || ''}`, href });
      }
      const agree = (doc.agreement && doc.agreement.doc) || {};
      for (const sec of agree.sections || []) {
        for (const rule of sec.rules || []) {
          out.push({ kind: 'rule', group, project: label, label: rule.id, sub: `${item.name} のルール`, text: `${rule.id} ${rule.text}`, body: rule.text, href });
        }
      }
    }
  }
  for (const g of groupOf('roles')) {
    const group = projectOfGroup(g), label = groupLabel(g);
    for (const item of (g.list ? g.list.items : [])) {
      const places = item.places || {};
      const eff = item.effective || (places.project ? 'project' : 'global');
      const doc = (places[eff] || {}).doc || {};
      const href = `/roles?open=${encodeURIComponent(token(item.name, group))}`;
      out.push({ kind: 'role', group, project: label, label: `${item.name} ${doc.title || ''}`.trim(), sub: '', text: `${item.name} ${doc.title || ''}`, href });
      for (const sec of doc.sections || []) {
        for (const rule of sec.rules || []) {
          out.push({ kind: 'rule', group, project: label, label: rule.id, sub: `ロール ${item.name}`, text: `${rule.id} ${rule.text}`, body: rule.text, href });
        }
      }
    }
  }
  for (const g of groupOf('profiles')) {
    const group = projectOfGroup(g), label = groupLabel(g);
    for (const item of (g.list ? g.list.items : [])) {
      const places = item.places || {};
      const eff = item.effective || (places.project ? 'project' : 'global');
      const doc = (places[eff] || {}).doc || {};
      const sum = item.summary || {};
      const kind = doc.kind || sum.kind || '';
      const args = (doc.args || []).join(' ') || sum.args || '';
      out.push({ kind: 'profile', group, project: label, label: item.name, sub: `${kind} ${args}`.trim(), text: `${item.name} ${kind} ${args}`, href: `/profiles?open=${encodeURIComponent(token(item.name, group))}` });
    }
  }
  return out;
}

const searchIndex = () => {
  if (!index || indexTeams !== st.groups.teams) { index = buildIndex(); indexTeams = st.groups.teams; }
  return index;
};

function liveEntries() {
  const out = [];
  for (const t of st.running.teams || []) {
    const wait = (t.waiting || []).length;
    out.push({
      kind: 'live', group: t.project, project: t.project_name || projectName(t.project),
      label: t.team, sub: `ワークスペース ${t.workspace.label}${wait ? ` · 許可待ち（${t.waiting.join(' · ')}）` : t.starting ? ' · 起動の途中' : ' · 動いている'}`,
      text: `${t.team} ${t.definition} ${t.workspace.label} ${t.project_name || ''}`,
      href: `/activity?open=${encodeURIComponent(token(t.team, t.project))}`,
    });
  }
  for (const w of st.workspaces || []) {
    const last = (st.running.teams || []).find((t) => t.workspace && t.workspace.id === w.id);
    out.push({
      kind: 'ws', group: last ? last.project : '', project: last ? (last.project_name || projectName(last.project)) : '',
      label: w.label,
      sub: last ? `${last.team} が動いている` : `hergy のチームはいない${w.last_team ? `（前は ${w.last_team}）` : ''}`,
      text: `${w.label} ${last ? last.team : ''} ${w.last_team || ''}`,
      href: last ? `/activity?open=${encodeURIComponent(token(last.team, last.project))}` : '',
    });
  }
  return out;
}

// marks is the words with the search's own letters marked: the three pieces a
// row shows, the match in a <mark>.
function marks(text, q) {
  const s = String(text == null ? '' : text);
  const i = s.toLowerCase().indexOf(q.toLowerCase());
  if (i < 0) return [s];
  return [s.slice(0, i), html`<mark>${s.slice(i, i + q.length)}</mark>`, s.slice(i + q.length)];
}
function snippet(text, q) {
  const s = String(text || '');
  const i = s.toLowerCase().indexOf(q.toLowerCase());
  if (i < 0) return [s.slice(0, 60)];
  const a = Math.max(0, i - 24), b = Math.min(s.length, i + q.length + 36);
  return [a > 0 ? '…' : '', ...marks(s.slice(a, b), q), b < s.length ? '…' : ''];
}

// ---- 検索の状態 --------------------------------------------------------
// q is the text in the band's search, pick the row the arrows stand on, and open
// whether the list stands: signals, so the list draws itself from them and no
// one has to remember to draw it (webui-components 1 章、5 章 2).
export const searchText = signal('');
export const searchPick = signal(0);
export const searchOpen = signal(false);

// searchLoading and searchGroups are what the list shows. The lists a search
// reads are read when it is opened, and a reading that comes later draws the
// list again: both read the headquarters' revision (lib/hq/state.js's rev).
export const searchLoading = computed(() => {
  rev.value;
  return !st.groupsLoaded;
});
export const searchGroups = computed(() => {
  const text = searchText.value.trim();
  if (!text) return [];
  rev.value;
  const low = text.toLowerCase();
  const all = liveEntries().concat(searchIndex()).filter((e) => e.text.toLowerCase().includes(low));
  return KINDS.map(([kind, title]) => ({ kind, title, items: all.filter((e) => e.kind === kind) })).filter((g) => g.items.length);
});
// searchShown is the rows the arrows walk, in the order they stand.
export const searchShown = computed(() => searchGroups.value.flatMap((g) => g.items.slice(0, CAP(g.kind))));

export const searchSel = () => searchPick.value;
export const searchBoxOpen = () => searchOpen.value;

// ---- 検索の一覧（部品） -------------------------------------------------
// Results is the one place the list is drawn in: it stands in the page once
// (mountResults) and draws the list while a search stands, or nothing at all.
function Results() {
  const box = useRef(null);
  const open = searchOpen.value;
  const text = searchText.value;
  const groups = searchGroups.value;
  const loading = searchLoading.value;
  const sel = searchPick.value;
  // The row the arrows stand on is brought into the list after the draw. The
  // list itself is never written by hand: the drawing is the signals' own.
  const was = useRef('');
  useEffect(() => {
    if (!open || !box.current) return;
    const at = `${text}\u0000${sel}`;
    if (was.current === at) return;
    was.current = at;
    const cur = box.current.querySelector('.hq-ri.sel');
    if (cur) cur.scrollIntoView({ block: 'nearest' });
  });
  if (!open) return null;
  const q = text.trim();
  let body;
  if (loading) body = html`<p class="hq-sub">読み込んでいます…</p>`;
  else if (!groups.length) body = html`<p class="hq-sub">${`「${q}」に合うものはありません。探すのは、並べたプロジェクトとグローバルの、チーム・ワークスペース・メンバー・ロール・規則・プロファイルです。`}</p>`;
  else {
    let i = 0;
    body = groups.map((g) => {
      const rows = g.items.slice(0, CAP(g.kind)).map((e) => {
        const n = i;
        i += 1;
        const inner = html`<span class="hq-rp">${e.project || '—'}</span><span class="hq-rl">${marks(e.label, q)}</span><span class="hq-rs">${e.body ? html`${snippet(e.body, q)} <span class="hq-rw">${e.sub}</span>` : marks(e.sub, q)}</span>`;
        const cls = 'hq-ri' + (n === sel ? ' sel' : '');
        return e.href
          ? html`<a class=${cls} data-i=${n} href=${e.href} role="option">${inner}</a>`
          : html`<div class=${cls} data-i=${n} role="option">${inner}</div>`;
      });
      return html`<div class="hq-srg"><div class="hq-srh">${g.title}<span class="n">${g.items.length}</span></div>${rows}${g.items.length > CAP(g.kind) ? html`<div class="hq-srm">ほか ${g.items.length - CAP(g.kind)} 件</div>` : ''}</div>`;
    });
  }
  return html`<div class="hq-sr" role="listbox" aria-label="探した結果" ref=${box}>${body}</div>`;
}

// mountResults stands the list's one place in the page, the way lib/deliver.js
// stands its layer: display: contents, so the band's row is not made to hold it.
function mountResults() {
  const host = document.createElement('div');
  host.className = 'hq-sr-host';
  host.style.display = 'contents';
  document.body.appendChild(host);
  draw(html`<${Results} />`, host);
}

// ---- 押し込み ----------------------------------------------------------
// pressSearch is the field's own press: the text is the part's state, and the
// lists are read the first time a search is opened (spec 3 章).
export function pressSearch(text) {
  searchText.value = String(text == null ? '' : text);
  searchPick.value = 0;
  const on = !!searchText.value.trim();
  searchOpen.value = on;
  if (on) readGroups();
}

export function closeBox() {
  searchOpen.value = false;
  searchPick.value = 0;
}

// moveSel walks the rows with the arrows; the row it lands on is brought into
// the list by the part itself.
export function moveSel(step) {
  const rows = searchShown.value;
  if (!rows.length) return;
  searchPick.value = Math.max(0, Math.min(rows.length - 1, searchPick.value + step));
}

export function mountSearch() {
  const input = document.getElementById('hq-q');
  if (!input) return;
  input.addEventListener('input', () => pressSearch(input.value));
  input.addEventListener('focus', () => { if (searchText.value.trim()) searchOpen.value = true; });
  mountResults();
}

export function resetSearch() {
  const input = document.getElementById('hq-q');
  if (input) input.value = '';
  searchText.value = '';
  searchPick.value = 0;
  searchOpen.value = false;
}

// A test hook, the way the other screens carry one: what the search has on show,
// so a walk of the screens can tell a key that did not land from one that did.
export const searchHook = () => ({ shown: searchShown.value.map((x) => x.href), sel: searchPick.value, box: searchBoxOpen() });
