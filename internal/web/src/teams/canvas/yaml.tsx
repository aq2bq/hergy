// teams/canvas/yaml.tsx — the team.yaml a save would write (this tab's
// preview)（spec-teams B7、spec-boundary 5）。
//
// TPrev is the トポロジー tab's team.yaml: the text a save would write, line by
// line, with the lines one edit changed marked. The lines, what each stands for
// and the marks all come from the server. The part also keeps the box scrolled
// where a person is looking (監査 R1, R2).
import type { JSX } from 'preact';
import { Counts } from '../../lib/ruledoc.js';
import { dkey, pressLine, teamIssues } from '../state.js';
import type { CanvasPick, Finding, Rec, TeamPreview } from '../state.js';
import { pair, selOf } from './shared.js';

// GLOW is the preview's own bookkeeping: the lines drawn last, the lines one
// edit just moved, and when they lit up (spec-teams B7).
const GLOW: {
  last: Record<string, string[]>;
  step: Record<string, number[]>;
  glow: Record<string, number>;
  timer: Record<string, ReturnType<typeof setTimeout>>;
} = { last: {}, step: {}, glow: {}, timer: {} };

export function TPrev({ r }: { r: Rec }): JSX.Element {
  const key = dkey(r);
  const pv: TeamPreview = r.preview.value || { lines: [], diff: { added: null, removedBefore: null, removedAtEnd: false } };
  const at = pv.lines || [], df = pv.diff || {};
  const added = df.added || [], removed = df.removedBefore || [], lastDel = !!df.removedAtEnd;
  const lines = at.map((z) => z.text);
  const prev = GLOW.last[key];
  // The first answer for a row is not a move: the text was not drawn before,
  // so the row's whole file is not marked as changed (監査 R1).
  if (prev && prev.length && prev.join('\n') !== lines.join('\n')) {
    GLOW.step[key] = [...stepOf(prev, lines)];
    GLOW.glow[key] = Date.now();
  }
  GLOW.last[key] = lines;
  const step = new Set(GLOW.step[key] || []);
  const marks = lineMarks(r), cur = selOf(r), n = at.length;
  const nadd = added.length, ndel = removed.length + (lastDel ? 1 : 0);
  const issues = teamIssues(r);
  return (
    <aside class="yprev tprev" aria-label="team.yaml">
      <div class="yph">
        <span class="pkl">YAML</span>
        <span class="yinfo">
          <span class="vt">{n} 行</span>
          {nadd || ndel ? <span class="dirty">+{nadd} −{ndel}</span> : ''}
          <span class="yr">{issues.length ? <span class="vcount"><Counts list={issues.map((m) => ({ sev: m.sev, text: m.text }))} /></span> : <span class="vclean">✓ 問題なし</span>}</span>
        </span>
      </div>
      <div class="ybox">
        {at.map((z, i) => {
          const p = atParts(z.at);
          const go = !!p.node || p.worker != null || !!p.a || p.f === 'description';
          const cls = [marks[z.at] === 'err' ? 'err' : marks[z.at] ? 'warn' : '', added.includes(i) ? 'ins' : '', removed.includes(i) ? 'del' : '', lastDel && i === n - 1 ? 'dele' : '', isCurLine(z, cur) ? 'cur' : '', step.has(i) ? 'step' : '', go ? 'go' : ''].filter(Boolean).join(' ');
          return (
            <div class={'yl ' + cls} data-act={go ? 'tyline' : null} data-node={p.node || null} data-worker={p.worker != null ? p.worker : null} data-a={p.a || null} data-b={p.b || null} data-f={p.f || null} onClick={go ? () => pressLine(r, { ...z, ...p }) : undefined}>
              <span class="lno">{i + 1}</span>
              <span class="yt"><YamlLine text={z.text} /></span>
            </div>
          );
        })}
      </div>
    </aside>
  );
}

// yamlHL is the YAML a preview line shows, with the keys and the arrows marked
// (モックのバージョン 7 から逐語).
function YamlLine({ text }: { text: string }): JSX.Element | string {
  let m = text.match(/^(\s*)(- )?([A-Za-z_]+):( .*)?$/);
  if (m) return <>{m[1]}{m[2] ? <span class="ar">- </span> : ''}<span class="k">{m[3]}:</span>{m[4] || ''}</>;
  m = text.match(/^(\s*- )(\S+) -> (\S+)$/);
  if (m) return <><span class="ar">{m[1]}</span>{m[2]}<span class="ar">{' -> '}</span>{m[3]}</>;
  return text;
}

// LineAt is what one line of the preview stands for, as the data attributes a
// press on the line carries back to the figure.
type LineAt = { node?: string; worker?: number; a?: string; b?: string; f?: string };

const atParts = (at: string): LineAt => {
  if (!at) return {};
  if (at.startsWith('node:')) return { node: at.slice(5) };
  if (at.startsWith('worker:')) return { worker: Number(at.slice(7)) };
  if (at.startsWith('turn:')) { const [a, b] = at.slice(5).split('->'); return { a, b }; }
  if (at === 'description') return { f: 'description' };
  return {};
};

// isCurLine is whether a preview line is one the figure has picked: a point
// takes its own lines and every turn that comes in or leaves it, a line takes
// both directions of its turn, and a worker its own lines (spec-teams B7).
function isCurLine(z: { at: string } | null, cur: CanvasPick | null): boolean {
  if (!cur || !z || !z.at) return false;
  const p = atParts(z.at);
  if (cur.type === 'node') return p.node === cur.id || p.a === cur.id || p.b === cur.id;
  if (cur.type === 'edge') return !!p.a && !!p.b && pair(p.a, p.b) === cur.k;
  if (cur.type === 'worker') return p.worker === cur.i;
  return false;
}

// stepOf marks the lines one edit just moved, against the text drawn a moment
// ago: it is only for the 1.8s glow, and the saved text's own diff — the `+N
// −M` the head shows — is the server's (spec-teams B7, spec-boundary 5).
function stepOf(prev: string[] | undefined, now: string[]): Set<number> {
  const out = new Set<number>();
  if (!prev) return out;
  let i = 0;
  while (i < prev.length && i < now.length && prev[i] === now[i]) i++;
  let j = 0;
  while (j < prev.length - i && j < now.length - i && prev[prev.length - 1 - j] === now[now.length - 1 - j]) j++;
  for (let k = i; k < now.length - j; k++) out.add(k);
  return out;
}

// lineMarks is the severest finding each line stands for, so the line a
// problem is about is marked the way the figure's point is (spec-teams B7).
function lineMarks(r: Rec): Record<string, string> {
  const out: Record<string, string> = {};
  for (const m of teamIssues(r)) {
    const at = markAt(m);
    if (at && out[at] !== 'err') out[at] = m.sev;
  }
  return out;
}

// markAt is the preview line a finding stands beside: the description, a point, or
// a worker.
function markAt(m: Finding): string {
  return m.field === 'description' ? 'description' : m.node ? 'node:' + m.node : m.worker != null && m.worker >= 0 ? 'worker:' + m.worker : '';
}

// fitYbox caps the topology tab's YAML box at the room between the box's top
// and the save's band, which stands at the bottom of the window while the
// panel is taller than the window (spec-teams B7, 監査 R2).
export function fitYbox(panel: Element): void {
  const box = panel.querySelector<HTMLElement>('.tprev');
  if (!box) return;
  if (getComputedStyle(box).display === 'none') { box.style.maxHeight = ''; return; }
  const foot = panel.querySelector<HTMLElement>('.pf'), fh = foot ? foot.offsetHeight : 0;
  const h = Math.max(160, Math.floor(window.innerHeight - fh - box.getBoundingClientRect().top - 8));
  if (box.style.maxHeight !== h + 'px') box.style.maxHeight = h + 'px';
}

export function fitYboxes(): void {
  Array.from(document.querySelectorAll('.panel.team-p')).forEach(fitYbox);
}

// afterTPrev brings the lines a person is looking at into the box (spec-teams
// B7): the lines one edit just moved after an edit, and the lines the figure
// has picked after a pick. It moves the box's own scroll and nothing else, so
// opening a row leaves the page where it stands (監査 R1).
export function afterTPrev(r: Rec): void {
  const key = dkey(r);
  const panel = document.querySelector<HTMLElement>(`.panel.team-p[data-key="${CSS.escape(key)}"]`);
  if (!panel) return;
  const box = panel.querySelector<HTMLElement>('.ybox');
  if (!box || !box.clientHeight) return;
  const at = (r.preview.value || { lines: [] }).lines || [];
  const step = GLOW.step[key] || [];
  const df = (r.preview.value || {}).diff || {};
  // The lines a change moved are the glow's own step, and also the lines the
  // server's diff names — the ones it added, and the place it took one from
  // (spec-teams B7). A step the glow marks alone can miss a removal the added
  // line's shift hides (監査 R2), so both are read, and the last of them is
  // where the box is sent.
  let want = -1;
  [...step, ...(df.added || []), ...(df.removedBefore || []), ...(df.removedAtEnd ? [at.length - 1] : [])]
    .forEach((i) => { if (i > want && i < at.length) want = i; });
  if (want < 0) {
    const cur = selOf(r);
    if (cur) { for (let i = 0; i < at.length; i++) if (isCurLine(at[i], cur)) { want = i; break; } }
  }
  if (want >= 0 && box.children[want]) {
    const br = box.getBoundingClientRect(), lr = box.children[want].getBoundingClientRect();
    if (lr.top < br.top) box.scrollTop -= br.top - lr.top;
    else if (lr.bottom > br.bottom) box.scrollTop += lr.bottom - br.bottom;
  }
  if (GLOW.glow[key]) {
    const glowed = GLOW.glow[key];
    clearTimeout(GLOW.timer[key]);
    GLOW.timer[key] = setTimeout(() => {
      if (Date.now() - glowed < 1700) return;
      Array.from(box.querySelectorAll('.yl.step')).forEach((el) => el.classList.remove('step'));
    }, 1800);
  }
}
