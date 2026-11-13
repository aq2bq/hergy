import { turnRecipientLabel } from '../../lib/screen-words.js';
import { term } from '../../lib/terms.generated.js';
import { dangerAttrs } from '../../lib/danger.js';
// teams/canvas/attr.tsx — 構成図の右の列（属性）（spec-teams B7、n-6be7）。
//
// 選んだ点・線・ワーカーの属性を描く。書き込みはすべて画面の ops として
// canvas が state へ渡し、ここは読んで描くだけ（webui-preact-design 5 章 1）。
import type { ComponentChildren, JSX } from 'preact';
import { hasTurn, memberOf, nodeName, profilesFor, rolesFor, teamIssues } from '../state.js';
import type { Finding, Member, Rec, RoleName, Team, Worker } from '../state.js';
import { NAME, typedOf } from './shared.js';
import { pickOf } from '../selection.js';
import { Field } from '../../lib/field.js';
import type { TeamChanges } from '../../lib/editing.js';
import { RoleButton } from '../role-modal.js';

// Opt is one choice of a select: the option carries the selected attribute a
// person (and the walks) read, and the property the browser picks by, so the two
// say the same thing (the markup hergy wrote before carried selected="").
function Opt({ v, on, children }: { v: string; on: boolean; children?: ComponentChildren }) {
  return <option value={v} selected={on} ref={(el: HTMLOptionElement | null) => { if (el) el.toggleAttribute('selected', !!on); }}>{children}</option>;
}

// ---- the right column: 属性 -----------------------------------------
// turnRows is the turns that leave ('out') or reach ('in') one point.
function turnRows(changes: TeamChanges, id: string, dir: 'out' | 'in'): JSX.Element | JSX.Element[] {
  const list: [string, string][] = dir === 'out'
    ? changes.turns.filter((t) => t.from === id).map((t): [string, string] => [id, t.to])
    : changes.turns.filter((t) => t.to === id).map((t): [string, string] => [t.from, id]);
  if (!list.length) return <span class="cv-empty">なし</span>;
  return list.map(([a, b]) => (
    <div class={'cv-li' + (changes.turns.some((t) => t.from === a && t.to === b && t.added) ? ' new' : '')}>
      <span>{dir === 'out' ? '→' : '←'} {nodeName(dir === 'out' ? b : a)}</span>
      <button class="cv-x" type="button" data-cv-act="turn-del" {...dangerAttrs('removeTurn')} data-a={a} data-b={b} aria-label={`${nodeName(a)} から ${nodeName(b)} への${term('turn')}を外す`} title="外す">×</button>
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
  if (t.members.filter((x) => x.name === m.name).length > 1) msgs.push(`この${term('team')}にもういます`);
  if (`${r.name.value}-${m.name}`.length > 32) msgs.push(`${term('herdr-name')} ${r.name.value}-${m.name} が 32 文字を超えます`);
  return msgs;
}

export function SideAttr({ r }: { r: Rec }): JSX.Element | null {
  const t = r.draft.value, changes = r.editing.changes.value, sel = pickOf(r), team = r.name.value;
  if (!sel) {
    const d = changes.list, humanIn = t.turns.some(([, b]) => b === 'human');
    return (
      <>
        <div class="cv-t"><code>{team}</code><span class="cv-k">{term('team-definition')}</span></div>
        <div class="cv-f"><label for={`cv-mi-${r.uid}`}>{term('description')}</label><Field id={`cv-mi-${r.uid}`} class="cv-body in-description" data-cv-f="description" value={t.description} autocomplete="off" spellcheck={false} /></div>
        <div class="cv-note">{`${term('member')} `}{t.members.length}{` · ${term('worker')} `}{t.workers.length}{` · ${term('turn')} `}{t.turns.length}</div>
        {humanIn ? '' : <div class="cv-warn">{`${term('human')}に届く${term('turn')}がありません。${term('human')}の点の縁の丸から、または${term('human')}へ渡す${term('member')}の縁の丸から、線を引いてください`}</div>}
        {d.length ? <div class="cv-sec">
          <span class="cv-sl">保存していない変更 {d.length}</span>
          <div class="cv-diff">{d.map(([k, x]) => <span class={'cv-' + k}>{k === 'a' ? '+ ' : k === 'd' ? '− ' : '~ '}{x}</span>)}</div>
        </div> : null}
        <div class="cv-sec"><span class="cv-sl">操作</span><span class="cv-note">{`選んだ点の縁の丸から、ほかの点へ引くと${term('turn')}ができる。`}<span class="cv-kbd">Delete</span> で外す、<span class="cv-kbd">Esc</span> で選ぶのをやめる、<span class="cv-kbd">⌘Z</span>{` で 1 つ${term('undo')}。`}</span></div>
      </>
    );
  }
  if (sel.type === 'edge') {
    const [a, b] = sel.k.split('|');
    const row = (p: string, q: string) => (hasTurn(t, p, q)
      ? <div class={'cv-li' + (changes.turns.some((t) => t.from === p && t.to === q && t.added) ? ' new' : '')}><span>{nodeName(p)} → {nodeName(q)}</span><button class="cv-x" type="button" data-cv-act="turn-del" {...dangerAttrs('removeTurn')} data-a={p} data-b={q} title="外す" aria-label="外す">×</button></div>
      : <div class="cv-li"><span class="cv-empty">{nodeName(p)} → {nodeName(q)}</span><button class="btn sm" type="button" data-cv-act="turn-add" data-a={p} data-b={q}>この向きも足す</button></div>);
    return (
      <>
        <div class="cv-t"><code>{`${nodeName(a)} ⇄ ${nodeName(b)}`}</code><span class="cv-k">{term('turn')}</span></div>
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
        <div class="cv-t"><code>{w.purpose || `（${term('purpose')}）`}</code><span class="cv-k">{term('worker')}</span></div>
        <div class="cv-f"><label for={`cv-wp-${r.uid}`}>{term('purpose')}</label><Field id={`cv-wp-${r.uid}`} data-cv-f="w-purpose" value={w.purpose} placeholder="例: 調べ物" autocomplete="off" spellcheck={false} /></div>
        <div class="cv-f">
          <label for={`cv-wf-${r.uid}`}>{`追加する${term('member')}`}</label>
          <span class="sel"><select id={`cv-wf-${r.uid}`} data-cv-f="w-from">{t.members.map((m) => <Opt v={m.name} on={m.name === w.from}>{m.name}</Opt>)}</select></span>
        </div>
        <div class="cv-f">
          <label for={`cv-wpf-${r.uid}`}>{term('profile')}</label>
          <span class="sel"><select id={`cv-wpf-${r.uid}`} data-cv-f="w-profile">{profs.map((p) => <Opt v={p} on={p === w.profile}>{p}</Opt>)}</select></span>
          <span class="cv-sub">{`${term('worker')}は${term('rule')}を受けない。`}</span>
        </div>
        <button class="btn danger sm cv-danger" type="button" data-cv-act="del" {...dangerAttrs('removeWorker')}>{`${term('worker')}を外す`}</button>
      </>
    );
  }
  const id = sel.id;
  if (id === 'human') {
    return (
      <>
        <div class="cv-t"><code>{term('human')}</code><span class="cv-k">{`あなた。${term('team')}に 1 人だけで、外せない`}</span></div>
        <div class="cv-sec"><span class="cv-sl">{turnRecipientLabel()}</span>{turnRows(changes, id, 'out')}{addTurnSel(t, id)}</div>
        <div class="cv-sec"><span class="cv-sl">{`${term('turn')}を受け取る相手`}</span>{turnRows(changes, id, 'in')}</div>
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
      <div class="cv-t"><code>{m.name}</code><span class="cv-k">{term('member')}{role && role.title ? ` · ${role.title}` : ''}</span></div>
      <div class="cv-f">
        <label for={`cv-mn-${r.uid}`}>{term('member-name')}</label>
        <Field id={`cv-mn-${r.uid}`} data-cv-f="m-name" value={mine ? mine.value : m.name} spellcheck={false} autocomplete="off" />
        {msgs.map((x) => <span class="cv-msg">{x}</span>)}
        <span class="cv-sub">{`${term('herdr-name')} `}{`${team}-${m.name}`}</span>
      </div>
      <div class="cv-f">
        <label for={`cv-mr-${r.uid}`}>{term('role')}</label>
        <span class="sel"><select id={`cv-mr-${r.uid}`} data-cv-f="m-role">{role ? null : <option value="" disabled selected>{`${term('role')}を選ぶ`}</option>}{(rolesFor(r) || []).map((x) => <Opt v={x.name} on={x.name === m.role}>{x.name}{x.title ? ` · ${x.title}` : ''}</Opt>)}</select></span>
        <RoleButton r={r} name={m.role} />
      </div>
      <div class="cv-f">
        <label for={`cv-mp-${r.uid}`}>{term('profile')}</label>
        <span class="sel"><select id={`cv-mp-${r.uid}`} data-cv-f="m-profile">{profilesFor(r).includes(m.profile) ? null : <option value="" disabled selected>{`${term('profile')}を選ぶ`}</option>}{profilesFor(r).map((p) => <Opt v={p} on={p === m.profile}>{p}</Opt>)}</select></span>
      </div>
      <div class="cv-sec">
        <span class="cv-sl">{turnRecipientLabel()}</span>{turnRows(changes, id, 'out')}
        {outs.length ? '' : <span class="cv-note">{`まだ誰にも${term('turn')}を渡していません。縁の丸から、渡す相手の点へ引いてください`}</span>}
        {addTurnSel(t, id)}
      </div>
      <div class="cv-sec"><span class="cv-sl">{`${term('turn')}を受け取る相手`}</span>{turnRows(changes, id, 'in')}</div>
      <div class="cv-sec">
        <span class="cv-sl">{term('worker')}</span>
        {ws.length ? ws.map(({ w, i }) => (
          <div class={'cv-li' + (changes.workers[i]?.added ? ' new' : '')}>
            <span>{`${w.purpose || `（${term('purpose')}）`} · ${w.profile}`}</span>
            <span class="cv-liend"><button class="cv-link" type="button" data-cv-act="pick-w" data-i={i}>選ぶ</button><button class="cv-x" type="button" data-cv-act="w-del" {...dangerAttrs('removeWorker')} data-i={i} title="外す" aria-label="外す">×</button></span>
          </div>
        )) : <span class="cv-empty">なし</span>}
        <button class="btn sm cv-danger" type="button" data-cv-act="w-add">{`＋ ${term('worker')}`}</button>
      </div>
      <div class="cv-sec"><button class="btn sm" type="button" data-cv-act="m-copy">複製</button><span class="cv-note">{`同じ${term('role')}・${term('profile')}・${term('turn')}の相手・${term('worker')}の${term('purpose')}で、もう 1 人足す`}</span></div>
      <div class="cv-sec"><button class="btn danger sm cv-danger" type="button" data-cv-act="del" {...dangerAttrs('removeMember')}>{`${term('member')}を外す`}</button><span class="cv-note">{`${term('turn')} `}{nt}{` 本と${term('worker')} `}{ws.length} 件も一緒に外れる。</span></div>
    </>
  );
}
