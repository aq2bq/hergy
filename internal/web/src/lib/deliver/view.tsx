// lib/deliver/view.tsx — 変更を伝える の見た目（r-a8fa）。
//
// 帯のボタン、シート（チーム・メンバー・変わる所・足）、下の知らせ。どれも
// Preact が描き、class・data-*・文言は移す前のまま（webui-preact-design 5 章 6）。
// 押し込みは部品が自分で受ける（シートは画面から出た所に立つ 1 枚なので、
// 画面の委譲と二重にならない）。
import { useRef, useEffect } from '../preact.js';
import type { JSX } from 'preact';
import type { components } from '../../../../apicontract/api-types';
import {
  count, opened, answer, note, chosen, toast, toastOn,
  canSend, sendLabel, toggle, close, send, openSheet,
} from './state.js';

type DeliveryTeam = components['schemas']['DeliveryTeam'];
type DeliveryMember = components['schemas']['DeliveryMember'];
type DeliveryChange = components['schemas']['DeliveryChange'];

// 状態の字（brief 2）。サーバは状態の名だけを運び、字はここで決める。
const STATE: Record<string, string> = { idle: '止まっている', handed: '人間に渡した', active: '仕事中', blocked: '許可待ち', absent: '動いていない' };
const INTRO = 'チェックした人に、初期プロンプトを送り直します（それまでの会話は残ります）。仕事中と許可待ちの人には送れません。';

// press keeps a press out of the page underneath and calls the action.
const press = (fn: () => unknown) => (e: { preventDefault: () => void }): void => { e.preventDefault(); fn(); };

// ---- 帯のボタン ----------------------------------------------------------
// 画面の切り替えの右隣に、独立したボタン。数が見える所を、そのまま押す所に
// する（試作 3 回目）。0 のときは出さない。
export function BandButton(): JSX.Element {
  const n = count.value;
  return <button
    type="button"
    class="dlv-bandbtn"
    data-dlv="open"
    aria-haspopup="dialog"
    title="定義を直したあと、まだ伝えていない人"
    aria-label={`変更を伝えていない人 ${n}`}
    hidden={n <= 0}
    onClick={press(openSheet)}
  >変更を伝えていない人 <b>{n}</b></button>;
}

// ---- 変わる所の行き先 ----------------------------------------------------
// The server names each change's document by the path a screen shows
// (".hergy/roles/builder.yaml", "~/.config/hergy/roles/scout.yaml",
// ".hergy/teams/core/agreement.yaml"). A role's rule opens on the role screen,
// the team's own rules on its rules tab, and everything else a member reads
// opens on its team definition (brief 2).
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

function linkFor(change: DeliveryChange, group: DeliveryTeam): string {
  const kind = change.kind || '';
  if (kind === 'turn' || kind === 'worker' || kind === 'roster' || kind === 'template' || kind === 'prompt') {
    return `/teams?open=${enc(group.definition || group.name)}`;
  }
  if (kind === 'rule' || kind === 'document') {
    const role = roleOf(change.document);
    if (role) return `/roles?open=${enc(role)}${kind === 'rule' && change.rule ? `&rule=${enc(change.rule)}` : ''}`;
    const team = teamOf(change.document) || group.definition || group.name;
    const tab = kind === 'rule' || /agreement\.ya?ml$/.test(baseName(change.document)) ? `&agree=${enc(team)}` : '';
    return `/teams?open=${enc(team)}${tab}`;
  }
  return '';
}

// ---- シート --------------------------------------------------------------
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
  return <section class="dlv-sheet" role="dialog" aria-label="変更を伝える" tabIndex={-1} ref={box}>
    <div class="dlv-h"><b>変更を伝える</b><button class="dlv-x" type="button" data-dlv="close" aria-label="閉じる" onClick={press(close)}>×</button></div>
    <div class="dlv-sub2">{INTRO}</div>
    <p class="dlv-note err" data-dlv-note hidden={!note.value}>{note.value}</p>
    {(a.teams || []).map((g) => <Team group={g} />)}
    <div class="dlv-acts"><button class="dlv-btn dlv-primary" type="button" data-dlv="send" disabled={!canSend.value} onClick={press(send)}>{sendLabel.value}</button><button class="dlv-btn" type="button" data-dlv="close" onClick={press(close)}>やめる</button></div>
  </section>;
}

function Team({ group }: { group: DeliveryTeam }): JSX.Element {
  const def = group.definition && group.definition !== group.name
    ? <>（定義 <code>{group.definition}</code>）</>
    : '';
  return <div class="dlv-team">
    <div class="dlv-th">チーム <code>{group.name}</code>{def}</div>
    <div class="dlv-ms">{(group.members || []).map((m) => <Member m={m} group={group} />)}</div>
  </div>;
}

function Member({ m, group }: { m: DeliveryMember; group: DeliveryTeam }): JSX.Element {
  const can = !!m.can_send;
  return <label class={can ? 'dlv-mm' : 'dlv-mm no'}>
    <input type="checkbox" data-dlv-name={m.name} checked={can && chosen.value.has(m.name)} disabled={!can} onChange={() => toggle(m.name)} />
    <span class="dlv-who"><code>{m.name}</code><span class="dlv-st">{STATE[m.state] || m.state}</span>{can ? null : <span class="dlv-wait">仕事が終わると伝えられます</span>}</span>
    <span class="dlv-what">{(m.changes || []).map((c) => {
      const href = linkFor(c, group);
      const text = c.text || '';
      return <span>{href ? <a class="dlv-chg" href={href}>{text}</a> : text}</span>;
    })}</span>
  </label>;
}

// ---- 下の知らせ ----------------------------------------------------------
export function Toast(): JSX.Element {
  return <div class="dlv-toast" role="status" hidden={!toastOn.value}>{toast.value}</div>;
}

// Layer is everything the part puts on the page that is not the band's button:
// the sheet while it stands, and the notice.
export function Layer(): JSX.Element {
  return <>{opened.value && answer.value ? <Sheet /> : null}<Toast /></>;
}
