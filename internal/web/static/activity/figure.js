// activity/figure.js — チームの図（spec 6 章）。
//
// 図は共通部品 lib/hq/figure.js が描く（活動の画面のタイルと詳細、そして将来は
// チーム定義の画面も同じ 1 つの図）。前は hq.js が SVG の文字列を返し、ここが
// 唯一 dangerouslySetInnerHTML で流し込んでいた。hq が module になったので、
// その例外（markup_test.go）も一緒に消えた（webui-preact-design 5 章 1）。
import { html } from '../vendor/preact.js';
import { TeamFigure } from '../lib/hq/figure.js';

export function Figure({ team }) {
  return html`<div class="hq-fig"><${TeamFigure} team=${team} turns=${team.turns} /></div>`;
}
