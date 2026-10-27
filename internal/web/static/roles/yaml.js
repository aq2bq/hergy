// roles/yaml.js — YAML のタブ（spec-boundary 7）。
//
// 左にファイルの並び、右に選んだ 1 つのファイルの面。面そのものは共通部品
// （lib/yamltab.js）が描く。この画面が持つのは、どのファイルをどのテキストで
// 読んだか、そのファイルのチェックの答え（並びの ● に使う）、そして保存の答えを
// 行へ戻すこと（state.js）。
import { html, useEffect, useRef, useState } from '../vendor/preact.js';
import { YamlTab } from '../lib/yamltab.js';
import { fromJSON, Badge, rulesOf } from '../lib/ruledoc.js';
import {
  PLACES, LNAME, DIR, state, filesList, live, issuesOf,
  ychecksOf, setYchecks, docText, pathOf, clone,
} from './state.js';

const keyOfYed = (r, l) => r.uid + l;

// YamlFile is one file's section. The section element is the screen's (it carries
// the key the presses read and the hash the save carries); the part draws what
// stands inside it. The part takes the section so the screen can hand it a
// problem's press, a revert or a save through the screen's own call; the element is known
// after the first draw, so the file draws itself once more with it.
function YamlFile({ r, l }) {
  const section = useRef(null);
  const [drawn, setDrawn] = useState(false);
  const e = r[l];
  const hid = l === 'global' && live(r, 'project');
  const both = live(r, 'project') && live(r, 'global');
  const name = e.saved.name;
  useEffect(() => { if (!drawn) setDrawn(true); }, [drawn]);
  return html`<section class="yed" data-key=${keyOfYed(r, l)} data-hash=${e.hash || ''} ref=${section}>
    <${YamlTab} key=${keyOfYed(r, l)} section=${section.current} label=${LNAME[l]} path=${pathOf(l, e.saved)}
      badge=${both ? { cls: 'state ' + (hid ? 'off' : 'on'), text: hid ? '隠れている' : '効いている' } : null}
      text=${docText(r, l)} kind="role" place=${l} name=${name}
      problems=${ychecksOf(l, name) || []}
      onProblems=${(sec, probs) => setYchecks(l, name, probs)}
      onState=${({ text }) => { e.ytext = text; }}
      onSaved=${(sec, answer) => {
        const json = (answer && answer.json) || {};
        e.hash = json.hash || e.hash;
        if (typeof json.text === 'string') e.text = json.text;
        if (json.doc) { e.saved = fromJSON(json.doc); e.draft = clone(e.saved); }
        e.ytext = undefined;
        if (sec) sec.dataset.hash = e.hash;
      }} />
  </section>`;
}

// ychecksText marks a file whose text the check found something wrong with.
function ychecksText(place, name) {
  const y = ychecksOf(place, name);
  return y && y.some((m) => m.sev === 'err') ? html`<span class="dirty">●</span>` : '';
}

export function YamlPage() {
  const files = filesList();
  // With no role document at all, the tab says what is missing and the one thing
  // that makes the documents (n-f2ef, the same shape as the team definition's
  // YAML tab).
  if (!files.length) {
    return html`<section class="yed yempty">
      <p>ロールの文書はまだありません。</p>
      <p>一覧の『＋ ロールを追加』で作ると、ここに roles/&lt;ロール名&gt;.yaml が並びます。</p>
      <span><button class="btn" data-act="tab" data-t="roles">一覧へ</button></span></section>`;
  }
  const chosen = state.yfile && files.some(({ r, l }) => keyOfYed(r, l) === state.yfile)
    ? state.yfile
    : (files[0] ? keyOfYed(files[0].r, files[0].l) : null);
  if (state.yfile !== chosen) state.yfile = chosen;
  const one = files.find(({ r, l }) => keyOfYed(r, l) === chosen) || null;
  return html`<div class="ylay">
      <nav class="files">${PLACES.map((l) => html`<div class="fgroup"><div class="gh"><b>${LNAME[l]}</b><code>${DIR[l]}</code></div>${files.filter((f) => f.l === l).map(({ r, e }) => {
        const key = keyOfYed(r, l), is = issuesOf(r, l), hid = l === 'global' && live(r, 'project');
        return html`<button class=${['file', chosen === key ? 'on' : '', hid ? 'hid' : ''].filter(Boolean).join(' ')} data-act="yfile" data-key=${key} aria-pressed=${chosen === key}><code>${e.saved.name}.yaml</code><span class="fs">${ychecksText(l, e.saved.name)}${badgeOrCount(is, e.saved)}</span><span class="ft">${e.saved.title}${hid ? '・隠れている' : ''}</span></button>`;
      })}</div>`)}</nav>
      ${one ? html`<${YamlFile} key=${chosen} r=${one.r} l=${one.l} />` : html`<div class="none">ロールの文書はありません</div>`}
    </div>`;
}

// badgeOrCount is what the file's line carries after its name: the problems the
// file's own check found, or, with none, how many rules it holds.
function badgeOrCount(issues, saved) {
  if ((issues || []).length) return html`<${Badge} list=${issues} />`;
  const n = saved ? rulesOf(saved).length : 0;
  return n ? html`${n}` : '';
}
