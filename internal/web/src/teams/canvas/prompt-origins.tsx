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
// A SectionSpan is where one topology section stands in the current text: the
// lines its own segment covers, with the heading above them when the template
// holds that heading (r-04fc). The heading keeps no origin of its own: the span
// is the drawing, not a mark.
export type SectionSpan = {kind: string; start: number; end: number};

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
// sectionSpans finds the four topology sections in the lines, one span each.
// The span starts at the section's first marked line; blank lines above it are
// skipped, and when the first line after them is a heading with no mark, the
// span starts there (d-57c6). A template that puts a blank line between its
// heading and the section therefore keeps the heading inside the span.
export function sectionSpans(lines: OriginLine[]): SectionSpan[] {
 const spans: SectionSpan[] = [];
 for (const [kind] of topologySections) {
  const marked = lines.flatMap((line, i) => (line.segments.some(segment => segment.kind === kind) ? [i] : []));
  if (!marked.length) continue;
  let start = marked[0];
  let above = start - 1;
  while (above >= 0 && !lines[above].text.trim()) above -= 1;
  if (above >= 0 && !lines[above].segments.length && /^#{1,6} /.test(lines[above].text)) start = above;
  spans.push({kind, start, end: marked[marked.length - 1] + 1});
 }
 return spans;
}
export const sectionAt = <T extends SectionSpan>(spans: T[], index: number): T | undefined => spans.find(span => index >= span.start && index < span.end);
export function OriginMarks({segments}: {segments: Segment[]}) {
 return <span class="prompt-origins">{segments.map(segment => {
  const group = segment.kind === 'role_doc' || segment.kind === 'team_rules' ? '規則' : 'トポロジー';
  const place = segment.origin.place === 'global' ? 'グローバル' : segment.origin.place === 'project' ? 'プロジェクト' : '';
  const description = [group, labels[segment.kind], segment.origin.name, place, segment.origin.path].filter(Boolean).join(' · ');
  return <span class="prompt-origin" data-kind={segment.kind} title={description} aria-label={description}>{labels[segment.kind]}</span>;
 })}</span>;
}
export function OriginLegend({diff}: {diff: boolean}) {
 return <p class="prompt-origin-legend">トポロジー由来: 名簿・連絡・ターン・ワーカー ／ 規則: ロールの規則・チームの規則。{diff && '印は今の文の由来。削除行の由来は記録にありません。'}トポロジーの 4 節は、見出しから節の終わりまでを帯で示します。見出しがひな形の文字でも、印は付けません。</p>;
}
export function CurrentPrompt({text, lines}: {text: string; lines: OriginLine[] | null}) {
 if (!lines) return <><p role="alert">本文と由来が一致しないため、印を表示できません。再取得してください。</p><pre class="prompt-full">{text}</pre></>;
 const spans = sectionSpans(lines);
 return <div class="prompt-full">{lines.map((line, index) => {
  const span = sectionAt(spans, index);
  return <div class="prompt-current-line" data-line={index + 1} data-section={span?.kind} data-section-start={span && span.start === index ? '' : undefined} data-section-end={span && span.end - 1 === index ? '' : undefined} key={index}>
   <OriginMarks segments={line.segments}/><pre class="prompt-line-text">{line.text}</pre>
  </div>;
 })}{text && !text.endsWith('\n') && <p class="prompt-eof">末尾に改行はありません</p>}</div>;
}
