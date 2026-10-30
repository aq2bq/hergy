// teams/canvas/attr.tsx — 構成図の右の列（属性）（spec-teams B7、n-6be7）。
//
// 選んだ点・線・ワーカーの属性を描く。書き込みはすべて画面の ops として
// canvas が state へ渡し、ここは読んで描くだけ（webui-preact-design 5 章 1）。
import type { ComponentChildren, JSX } from 'preact';
import { hasTurn, memberOf, nodeName, profilesFor, rolesFor, teamIssues } from '../state.js';
import type { Finding, Member, Rec, RoleName, Team, Worker } from '../state.js';
import { NAME, selOf, typedOf } from './shared.js';
import { MemberPrompt, WorkerPromptSection } from './prompt.js';
import { Field } from '../../lib/field.js';

// Opt is one choice of a select: the option carries the selected attribute a
// person (and the walks) read, and the property the browser picks by, so the two
// say the same thing (the markup hergy wrote before carried selected="").
function Opt({ v, on, children }: { v: string; on: boolean; children?: ComponentChildren }) {
  return <option value={v} selected={on} ref={(el: HTMLOptionElement | null) => { if (el) el.toggleAttribute('selected', !!on); }}>{children}</option>;
}

// ---- the right column: 属性 -----------------------------------------
// Change is one unsaved change: how it reads (+ ~ −) and the words it carries.
type Change = ['a' | 'c' | 'd', string];

function diffList(r: Rec): Change[] {
  const s = r.saved.value, t = r.draft.value, out: Change[] = [];
  const sm = new Map(s.members.map((m) => [m.name, m])), cm = new Map(t.members.map((m) => [m.name, m]));
  if (s.mission !== t.mission) out.push(['c', `使命: ${t.mission}`]);
  cm.forEach((m, n) => {
    if (!sm.has(n)) out.push(['a', `メンバー ${n}（${m.role} · ${m.profile}）`]);
    else {
      const o = sm.get(n);
      if (o && o.role !== m.role) out.push(['c', `${n} のロール ${o.role} → ${m.role}`]);
      if (o && o.profile !== m.profile) out.push(['c', `${n} のプロファイル ${o.profile} → ${m.profile}`]);
    }
  });
  sm.forEach((_m, n) => { if (!cm.has(n)) out.push(['d', `メンバー ${n}`]); });
  const sw = new Map(s.workers.map((w) => [w.from + '\u0000' + w.purpose, w])), cw = new Map(t.workers.map((w) => [w.from + '\u0000' + w.purpose, w]));
  cw.forEach((w, k) => {
    if (!sw.has(k)) out.push(['a', `ワーカー ${w.purpose || '（目的）'}（${w.from} · ${w.profile}）`]);
    else {
      const o = sw.get(k);
      if (o && (o.purpose !== w.purpose || o.profile !== w.profile || o.from !== w.from)) out.push(['c', `ワーカー ${w.purpose || '（目的）'}（${w.from} · ${w.profile}）`]);
    }
  });
  sw.forEach((w, k) => { if (!cw.has(k)) out.push(['d', `ワーカー ${w.purpose || '（目的）'}`]); });
  t.turns.forEach(([a, b]) => { if (!hasTurn(s, a, b)) out.push(['a', `ターン ${nodeName(a)} → ${nodeName(b)}`]); });
  s.turns.forEach(([a, b]) => { if (!hasTurn(t, a, b)) out.push(['d', `ターン ${nodeName(a)} → ${nodeName(b)}`]); });
  return out;
}

// turnRows is the turns that leave ('out') or reach ('in') one point.
function turnRows(t: Team, s: Team, id: string, dir: 'out' | 'in'): JSX.Element | JSX.Element[] {
  const list: [string, string][] = dir === 'out'
    ? t.turns.filter(([a]) => a === id).map(([, b]): [string, string] => [id, b])
    : t.turns.filter(([, b]) => b === id).map(([a]): [string, string] => [a, id]);
  if (!list.length) return <span class="cv-empty">なし</span>;
  return list.map(([a, b]) => (
    <div class={'cv-li' + (hasTurn(s, a, b) ? '' : ' new')}>
      <span>{dir === 'out' ? '→' : '←'} {nodeName(dir === 'out' ? b : a)}</span>
      <button class="cv-x" type="button" data-cv-act="turn-del" data-a={a} data-b={b} aria-label={`${nodeName(a)} から ${nodeName(b)} へのターンを外す`} title="外す">×</button>
    </div>
  ));
}

// addTurnSel is the select that adds a turn out of one point.
function addTurnSel(t: Team, id: string): JSX.Element | '' {
  const cand = ['human', ...t.members.map((m) => m.name)].filter((x) => x !== id && !hasTurn(t, id, x));
  if (!cand.length) return '';
  return (
    <span class="sel">
      <select data-cv-f="turn-add">
        <option value="">＋ 渡す相手を足す</option>
        {cand.map((c) => <option value={c}>{nodeName(c)}</option>)}
      </select>
    </span>
  );
}

function fieldFindings(r: Rec, id: string): Finding[] {
  return (teamIssues(r) || []).filter((m) => m.node === id && (m.field === 'members' || m.field === 'name' || m.field === 'node'));
}

function memberMsgs(r: Rec, t: Team, m: Member): string[] {
  const msgs: string[] = [];
  if (!NAME.test(m.name)) msgs.push('英小文字で始め、英小文字・数字・ハイフン・下線で 32 文字以内にしてください');
  if (t.members.filter((x) => x.name === m.name).length > 1) msgs.push('このチームにもういます');
  if (`${r.name.value}-${m.name}`.length > 32) msgs.push(`herdr 名 ${r.name.value}-${m.name} が 32 文字を超えます`);
  return msgs;
}

export function SideAttr({ r }: { r: Rec }): JSX.Element | null {
  const t = r.draft.value, s = r.saved.value, sel = selOf(r), team = r.name.value;
  if (!sel) {
    const d = diffList(r), humanIn = t.turns.some(([, b]) => b === 'human');
    return (
      <>
        <div class="cv-t"><code>{team}</code><span class="cv-k">チーム定義</span></div>
        <div class="cv-f"><label for={`cv-mi-${team}`}>使命</label><Field id={`cv-mi-${team}`} class="cv-body" data-cv-f="mission" value={t.mission} autocomplete="off" spellcheck={false} /></div>
        <div class="cv-note">メンバー {t.members.length} · ワーカー {t.workers.length} · ターン {t.turns.length}</div>
        {humanIn ? '' : <div class="cv-warn">人間に届くターンがありません。人間の点の縁の丸から、または人間へ渡すメンバーの縁の丸から、線を引いてください</div>}
        <div class="cv-sec">
          <span class="cv-sl">保存していない変更 {d.length}</span>
          {d.length ? <div class="cv-diff">{d.map(([k, x]) => <span class={'cv-' + k}>{k === 'a' ? '+ ' : k === 'd' ? '− ' : '~ '}{x}</span>)}</div> : <span class="cv-empty">なし</span>}
        </div>
        <div class="cv-sec"><span class="cv-sl">操作</span><span class="cv-note">点や線を押すと、ここにその属性が出る。選んだ点の縁の丸から、ほかの点へ引くとターンができる。<span class="cv-kbd">Delete</span> で外す、<span class="cv-kbd">Esc</span> で選ぶのをやめる、<span class="cv-kbd">⌘Z</span> で 1 つ取り消す。</span></div>
      </>
    );
  }
  if (sel.type === 'edge') {
    const [a, b] = sel.k.split('|');
    const row = (p: string, q: string) => (hasTurn(t, p, q)
      ? <div class={'cv-li' + (hasTurn(s, p, q) ? '' : ' new')}><span>{nodeName(p)} → {nodeName(q)}</span><button class="cv-x" type="button" data-cv-act="turn-del" data-a={p} data-b={q} title="外す" aria-label="外す">×</button></div>
      : <div class="cv-li"><span class="cv-empty">{nodeName(p)} → {nodeName(q)}</span><button class="btn sm" type="button" data-cv-act="turn-add" data-a={p} data-b={q}>この向きも足す</button></div>);
    return (
      <>
        <div class="cv-t"><code>{`${nodeName(a)} ⇄ ${nodeName(b)}`}</code><span class="cv-k">ターン</span></div>
        <div class="cv-sec"><span class="cv-sl">向き</span>{row(a, b)}{row(b, a)}</div>
        <div class="cv-note"><span class="cv-kbd">Delete</span> で両方の向きを外す。</div>
      </>
    );
  }
  if (sel.type === 'worker') {
    const w = t.workers[sel.i];
    if (!w) return null;
    const profs = (profilesFor(r).includes(w.profile) ? [] : [w.profile]).concat(profilesFor(r));
    return (
      <>
        <div class="cv-t"><code>{w.purpose || '（目的）'}</code><span class="cv-k">ワーカー</span></div>
        <div class="cv-f"><label for={`cv-wp-${team}`}>目的</label><Field id={`cv-wp-${team}`} data-cv-f="w-purpose" value={w.purpose} placeholder="例: 調べ物" autocomplete="off" spellcheck={false} /></div>
        <div class="cv-f">
          <label for={`cv-wf-${team}`}>追加するメンバー</label>
          <span class="sel"><select id={`cv-wf-${team}`} data-cv-f="w-from">{t.members.map((m) => <Opt v={m.name} on={m.name === w.from}>{m.name}</Opt>)}</select></span>
        </div>
        <div class="cv-f">
          <label for={`cv-wpf-${team}`}>プロファイル</label>
          <span class="sel"><select id={`cv-wpf-${team}`} data-cv-f="w-profile">{profs.map((p) => <Opt v={p} on={p === w.profile}>{p}</Opt>)}</select></span>
          <span class="cv-sub">ワーカーは規則を受けない。安いプロファイルで</span>
        </div>
        <WorkerPromptSection r={r} w={w} i={sel.i} />
        <button class="btn danger sm cv-danger" type="button" data-cv-act="del">ワーカーを外す</button>
      </>
    );
  }
  const id = sel.id;
  if (id === 'human') {
    return (
      <>
        <div class="cv-t"><code>人間</code><span class="cv-k">あなた。チームに 1 人だけで、外せない</span></div>
        <div class="cv-sec"><span class="cv-sl">ターンを渡す相手</span>{turnRows(t, s, id, 'out')}{addTurnSel(t, id)}</div>
        <div class="cv-sec"><span class="cv-sl">ターンを受け取る相手</span>{turnRows(t, s, id, 'in')}</div>
      </>
    );
  }
  const m = memberOf(t, id);
  if (!m) return null;
  // The field holds what a person is typing: a name the server would refuse is
  // not put into the draft, so the part keeps the typed letters itself and draws
  // them with the words beside them (5 章 2).
  const typed = typedOf(r);
  const mine = typed && typed.id === id ? typed : null;
  const msgs = (mine ? mine.msgs : memberMsgs(r, t, m)).concat(fieldFindings(r, id).map((x) => x.text));
  const outs = t.turns.filter(([a]) => a === id).map(([, b]) => b);
  const ws: { w: Worker; i: number }[] = t.workers.map((w, i) => ({ w, i })).filter((z) => z.w.from === id);
  const nt = t.turns.filter(([a, b]) => a === id || b === id).length;
  const role: RoleName | undefined = (rolesFor(r) || []).find((x) => x.name === m.role);
  return (
    <>
      <div class="cv-t"><code>{m.name}</code><span class="cv-k">メンバー{role && role.title ? ` · ${role.title}` : ''}</span></div>
      <div class="cv-f">
        <label for={`cv-mn-${team}`}>メンバー名</label>
        <Field id={`cv-mn-${team}`} data-cv-f="m-name" value={mine ? mine.value : m.name} spellcheck={false} autocomplete="off" />
        {msgs.map((x) => <span class="cv-msg">{x}</span>)}
        <span class="cv-sub">herdr 名 {`${team}-${m.name}`}</span>
      </div>
      <div class="cv-f">
        <label for={`cv-mr-${team}`}>ロール</label>
        <span class="sel"><select id={`cv-mr-${team}`} data-cv-f="m-role">{role ? null : <option value="" disabled selected>ロールを選ぶ</option>}{(rolesFor(r) || []).map((x) => <Opt v={x.name} on={x.name === m.role}>{x.name}{x.title ? ` · ${x.title}` : ''}</Opt>)}</select></span>
        <span class="cv-sub">無いロールは <a href="/roles">ロールの画面</a> で先に作る</span>
      </div>
      <div class="cv-f">
        <label for={`cv-mp-${team}`}>プロファイル</label>
        <span class="sel"><select id={`cv-mp-${team}`} data-cv-f="m-profile">{profilesFor(r).includes(m.profile) ? null : <option value="" disabled selected>プロファイルを選ぶ</option>}{profilesFor(r).map((p) => <Opt v={p} on={p === m.profile}>{p}</Opt>)}</select></span>
      </div>
      <div class="cv-sec">
        <span class="cv-sl">ターンを渡す相手</span>{turnRows(t, s, id, 'out')}
        {outs.length ? '' : <span class="cv-note">まだ誰にもターンを渡していません。縁の丸から、渡す相手の点へ引いてください</span>}
        {addTurnSel(t, id)}
      </div>
      <div class="cv-sec"><span class="cv-sl">ターンを受け取る相手</span>{turnRows(t, s, id, 'in')}</div>
      <div class="cv-sec">
        <span class="cv-sl">ワーカー</span>
        {ws.length ? ws.map(({ w, i }) => (
          <div class={'cv-li' + (s.workers.some((x) => x.from === w.from && x.purpose === w.purpose) ? '' : ' new')}>
            <span>{`${w.purpose || '（目的）'} · ${w.profile}`}</span>
            <span class="cv-liend"><button class="cv-link" type="button" data-cv-act="pick-w" data-i={i}>選ぶ</button><button class="cv-x" type="button" data-cv-act="w-del" data-i={i} title="外す" aria-label="外す">×</button></span>
          </div>
        )) : <span class="cv-empty">なし</span>}
        <button class="btn sm cv-danger" type="button" data-cv-act="w-add">＋ ワーカー</button>
      </div>
      <MemberPrompt r={r} m={m} />
      <div class="cv-sec"><button class="btn sm" type="button" data-cv-act="m-copy">複製</button><span class="cv-note">同じロール・プロファイル・ターンの相手・ワーカーの目的で、もう 1 人足す</span></div>
      <div class="cv-sec"><button class="btn danger sm cv-danger" type="button" data-cv-act="del">メンバーを外す</button><span class="cv-note">ターン {nt} 本とワーカー {ws.length} 件も一緒に外れる。<span class="cv-kbd">⌘Z</span> で取り消せる。</span></div>
    </>
  );
}
