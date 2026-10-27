// profiles/yaml.js — YAML のタブ（spec-boundary 7）。
//
// 面は共通部品（lib/yamltab.js）が描く。この画面が持つのは、どのファイルを
// どのテキストで読んだかだけで、帯に渡す下書きもそこから作る（spec-nav 1.5）。
import { html, useEffect, useRef, useState } from '../vendor/preact.js';
import { yamlFiles, yamlState, tellYaml, tellDirty } from './state.js';
import { YamlTab } from '../lib/yamltab.js';

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
  return html`<section class="yed" data-file=${file.file} data-hash=${file.hash} ref=${section}>
    <${YamlTab} key=${file.file} section=${section.current} file=${file.file} label=${file.label} path=${file.path}
      text=${file.text} hash=${file.hash} kind="profile" place=${file.file} name="_yaml" problems=${file.problems || []}
      carried=${one.carried === undefined ? null : one.carried}
      onState=${(state) => { tellYaml(file.file, { ...(yamlState.value.get(file.file) || {}), ...state, carried: null }); tellDirty(); }} />
  </section>`;
}

export function YamlPage() {
  return html`<div class="yeds">${yamlFiles.value.map((file) => html`<${YamlFile} key=${file.file} file=${file} />`)}</div>`;
}
