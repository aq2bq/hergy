// roles/assign.js — 割り当てのタブ（spec 4 章、spec-roles 2「割り当てのタブ」）。
//
// チーム定義ごとのカードと、メンバーのロールの選択を描く。押し込みは main.js が
// state.js へ渡す（webui-preact-design 5 章 1）。選んだロールは、そのチームの
// 保存が書くまで画面の下書き（state.js）。
import { html } from '../vendor/preact.js';
import {
  PLACES, LNAME, DIR, TEXT, state, teams, savedRoles, cur, curName, eff,
  roleByName, okOf, teamErrorOf, fill,
} from './state.js';
import { Vm } from '../lib/ruledoc.js';

function roleOptions(curRole) {
  const rs = savedRoles();
  const missing = !rs.some((r) => curName(r) === curRole);
  return html`${missing ? html`<option selected>${curRole}</option>` : ''}${PLACES.map((l) => {
    const mine = rs.filter((r) => eff(r) === l);
    return mine.length ? html`<optgroup label=${LNAME[l]}>${mine.map((r) => html`<option value=${curName(r)} selected=${curName(r) === curRole}>${`${curName(r)}　${cur(r).title}`}</option>`)}</optgroup>` : '';
  })}`;
}

function ARow({ t, m, i }) {
  const r = roleByName(m.draft);
  const dim = state.roleHi && m.draft !== state.roleHi;
  // The sentence the server would print for validate, with its values: the
  // member's own name, and where hergy looked, as a person reads a path.
  const places = [DIR.project + m.draft + '.yaml', DIR.global + m.draft + '.yaml'].join(TEXT.separator || '、');
  return html`<div class=${['arow', m.draft !== m.saved ? 'changed' : '', r ? '' : 'missing', dim ? 'dim' : ''].filter(Boolean).join(' ')} data-m=${m.name}>
      <div class="who"><code>${m.name}</code></div>
      <div class="pick">${m.draft !== m.saved ? html`<s class="was">${m.saved}</s>` : ''}<span class="sel"><select class="pickrole" id=${`s-${t.id}-${i}`} data-t=${t.id} data-i=${i} aria-label=${`${m.name} のロール`}>${roleOptions(m.draft)}</select></span></div>
      ${r ? '' : html`<ul class="vm"><${Vm} list=${[{ sev: 'err', msg: fill(TEXT.no_role_doc, m.name, m.draft, places) }]} /></ul>`}
    </div>`;
}

function Team({ t }) {
  const dirty = t.members.filter((m) => m.draft !== m.saved).length;
  const errs = t.members.filter((m) => !roleByName(m.draft)).length;
  const refusal = teamErrorOf(t.id);
  return html`<section class="team" data-t=${t.id}>
      <header class="th"><span class="tl">チーム定義</span><code class="tid">${t.id}</code><span class="pill on">${LNAME[t.place] || LNAME.project}</span>
        <span class="tside">${errs ? html`<span class="sev err">問題 ${errs}</span>` : ''}${dirty ? html`<span class="dirty">${dirty} 人</span><button class="btn primary" data-act="t-save" disabled=${errs > 0}>保存</button>` : okOf('t' + t.id) ? html`<span class="ok">保存しました</span>` : ''}</span>
        ${refusal ? html`<span class="errline">${refusal}</span>` : ''}
      </header>
      ${t.members.map((m, i) => html`<${ARow} key=${m.name} t=${t} m=${m} i=${i} />`)}
    </section>`;
}

// RoleBar is the top of the tab: one chip per role the teams name, with how many
// people hold it. A chip a person picks dims the others (P6).
function RoleBar() {
  const cnt = {};
  teams.forEach((t) => t.members.forEach((m) => { cnt[m.draft] = (cnt[m.draft] || 0) + 1; }));
  return html`<div class="rolebar">${savedRoles().filter((r) => cnt[curName(r)]).map((r) => {
    const n = curName(r);
    return html`<button class=${'rseg' + (state.roleHi === n ? ' on' : state.roleHi ? ' dim' : '')} data-act="role-hi" data-n=${n} aria-pressed=${state.roleHi === n}><code>${n}</code><span>${cur(r).title}</span><b>${cnt[n]}</b></button>`;
  })}</div>`;
}

export function AssignPage() {
  if (!teams.length) return html`<p class="none">チーム定義はありません</p>`;
  return html`<${RoleBar} /><div class="teams">${teams.map((t) => html`<${Team} key=${t.id} t=${t} />`)}</div>`;
}
