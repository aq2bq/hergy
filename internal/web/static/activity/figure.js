// activity/figure.js — チームの図（spec 6 章）。
//
// これが WebUI で唯一、文字列のマークアップを流し込む所（webui-preact-design
// 5 章 1 の例外。理由と、いつ消えるかは internal/web/markup_test.go の
// figureException に書いてある）。
//
// 図は hq.js の teamSVG が返す。hq.js はまだ classic script で、この画面と
// チーム定義の画面が同じ 1 つの図を描く。図を Preact の部品にするのは hq.js を
// module にする段で、そのときこの dangerouslySetInnerHTML と markup_test.go の
// 例外を一緒に消す。
//
// 流し込んでよいのは、図が「操作しない図」だから: teamSVG が出すのは
// svg(role="img") / g / circle / text / path / line だけで、押せるものも、
// フォーカスの入るものも無い。
import { html } from '../vendor/preact.js';

const A = window.hergyHQ;

export function Figure({ team }) {
  const svg = A ? A.teamSVG(team, team.turns) : '';
  return html`<div class="hq-fig" dangerouslySetInnerHTML=${{ __html: svg }}></div>`;
}
