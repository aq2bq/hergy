import { expect, test } from 'bun:test';
import { clockText, lengthText } from '../src/activity/time.js';

test('activity formats elapsed seconds and lower bounds in Japanese', () => {
  expect(lengthText(0, false)).toBe('1 秒');
  expect(lengthText(30, false)).toBe('30 秒');
  expect(lengthText(25 * 60, false)).toBe('25 分');
  expect(lengthText(2 * 3600, false)).toBe('2 時間');
  expect(lengthText(4 * 3600 + 5 * 60, true)).toBe('4 時間 5 分以上');
});

test('activity shows the date when a timestamp is outside today', () => {
  const now = new Date(2026, 9, 17, 12, 0);
  expect(clockText(new Date(2026, 9, 17, 10, 0).getTime(), now)).toBe('10:00');
  expect(clockText(new Date(2026, 9, 16, 11, 0).getTime(), now)).toBe('10/16 11:00');
});
