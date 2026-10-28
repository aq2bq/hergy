// roles/yaml.js — YAML のタブ（spec-boundary 7）。
//
// 左にファイルの並び、右に選んだ 1 つのファイルの面。面そのものは共通部品
// （lib/yamltab.js）が描く。この画面が持つのは、どのファイルをどのテキストで
// 読んだか、そのファイルのチェックの答え（並びの ● に使う）、そして保存の答えを
// 行へ戻すこと（state.js）。面の中の押し込みは部品のもので、画面は props で
// 読むものと知らせる先を渡す（依頼書 4）。
import { useEffect } from '../lib/preact.js';
import { YamlTab } from '../lib/yamltab.js';
import { fromJSON, Badge, rulesOf } from '../lib/ruledoc.js';
import {
  PLACES, LNAME, DIR, state, filesList, live, issuesOf,
  ychecksOf, setYchecks, docText, pathOf, clone, pressYamlRevert, pressTab, pressYfile, setYfile,
} from './state.js';
import type { Place, RoleRow, SavedFileEntry } from './state.js';
import type { EditableDocument, RuleIssue } from '../lib/ruledoc.js';
import type { YamlTabProps } from '../lib/yamltab.js';
import type { components } from '../../../apicontract/api-types';

const keyOfYed = (r: RoleRow, l: Place): string => r.uid + l;

// YamlFile is one file's section: the section is the screen's (it carries the
// key the presses read and the hash the save carries); the part draws what
// stands inside it, and reads the file and the hash as props.
function YamlFile({ file }: { file: SavedFileEntry }) {
  const { r, l, e } = file;
  const hid = l === 'global' && live(r, 'project');
  const both = live(r, 'project') && live(r, 'global');
  const name = e.saved.name;
  return <section class="yed" data-key={keyOfYed(r, l)} data-hash={e.hash || ''}>
    <YamlTab key={keyOfYed(r, l)} label={LNAME[l]} path={pathOf(l, e.saved)}
      badge={both ? { cls: 'state ' + (hid ? 'off' : 'on'), text: hid ? '隠れている' : '効いている' } : null}
      text={docText(r, l)} hash={e.hash || ''} kind="role" place={l} name={name}
      problems={ychecksOf(l, name) || []}
      onProblems={(probs: components['schemas']['Problem'][]) => setYchecks(l, name, probs)}
      onRevert={() => pressYamlRevert(r, l)}
      onState={({ text }: { text: string }) => { e.ytext = text; }}
      onSaved={(answer: Parameters<NonNullable<YamlTabProps['onSaved']>>[0]) => {
        const json = answer.json;
        if (!json) return;
        e.hash = json.hash || e.hash;
        if (typeof json.text === 'string') e.text = json.text;
        if (json.doc && 'sections' in json.doc) { e.saved = fromJSON(json.doc); e.draft = clone(e.saved); }
        e.ytext = undefined;
      }} />
  </section>;
}

// ychecksText marks a file whose text the check found something wrong with.
function ychecksText(place: Place, name: string) {
  const y = ychecksOf(place, name);
  return y && y.some((m) => m.sev === 'err') ? <span class="dirty">●</span> : '';
}

export function YamlPage() {
  const files = filesList();
  // With no role document at all, the tab says what is missing and the one thing
  // that makes the documents (n-f2ef, the same shape as the team definition's
  // YAML tab).
  const picked = state.value.yfile;
  const chosen = picked && files.some(({ r, l }) => keyOfYed(r, l) === picked)
    ? picked
    : (files[0] ? keyOfYed(files[0].r, files[0].l) : null);
  // The address names no file (or one that is not here): the first file stands,
  // and the screen keeps it for the address its presses write.
  useEffect(() => { if (chosen && picked !== chosen) setYfile(chosen); }, [chosen]);
  if (!files.length) {
    return <section class="yed yempty">
      <p>ロールの文書はまだありません。</p>
      <p>一覧の『＋ ロールを追加』で作ると、ここに roles/&lt;ロール名&gt;.yaml が並びます。</p>
      <span><button class="btn" data-act="tab" data-t="roles" onClick={() => pressTab('roles')}>一覧へ</button></span></section>;
  }
  const one = files.find(({ r, l }) => keyOfYed(r, l) === chosen) || null;
  return <div class="ylay">
      <nav class="files">{PLACES.map((l) => <div class="fgroup"><div class="gh"><b>{LNAME[l]}</b><code>{DIR[l]}</code></div>{files.filter((f) => f.l === l).map(({ r, e }) => {
        const key = keyOfYed(r, l), is = issuesOf(r, l), hid = l === 'global' && live(r, 'project');
        return <button class={['file', chosen === key ? 'on' : '', hid ? 'hid' : ''].filter(Boolean).join(' ')} data-act="yfile" data-key={key} aria-pressed={chosen === key} onClick={() => pressYfile(key)}><code>{e.saved.name}.yaml</code><span class="fs">{ychecksText(l, e.saved.name)}{badgeOrCount(is, e.saved)}</span><span class="ft">{e.saved.title}{hid ? '・隠れている' : ''}</span></button>;
      })}</div>)}</nav>
      {one ? <YamlFile key={chosen} file={one} /> : <div class="none">ロールの文書はありません</div>}
    </div>;
}

// badgeOrCount is what the file's line carries after its name: the problems the
// file's own check found, or, with none, how many rules it holds.
function badgeOrCount(issues: RuleIssue[], saved: EditableDocument | null) {
  if ((issues || []).length) return <Badge list={issues} />;
  const n = saved ? rulesOf(saved).length : 0;
  return n ? <>{n}</> : '';
}
