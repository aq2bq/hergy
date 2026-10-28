// teams/canvas.tsx — トポロジー（team.yaml を点と線で見て直す）のカンバス
// （n-6be7、設計者のモック 2026-10-23。webui-preact-design 4 章）。
//
// 画面が図と属性の欄を描き、変える所はすべて画面の ops として state へ渡す。
// サーバーがそれを検査し、team.yaml としてそのまま書く（spec-teams B7、
// spec-boundary 5）。この部品が持つのは定義に無いものだけ: どの点・線を選んだか、
// 右の欄が 属性 か YAML か、点をどこへ引いたか。点の位置は保存しない（開き直すと
// 引き直す、d-a575）。
//
// 押し込みは部品が自分の器で受ける（5 章 1）。図は canvas/figure.tsx、右の列は
// canvas/attr.tsx、右の YAML は canvas/yaml.tsx、余白のメニューは
// canvas/menu.tsx、押し込みとキーとドラッグは canvas/act.ts が持つ。
import { useLayoutEffect, useRef } from '../lib/preact.js';
import type { JSX } from 'preact';
import { rolesIn } from './state.js';
import type { Rec } from './state.js';
import { afterTPrev, fitYbox, TPrev } from './canvas/yaml.js';
import { Figure } from './canvas/figure.js';
import { Menu } from './canvas/menu.js';
import { SideAttr } from './canvas/attr.js';
import { onClick, onInput, onPointerDown } from './canvas/act.js';
import { selOf, sideOf } from './canvas/shared.js';

export { fitYboxes } from './canvas/yaml.js';
export { mountCanvas } from './canvas/act.js';

// ---- the canvas itself ------------------------------------------------
export function Canvas({ r, vlist }: { r: Rec; vlist: JSX.Element }): JSX.Element {
  const box = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    const el = box.current;
    if (el) fitYbox(el);
    afterTPrev(r);
  });
  const sel = selOf(r);
  const cur = sel, side = sideOf(r);
  const picked = cur && cur.type === 'node' && cur.id !== 'human' ? cur.id : '';
  const hint = picked ? `${picked} を選んでいる。縁の丸から引くとターン、Delete で外す` : '点や線を押すと属性が出る。余白をダブルクリックすると、そこに足せる';
  return (
    <div class="cv-main" ref={box} onPointerDown={(e: PointerEvent) => onPointerDown(r, e)} onClick={(e: MouseEvent) => onClick(r, e)} onInput={(e: Event) => onInput(r, e)} onChange={(e: Event) => onInput(r, e)}>
      <div class="cv-left">
        <div class="cv-tools">
          <button class="btn sm" type="button" data-cv-act="m-add" disabled={!rolesIn()}>＋ メンバー</button>
          <button class="btn sm" type="button" data-cv-act="w-add">＋ ワーカー</button>
          <span class="cv-hint">{hint}</span>
        </div>
        <div class="cv-canvas"><Figure r={r} /><Menu r={r} /></div>
        <div class="vlist">{vlist}</div>
      </div>
      <aside class="cv-side" aria-label="選んだものの属性">
        <div class="cv-sh">
          <button type="button" class={side === 'attr' ? 'on' : ''} data-cv-side="attr">属性</button>
          <button type="button" class={side === 'yaml' ? 'on' : ''} data-cv-side="yaml">YAML</button>
        </div>
        <div class="cv-sb">{side === 'attr' ? <SideAttr r={r} /> : <TPrev r={r} />}</div>
      </aside>
    </div>
  );
}
