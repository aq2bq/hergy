import { expect, test } from 'bun:test';
import { routeEdge } from '../src/teams/edge-route';

const boxes = { a: { w: 80, h: 40 }, middle: { w: 80, h: 40 }, b: { w: 80, h: 40 } };
const positions = { a: [-150, 0], middle: [0, 0], b: [150, 0] };
test('a direct connection across a packed row avoids the intervening member', () => {
 const route = routeEdge([-107, 0], [107, 0], positions, boxes);
 expect(route[0]).toEqual([-107, 0]); expect(route.at(-1)).toEqual([107, 0]);
 expect(route.length).toBeGreaterThan(2);
 for (let i = 1; i < route.length; i++) {
  const a = route[i - 1], b = route[i];
  if (a[1] === b[1]) expect(Math.abs(a[1]) >= 22 || Math.max(a[0], b[0]) <= -42 || Math.min(a[0], b[0]) >= 42).toBe(true);
  else expect(Math.abs(a[0]) >= 42 || Math.max(a[1], b[1]) <= -22 || Math.min(a[1], b[1]) >= 22).toBe(true);
 }
 expect(positions.middle).toEqual([0, 0]);
});
test('an unobstructed connection stays straight, including a vertical connection', () => {
 expect(routeEdge([-107, -60], [107, -60], positions, boxes)).toEqual([[-107, -60], [107, -60]]);
 expect(routeEdge([70, -60], [70, 60], positions, boxes)).toEqual([[70, -60], [70, 60]]);
});
