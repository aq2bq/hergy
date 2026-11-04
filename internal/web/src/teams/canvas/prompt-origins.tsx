import type { Schemas } from '../../lib/api.js';

type Segment = Schemas['TeamPromptSegment'];
export const topologySections = [
 ['roster', '名簿'], ['herdr', '連絡'], ['turns', 'ターンを渡す相手'], ['workers', 'ワーカーの追加'],
] as const;
const labels: Record<string, string> = {
 roster: '名簿', herdr: '連絡', turns: 'ターン', workers: 'ワーカー',
 role_doc: 'ロールの規則', team_rules: 'チームの規則',
};
export type OriginLine = {text: string; segments: Segment[]};

// Ranges belong to the actual generated text, not Markdown headings or the
// attribute panel's potentially different execution context. LF ends its line.
export function originLines(text: string, segments: Segment[]): OriginLine[] | null {
 if (segments.map(segment => segment.text).join('') !== text) return null;
 const lines: OriginLine[] = (text.match(/[^\n]*\n|[^\n]+$/g) || []).map(text => ({text, segments: []}));
 let offset = 0, index = 0, start = 0;
 for (const segment of segments) {
  const end = offset + segment.text.length;
  while (index < lines.length && start + lines[index].text.length <= offset) start += lines[index++].text.length;
  let at = start;
  for (let i = index; i < lines.length && at < end; i++) {
   if (labels[segment.kind] && end > offset && !lines[i].segments.some(one => one.kind === segment.kind && one.origin.name === segment.origin.name && one.origin.path === segment.origin.path && one.origin.place === segment.origin.place)) lines[i].segments.push(segment);
   at += lines[i].text.length;
  }
  offset = end;
 }
 return lines;
}
export function OriginMarks({segments}: {segments: Segment[]}) {
 return <span class="prompt-origins">{segments.map(segment => {
  const group = segment.kind === 'role_doc' || segment.kind === 'team_rules' ? '規則' : 'トポロジー';
  const place = segment.origin.place === 'global' ? 'グローバル' : segment.origin.place === 'project' ? 'プロジェクト' : '';
  const description = [group, labels[segment.kind], segment.origin.name, place, segment.origin.path].filter(Boolean).join(' · ');
  return <span class="prompt-origin" data-kind={segment.kind} title={description} aria-label={description}>{labels[segment.kind]}</span>;
 })}</span>;
}
export function OriginLegend({diff}: {diff: boolean}) {
 return <p class="prompt-origin-legend">トポロジー由来: 名簿・連絡・ターン・ワーカー ／ 規則: ロールの規則・チームの規則。{diff && '印は今の文の由来。削除行の由来は記録にありません。'}</p>;
}
export function CurrentPrompt({text, lines}: {text: string; lines: OriginLine[] | null}) {
 if (!lines) return <><p role="alert">本文と由来が一致しないため、印を表示できません。再取得してください。</p><pre class="prompt-full">{text}</pre></>;
 return <div class="prompt-full">{lines.map((line, index) => <div class="prompt-current-line" data-line={index + 1} key={index}>
  <OriginMarks segments={line.segments}/><pre class="prompt-line-text">{line.text}</pre>
 </div>)}{text && !text.endsWith('\n') && <p class="prompt-eof">末尾に改行はありません</p>}</div>;
}
