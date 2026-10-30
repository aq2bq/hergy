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
import { dkey, recs, rolesIn, setWide } from './state.js';
import type { Rec } from './state.js';
import { afterTPrev, fitYbox, TPrev } from './canvas/yaml.js';
import { Figure, frameOf } from './canvas/figure.js';
import { Menu } from './canvas/menu.js';
import { SideAttr } from './canvas/attr.js';
import { onClick, onInput, onPointerDown } from './canvas/act.js';
import { selOf, sideOf } from './canvas/shared.js';

export { fitYboxes } from './canvas/yaml.js';
export { mountCanvas } from './canvas/act.js';

// MIN_SCALE is the smallest the figure is drawn: a member's name (14px) stays
// above 11px, so no one has to zoom the page to read a team (d-943f).
export const MIN_SCALE = 0.8;

// fitFigure gives one row's figure the room it has (r-e39c): its own units at
// most (a two-member team is no longer stretched to the column's width), and
// smaller only down to MIN_SCALE. A ring that would go under that is laid out
// in rows instead, and the next draw measures that shape.
function fitFigure(main: HTMLElement, r: Rec, f: { vw: number; vh: number }): void {
  const cv = main.querySelector<HTMLElement>('.cv-canvas');
  const svg = cv ? cv.querySelector<SVGSVGElement>('svg') : null;
  if (!cv || !svg || !f.vw || !f.vh) return;
  const panel = main.closest<HTMLElement>('.panel.team-p');
  const row = panel ? panel.closest<HTMLElement>('.tdef') : null;
  const head = row ? row.querySelector<HTMLElement>(':scope > .sum') : null;
  const foot = panel ? panel.querySelector<HTMLElement>('footer.pf') : null;
  const vlist = main.querySelector<HTMLElement>('.vlist');
  // The room is read where the row's head stands once it is opened: under the
  // band, with the check column and the save's band below the figure. Reading
  // it from the page as it stands would make the figure smaller the further
  // down the list the row is.
  const stick = Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--stick')) || 62;
  const fromPanel = panel ? cv.getBoundingClientRect().top - panel.getBoundingClientRect().top : cv.getBoundingClientRect().top;
  const top = stick + (head ? head.offsetHeight : 0) + fromPanel;
  const below = (vlist ? vlist.offsetHeight : 0) + (foot ? foot.offsetHeight : 0) + 12;
  const room = Math.max(160, Math.floor(window.innerHeight - top - below));
  const across = Math.max(160, cv.clientWidth - 2);
  let scale = Math.min(1, across / f.vw, room / f.vh);
  if (scale < MIN_SCALE && !r.wide.value) { setWide(r); return; }
  scale = Math.max(MIN_SCALE, scale);
  const w = `${Math.round(f.vw * scale)}px`, h = `${Math.round(f.vh * scale)}px`;
  if (svg.style.width !== w || svg.style.height !== h) { svg.style.width = w; svg.style.height = h; }
}

// fitFigures measures every open row's figure again, after the window moved.
export function fitFigures(): void {
  Array.from(document.querySelectorAll<HTMLElement>('.panel.team-p')).forEach((panel) => {
    const main = panel.querySelector<HTMLElement>('.cv-main'), svg = panel.querySelector<SVGSVGElement>('.cv-canvas svg');
    if (!main || !svg) return;
    const r = recs.value.find((x) => dkey(x) === panel.dataset.key);
    if (!r) return;
    fitFigure(main, r, { vw: Number(svg.dataset.vw) || 0, vh: Number(svg.dataset.vh) || 0 });
  });
}

// ---- the canvas itself ------------------------------------------------
export function Canvas({ r, vlist }: { r: Rec; vlist: JSX.Element }): JSX.Element {
  const box = useRef<HTMLDivElement>(null);
  // frame is read while the part draws, so the room is measured again whenever
  // the points move or the shape changes (r-e39c).
  const frame = frameOf(r);
  useLayoutEffect(() => {
    const el = box.current;
    if (el) fitYbox(el);
    afterTPrev(r);
    if (el) fitFigure(el, r, frame);
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
