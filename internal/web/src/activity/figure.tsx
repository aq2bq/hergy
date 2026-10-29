// activity/figure.tsx — チームの図（spec 6 章）。
//
// 図は共有部品 lib/hq/figure.tsx が描く（アクティビティのタイルと
// チーム定義の画面で同じ 1 つの図）。前は hq.js が SVG の文字列を返し、ここが
// 唯一 dangerouslySetInnerHTML で流し込んでいた。hq が module になったので、
// その例外（markup_test.go）も一緒に消えた（webui-preact-design 5 章 1）。
import type { JSX } from 'preact';
import { TeamFigure } from '../lib/hq/figure.js';
import type { ActivityTeam } from './state.js';

export function Figure({ team }: { team: ActivityTeam }): JSX.Element {
  return <div class="hq-fig">
    <TeamFigure team={team} turns={team.turns} W={440} layout="packed" />
  </div>;
}
