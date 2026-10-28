// lib/apply/view.tsx — 反映の見た目（n-4b22、設計は apply-design.md）。
//
// 帯のボタン、反映の欄（チーム・メンバー・反映の仕方の札・変わったこと・選ぶ）、
// 下の知らせ。どれも Preact が描く。押し込みは部品が自分で受ける（シートは画面から
// 出た所に立つ 1 枚なので、画面の委譲と二重にならない）。
import { useRef, useEffect } from '../preact.js';
import type { JSX } from 'preact';
import type { components } from '../../../../apicontract/api-types';

type ApplyChange = components['schemas']['ApplyChange'];
type ApplyMember = components['schemas']['ApplyMember'];
type ApplyTeam = components['schemas']['ApplyTeam'];
import {
  count, opened, answer, note, chosen, toast, toastOn, scope,
  teams, canApply, applyLabel, toggle, close, send, openSheet,
} from './state.js';

// 状態の字。サーバは状態の名だけを運び、字はここで決める。
const STATE: Record<string, string> = { idle: '止まっている', handed: '人間に渡した', active: '仕事中', blocked: '許可待ち', absent: '動いていない' };

// HOW is the mark of the way hergy would apply one member's change (d-3d30):
// 送り直す keeps the conversation, 起動し直す loses it, and 反映できない says why.
const HOW: Record<string, string> = { resend: '送り直す', start: '起動する', restart: '起動し直す', cannot: '反映できない' };
const INTRO = 'チェックした人に、定義の変更を反映します。送り直すは会話が残り、起動し直すは会話が消えます。仕事中と許可待ちの人には、今は反映できません。';

// press keeps a press out of the page underneath and calls the action.
const press = (fn: () => unknown) => (e: { preventDefault: () => void }): void => { e.preventDefault(); fn(); };

// ---- 帯のボタン ----------------------------------------------------------
// 画面の切り替えの右隣に、独立したボタン。数が見える所を、そのまま押す所に
// する。0 のときは出さない。
export function BandButton(): JSX.Element {
  const n = count.value;
  return <button
    type="button"
    class="apl-bandbtn"
    data-apl="open"
    aria-haspopup="dialog"
    title="定義を直したあと、まだ反映していない人"
    aria-label={`反映していない変更 ${n}`}
    hidden={n <= 0}
    onClick={press(openSheet)}
  >反映していない変更 <b>{n}</b></button>;
}

// ---- 変わったことの行き先 ------------------------------------------------
// The server names each change's document by the path a screen shows
// (".hergy/roles/builder.yaml", ".hergy/teams/core/agreement.yaml"). A role's
// rule opens on the role screen, the team's own rules on its rules tab, and
// everything else a member reads opens on its team definition.
const baseName = (p: string | undefined): string => String(p == null ? '' : p).replace(/\\/g, '/').split('/').pop() || '';
function roleOf(path: string | undefined): string {
  const m = /(^|\/)roles\/([^/]+)$/.exec(String(path == null ? '' : path).replace(/\\/g, '/'));
  return m ? m[2].replace(/\.(ya?ml|md)$/, '') : '';
}
function teamOf(path: string | undefined): string {
  const m = /(^|\/)teams\/([^/]+)\/[^/]+$/.exec(String(path == null ? '' : path).replace(/\\/g, '/'));
  return m ? m[2] : '';
}
const enc = (s: string): string => encodeURIComponent(String(s == null ? '' : s));

function linkFor(change: ApplyChange, group: ApplyTeam): string {
  const kind = change.kind || '';
  if (kind === 'turn' || kind === 'worker' || kind === 'roster' || kind === 'template' || kind === 'prompt') {
    return `/teams?open=${enc(group.definition || group.team)}`;
  }
  if (kind === 'rule' || kind === 'document') {
    const role = roleOf(change.document);
    if (role) return `/roles?open=${enc(role)}${kind === 'rule' && change.rule ? `&rule=${enc(change.rule)}` : ''}`;
    const team = teamOf(change.document) || group.definition || group.team;
    const tab = kind === 'rule' || /agreement\.ya?ml$/.test(baseName(change.document)) ? `&agree=${enc(team)}` : '';
    return `/teams?open=${enc(team)}${tab}`;
  }
  return '';
}

// ---- 欄 ----------------------------------------------------------------
// The sheet takes the focus when it stands, because a person just opened it
// (5 章 5). It is one element for as long as it is open, so nothing here puts
// the focus back after a redraw.
function Sheet(): JSX.Element | null {
  const box = useRef<HTMLElement>(null);
  useEffect(() => {
    const el = box.current;
    if (el) el.focus({ preventScroll: true });
  }, []);
  const a = answer.value;
  if (!a) return null;
  const groups = teams.value;
  return <section class="apl-sheet" role="dialog" aria-label="反映" tabIndex={-1} ref={box}>
    <div class="apl-h"><b>反映{scope.value ? `: ${scope.value}` : ''}</b><button class="apl-x" type="button" data-apl="close" aria-label="閉じる" onClick={press(close)}>×</button></div>
    <div class="apl-intro">{INTRO}</div>
    <p class="apl-note err" data-apl-note hidden={!note.value}>{note.value}</p>
    {groups.length ? groups.map((g) => <Team group={g} />) : <p class="apl-empty">反映していない変更はありません。</p>}
    <div class="apl-acts"><button class="apl-btn apl-primary" type="button" data-apl="send" disabled={!canApply.value} onClick={press(send)}>{applyLabel.value}</button><button class="apl-btn" type="button" data-apl="close" onClick={press(close)}>やめる</button></div>
  </section>;
}

function Team({ group }: { group: ApplyTeam }): JSX.Element {
  const def = group.definition && group.definition !== group.team
    ? <>（定義 <code>{group.definition}</code>）</>
    : '';
  const where = group.project_name ? <span class="apl-where">{group.project_name}</span> : '';
  const notes = (group.notes || []).map((t) => <p class="apl-tnote">{t}</p>);
  // A definition that cannot be built has no member to apply: the team's reason
  // stands in the same words hergy status uses (the row of 動き says the same).
  const broken = group.error ? <p class="apl-err" data-role="definition-error">{group.error}</p> : '';
  return <div class="apl-team">
    <div class="apl-th">チーム <code>{group.team}</code>{def}{where}</div>
    {broken}
    {notes}
    <div class="apl-ms">{(group.members || []).map((m) => <Member m={m} group={group} />)}</div>
  </div>;
}

function Member({ m, group }: { m: ApplyMember; group: ApplyTeam }): JSX.Element {
  const can = !!m.can_apply;
  const how = HOW[m.how] || m.how;
  return <label class={can ? 'apl-mm' : 'apl-mm no'}>
    <input type="checkbox" data-apl-name={m.name} checked={can && chosen.value.has(m.name)} disabled={!can} onChange={() => toggle(m.name)} />
    <span class="apl-who"><code>{m.name}</code><span class="apl-st">{STATE[m.state] || m.state}</span>
      <span class={'apl-how ' + m.how}>{how}</span>
      {m.conversation_lost ? <span class="apl-lost">会話が消えます</span> : null}
    </span>
    {m.how === 'cannot' && m.why_not ? <span class="apl-why">{m.why_not}</span> : null}
    <span class="apl-what">{(m.changes || []).map((c) => {
      const href = linkFor(c, group);
      const text = c.text || '';
      return <span>{href ? <a class="apl-chg" href={href}>{text}</a> : text}</span>;
    })}</span>
  </label>;
}

// ---- 下の知らせ ----------------------------------------------------------
export function Toast(): JSX.Element {
  return <div class="apl-toast" role="status" hidden={!toastOn.value}>{toast.value}</div>;
}

// Layer is everything the part puts on the page that is not the band's button:
// the sheet while it stands, and the notice.
export function Layer(): JSX.Element {
  return <>{opened.value && answer.value ? <Sheet /> : null}<Toast /></>;
}
