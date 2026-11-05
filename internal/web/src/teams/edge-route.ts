import type { Box, Positions, Spot } from './place.js';

type Obstacle = { left: number; right: number; top: number; bottom: number };
function crosses(a: Spot, b: Spot, box: Obstacle): boolean {
 let lo = 0, hi = 1;
 for (const [origin, delta, min, max] of [[a[0], b[0] - a[0], box.left + .01, box.right - .01], [a[1], b[1] - a[1], box.top + .01, box.bottom - .01]]) {
  if (!delta) { if (origin <= min || origin >= max) return false; }
  else { const first = (min - origin) / delta, last = (max - origin) / delta; lo = Math.max(lo, Math.min(first, last)); hi = Math.min(hi, Math.max(first, last)); }
 }
 return lo <= hi;
}

// Route on the boxes' outside edges. Even a packed row's direct connection
// remains a connection between its two endpoints, rather than a hidden line.
export function routeEdge(from: Spot, to: Spot, positions: Positions, boxes: Record<string, Box>): Spot[] {
 const obstacles = Object.entries(positions).filter(([id]) => boxes[id]).map(([id, p]) => ({ left: p[0] - boxes[id].w / 2 - 2, right: p[0] + boxes[id].w / 2 + 2, top: p[1] - boxes[id].h / 2 - 2, bottom: p[1] + boxes[id].h / 2 + 2 }));
 const clear = (a: Spot, b: Spot) => !obstacles.some(box => crosses(a, b, box));
 if (clear(from, to)) return [from, to];
 const xs = [...new Set([from[0], to[0], ...obstacles.flatMap(box => [box.left - 6, box.right + 6])])].sort((a, b) => a - b);
 const ys = [...new Set([from[1], to[1], ...obstacles.flatMap(box => [box.top - 6, box.bottom + 6])])].sort((a, b) => a - b);
 const key = (x: number, y: number) => y * xs.length + x;
 const point = (id: number): Spot => [xs[id % xs.length], ys[Math.floor(id / xs.length)]];
 const start = key(xs.indexOf(from[0]), ys.indexOf(from[1])), end = key(xs.indexOf(to[0]), ys.indexOf(to[1]));
 const distance = new Map([[start, 0]]), previous = new Map<number, number>(), pending = new Set([start]);
 const estimate = (id: number) => { const p = point(id); return distance.get(id)! + Math.abs(to[0] - p[0]) + Math.abs(to[1] - p[1]); };
 while (pending.size) {
  const current = [...pending].sort((a, b) => estimate(a) - estimate(b))[0];
  if (current === end) {
   const path = [to]; let at = end;
   while (at !== start) { at = previous.get(at)!; path.unshift(point(at)); }
   return path.filter((p, i) => !i || i === path.length - 1 || !((path[i - 1][0] === p[0] && p[0] === path[i + 1][0]) || (path[i - 1][1] === p[1] && p[1] === path[i + 1][1])));
  }
  pending.delete(current);
  const x = current % xs.length, y = Math.floor(current / xs.length), a = point(current);
  for (const [nx, ny] of [[x - 1, y], [x + 1, y], [x, y - 1], [x, y + 1]]) {
   if (nx < 0 || nx >= xs.length || ny < 0 || ny >= ys.length) continue;
   const next = key(nx, ny), b = point(next), cost = distance.get(current)! + Math.abs(b[0] - a[0]) + Math.abs(b[1] - a[1]);
   if (cost >= (distance.get(next) ?? Infinity) || !clear(a, b)) continue;
   distance.set(next, cost); previous.set(next, current); pending.add(next);
  }
 }
 return [from, to];
}
