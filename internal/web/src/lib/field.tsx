// lib/field.tsx — 文字の欄の 1 つの仕組み（r-1ee4、webui-preact-design 5 章 1）。
//
// 画面の欄は、値が変わるたびに描き直される。値を Preact の prop で毎回入れ直す
// （controlled な欄）と、描き直しのたびに DOM の値へ同じ言葉を書き戻すことになり、
// それがカーソルを末尾へ動かし、IME の変換を打ち切る。だからこの部品は、欄が
// 画面の値と違う所に立っているときだけ値を書き、変換の途中では書かない。
//
//   <Field as="input" class="in-name" data-f="name" value={d.name} onInput={...} />
//
// 変換の間、欄の中身はブラウザが持ち、入力のたびに画面の値もそれに合わせて
// 動く。画面の値と違うときだけ書くので、欄を直している人には何も起きない。
import { useLayoutEffect, useRef } from './preact.js';

export type FieldProps = {
  as?: 'input' | 'textarea';
  value?: string | number | null;
  // fieldRef is the element itself, for a part that moves the caret or measures
  // the box (the profile screen's focus ask, the environment value's height).
  fieldRef?: { current: HTMLElement | null } | null;
  [key: string]: unknown;
};

// Field is an input or a textarea whose value the screen keeps, written into the
// field only when the field stands somewhere else than the screen says — a rule
// put back with 元に戻す, a section drawn anew, a carried draft — and never while
// an IME is composing: the letters a person is making are the browser's until
// they are committed, and writing over them ends the conversion and loses them
// (r-1ee4).
export function Field({ as: Tag = 'input', value, fieldRef, ...rest }: FieldProps) {
  const input = useRef<HTMLInputElement | null>(null);
  const textarea = useRef<HTMLTextAreaElement | null>(null);
  const composing = useRef(false);
  const field = (): HTMLInputElement | HTMLTextAreaElement | null => (Tag === 'textarea' ? textarea.current : input.current);
  useLayoutEffect(() => {
    const one = field();
    if (!one || composing.current) return;
    const want = value === null || value === undefined ? '' : String(value);
    if (one.value !== want) one.value = want;
  });
  const keep = (el: HTMLInputElement | HTMLTextAreaElement | null): void => {
    if (Tag === 'textarea') textarea.current = el as HTMLTextAreaElement | null;
    else input.current = el as HTMLInputElement | null;
    if (fieldRef) fieldRef.current = el;
  };
  return Tag === 'textarea'
    ? <textarea {...rest} ref={keep} onCompositionStart={() => { composing.current = true; }} onCompositionEnd={() => { composing.current = false; }} />
    : <input {...rest} ref={keep} onCompositionStart={() => { composing.current = true; }} onCompositionEnd={() => { composing.current = false; }} />;
}

// inText is whether a key belongs to the field it was pressed in: Enter commits
// an IME's conversion, Space chooses one, and neither is a press on the row the
// field stands in. A row's own keys — opening and shutting it — leave the field
// alone, so the conversion a person confirms with Enter does not shut the row
// under their hands (r-1ee4).
export function inText(e: KeyboardEvent): boolean {
  if (e.isComposing || e.keyCode === 229) return true;
  const t = e.target;
  return t instanceof Element && !!t.closest('input, textarea, select, [contenteditable="true"]');
}
