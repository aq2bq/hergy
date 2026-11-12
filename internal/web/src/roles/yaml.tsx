import { HQ } from './state';
import { ProjectView } from '../lib/project-view';
// The file list and selection are the screen's. The text, validation and
// saved basis are read directly from the role's text editor.
import { useEffect } from '../lib/preact.js';
import { YamlTab } from '../lib/yamltab.js';
import { Badge } from '../lib/ruledoc.js';
import {
  PLACES, LNAME, DIR, state, filesList, issuesOf,
  pathOf, yamlSaved, pressTab, pressYfile, setYfile, tellDirty,
} from './state.js';
import type { Place, RoleRow, SavedFileEntry } from './state.js';
import type { EditableDocument, RuleIssue } from '../lib/ruledoc.js';
import type { YamlTabProps } from '../lib/yamltab.js';

const keyOfYed = (r: RoleRow, l: Place): string => r.uid + l;

// YamlFile is one file's section: the section is the screen's (it carries the
// key the presses read and the hash the save carries); the part draws what
// stands inside it, and reads the file and the hash as props.
function YamlFile({ file }: { file: SavedFileEntry }) {
  const { r, l, e } = file;
  // The owner marks a name both layers hold (r-ea17); the section does not
  // compare the two places itself.
  const hid = r.mark === 'hidden';
  const both = !!r.mark;
  const name = e.saved.name;
  return <section class="yed" data-key={keyOfYed(r, l)} data-hash={e.hash || ''}>
    <YamlTab key={keyOfYed(r, l)} label={LNAME[l]} path={pathOf(l, e.saved)}
      badge={both ? { cls: 'state ' + (hid ? 'off' : 'on'), text: hid ? '無効' : '優先' } : null}
      editing={e.editing.yaml} text={e.editing.yaml.read.value.saved || ''} hash={e.hash || ''} kind="role" place={l} name={name}
      problems={e.editing.yaml.read.value.problems.map((p) => ({ ...p }))} problem={e.problem || ''}
      onState={() => tellDirty()}
      onSaved={(answer: Parameters<NonNullable<YamlTabProps['onSaved']>>[0]) => yamlSaved(r, l, answer)} />
  </section>;
}

// ychecksText marks a file whose text the check found something wrong with.
function ychecksText(file: SavedFileEntry) {
  return file.e.editing.yaml.read.value.problems.some((p) => p.sev === 'err') ? <span class="dirty">●</span> : '';

}

function YamlPageBody() {
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
      <p>一覧の『＋ ロールを追加』で作ると、ここに roles/&lt;ロール名&gt;.md が並びます。</p>
      <span><button class="btn" data-act="tab" data-t="roles" onClick={() => pressTab('roles')}>一覧へ</button></span></section>;
  }
  const one = files.find(({ r, l }) => keyOfYed(r, l) === chosen) || null;
  return <div class="ylay">
      <nav class="files">{PLACES.map((l) => <div class="fgroup"><div class="gh"><b>{LNAME[l]}</b><code>{DIR[l]}</code></div>{files.filter((f) => f.l === l).map(({ r, e }) => {
        const key = keyOfYed(r, l), is = issuesOf(r, l), hid = r.mark === 'hidden';
        return <button class={['file', chosen === key ? 'on' : '', hid ? 'hid' : ''].filter(Boolean).join(' ')} data-act="yfile" data-key={key} aria-pressed={chosen === key} onClick={() => pressYfile(key)}><code>{e.saved.name}.md</code><span class="fs">{ychecksText({ r, l, e })}{badgeOrCount(is, e.saved)}</span><span class="ft">{e.saved.title}{hid ? '・無効' : ''}</span></button>;
      })}</div>)}</nav>
      {one ? <YamlFile key={chosen} file={one} /> : <div class="none">ロールの文書はありません</div>}
    </div>;
}

// badgeOrCount is what the file's line carries after its name: the problems the
// file's own check found, or, with none, how many rules it holds.
function badgeOrCount(issues: RuleIssue[], saved: EditableDocument | null) {
  if ((issues || []).length) return <Badge list={issues} />;
  const n = saved ? Array.from(saved.text).length : 0;
  return n ? <>{n}</> : '';
}

export function YamlPage() { return <><ProjectView projects={HQ?.projects} /><YamlPageBody /></>; }
