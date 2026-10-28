// profiles/yaml.tsx — YAML のタブ（spec-boundary 7）。
//
// 面は共通部品（lib/yamltab.js）が描く。この画面が持つのは、どのファイルを
// どのテキストで読んだかだけで、帯に渡す下書きもそこから作る（spec-nav 1.5）。
// 面の中の押し込み（問題の行・元に戻す・保存）と Tab は部品のもので、画面は
// props で読むものと知らせる先を渡す（依頼書 4）。
import { yamlFiles, yamlState, tellYaml, tellDirty } from './state.js';
import { YamlTab } from '../lib/yamltab.js';
import type { YamlFile, YamlOne } from './types.js';

// YamlFileSection is one file's section: the section is the screen's (it
// carries the file's name and hash), the part draws what stands inside it.
function YamlFileSection({ file }: { file: YamlFile }) {
  const one: YamlOne = yamlState.value.get(file.file) || {};
  return (
    <section class="yed" data-file={file.file} data-hash={file.hash}>
      <YamlTab key={file.file} file={file.file} label={file.label} path={file.path}
        text={file.text} hash={file.hash} kind="profile" place={file.file} name="_yaml" problems={file.problems || []}
        carried={one.carried === undefined ? null : one.carried}
        onState={(state: YamlOne) => { tellYaml(file.file, { ...(yamlState.value.get(file.file) || {}), ...state, carried: null }); tellDirty(); }} />
    </section>
  );
}

export function YamlPage() {
  return (
    <div class="yeds">
      {yamlFiles.value.map((file) => <YamlFileSection key={file.file} file={file} />)}
    </div>
  );
}
