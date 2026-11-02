import { expect, test } from 'bun:test';

test('tab arrows leave modified browser shortcuts alone', async () => {
  document.body.innerHTML = '<nav role="tablist"><button role="tab">One</button><button role="tab">Two</button></nav>';
  await import('../src/lib/nav');
  const [one, two] = document.querySelectorAll('[role="tab"]');
  let clicks = 0;
  two.addEventListener('click', () => { clicks++; });
  for (const modifier of ['altKey', 'ctrlKey', 'metaKey', 'shiftKey', 'isComposing']) {
    const event = new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true, [modifier]: true });
    one.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(false);
  }
  expect(clicks).toBe(0);
  const arrow = new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true });
  one.dispatchEvent(arrow);
  expect(arrow.defaultPrevented).toBe(true);
  expect(clicks).toBe(1);
});
