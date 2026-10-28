// profiles/yaml.js — YAML のタブ（spec-boundary 7）。
//
// 面は共通部品（lib/yamltab.js）が描く。この画面が持つのは、どのファイルを
// どのテキストで読んだかだけで、帯に渡す下書きもそこから作る（spec-nav 1.5）。
//
// 面の中の押し込み（問題の行・元に戻す・保存）と Tab、行番号の欄の追従は、この
// 画面の section が受ける（section はこの画面が描く要素なので、その中で起きた
// ことはここで答える）。部品そのものが押し込みを持つのは、3 つの画面が同じ形に
// なってから（webui-preact-design 依頼書 2）。
import { html, useEffect, useRef, useState } from '../vendor/preact.js';
import { yamlFiles, yamlState, tellYaml, tellDirty } from './state.js';
import { YamlTab, jump, revert, save, insertTab } from '../lib/yamltab.js';

// press answers one press inside the section: a problem's line, the revert or the
// save. The part does the work; the section names which part it is.
function press(section, event) {
  const b = event.target.closest('[data-act]');
  if (!b || !section.contains(b)) return;
  const act = b.dataset.act;
  if (act === 'yaml-jump') jump(section, Number(b.dataset.line));
  else if (act === 'yaml-revert') revert(section);
  else if (act === 'yaml-save') save(section);
}

// YamlFile is one file's section: the section is the screen's (it carries the
// file's name and hash), the part draws what stands inside it. The part takes
// the section so the screen can hand it a problem's press, a revert or a save
// through it; the section element is known after the first draw, so the file
// draws itself once more with it.
function YamlFile({ file }) {
  const section = useRef(null);
  const [drawn, setDrawn] = useState(false);
  const one = yamlState.value.get(file.file) || {};
  useEffect(() => { if (!drawn) setDrawn(true); }, [drawn]);

  // The numbered gutter follows the text's scroll, as it did when the screen
  // listened for it (a scroll does not bubble, so the section is caught on the
  // way down).
  useEffect(() => {
    const el = section.current;
    if (!el) return undefined;
    const follow = (event) => {
      if (!event.target.classList || !event.target.classList.contains('ytext')) return;
      const gutter = el.querySelector('.gutter');
      if (gutter) gutter.scrollTop = event.target.scrollTop;
    };
    el.addEventListener('scroll', follow, true);
    return () => el.removeEventListener('scroll', follow, true);
  }, [drawn]);

  return html`<section class="yed" data-file=${file.file} data-hash=${file.hash} ref=${section}
    onClick=${(e) => press(section.current, e)}
    onKeyDown=${(e) => { if (e.key === 'Tab' && e.target.matches('.ytext') && !e.shiftKey) { e.preventDefault(); insertTab(e.target); } }}>
    <${YamlTab} key=${file.file} section=${section.current} file=${file.file} label=${file.label} path=${file.path}
      text=${file.text} hash=${file.hash} kind="profile" place=${file.file} name="_yaml" problems=${file.problems || []}
      carried=${one.carried === undefined ? null : one.carried}
      onState=${(state) => { tellYaml(file.file, { ...(yamlState.value.get(file.file) || {}), ...state, carried: null }); tellDirty(); }} />
  </section>`;
}

export function YamlPage() {
  return html`<div class="yeds">${yamlFiles.value.map((file) => html`<${YamlFile} key=${file.file} file=${file} />`)}</div>`;
}
