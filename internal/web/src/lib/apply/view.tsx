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
  count, opened, answer, note, chosen, scope,
  teams, canApply, applyLabel, toggle, close, send, openSheet,
} from './state.js';

// 状態の語と、何が起きているか・反映にどう関わるかの文（d-7b45）。語は枠の
// 無い字で、文はそのまま知らせとして置く（押せない）。
const STATE: Record<string, { word: string; why: string }> = {
  idle: { word: '止まっている', why: 'ターンを持たずに待っています。今、反映できます。' },
  handed: { word: '人間の返事待ち', why: 'ターンを人間に渡して、返事を待っています。今、反映できます。' },
  active: { word: '仕事中', why: '仕事が終わると、反映できます。' },
  blocked: { word: '許可待ち', why: 'ペインで許可を求めています。答えると、反映できます。' },
  absent: { word: '動いていない', why: 'このメンバーのペインがありません。起動すると、定義のとおりに加わります。' },
};

// HOW is the mark of the way hergy would apply one member's change (d-3d30),
// written as the check's own name: a noun, so it is not read as a button
// (d-7b45). The parenthesis says what happens to the conversation.
const HOW: Record<string, string> = { resend: '送り直し', start: '起動', restart: '起動し直し' };
const COST: Record<string, string> = { resend: '（会話は残る）', start: '（新しいペインで始める）', restart: '（会話が消える）' };
const INTRO = 'チェックしたメンバーに、保存した定義の変更を反映します。送り直しは会話が残り、起動し直しは会話が消えます。仕事中と許可待ちのメンバーには、今は反映できません。チーム定義の説明だけを変えたときは、ずれが出ないので、ここには出ません。';

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
  if (kind === 'turn' || kind === 'worker' || kind === 'worker_profile' || kind === 'roster' || kind === 'template' || kind === 'prompt') {
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

// Member is one member's row: the check and its name (the only pressable line),
// then the state as a word and a sentence, and what changed. A member hergy
// cannot apply has no check: a dash stands where it would be, and the reason
// comes as a sentence (d-7b45).
function Member({ m, group }: { m: ApplyMember; group: ApplyTeam }): JSX.Element {
  const can = !!m.can_apply;
  const st = STATE[m.state] || { word: m.state, why: '' };
  const how = HOW[m.how] || '';
  const pick = (
    <span class="apl-pick"><code>{m.name}</code>
      {can ? <span class={'apl-how ' + m.how}>{how}{COST[m.how] ? <span class={'apl-cost' + (m.how === 'restart' ? ' lost' : '')}>{COST[m.how]}</span> : null}</span>
        : <span class="apl-how cannot">反映できない</span>}
    </span>
  );
  return <div class={'apl-m' + (can ? '' : ' no')}>
    {can
      ? <label class="apl-mm"><input type="checkbox" data-apl-name={m.name} checked={chosen.value.has(m.name)} onChange={() => toggle(m.name)} />{pick}</label>
      : <div class="apl-mm no"><span class="apl-nopick" aria-hidden="true">―</span>{pick}</div>}
    <p class={'apl-state ' + m.state}><span class="apl-st">{st.word}</span>{st.why}</p>
    {m.how === 'cannot' && m.why_not ? <p class="apl-why">{m.why_not}</p> : null}
    <span class="apl-what">{(m.changes || []).map((c) => {
      const href = linkFor(c, group);
      const text = c.text || '';
      return <span>{href ? <a class="apl-chg" href={href}>{text}</a> : text}</span>;
    })}</span>
  </div>;
}

// ---- 欄の置き場 ----------------------------------------------------------
// Layer is everything the part puts on the page that is not the band's button:
// the sheet while it stands.
export function Layer(): JSX.Element {
  return <>{opened.value && answer.value ? <Sheet /> : null}</>;
}
