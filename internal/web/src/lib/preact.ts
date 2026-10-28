// lib/preact.ts — the one module the WebUI reads the three packages through: the
// screens write `import { html, signal } from '../lib/preact.js'`, so what they
// may use is decided here and nowhere else (webui-preact-design 3 章). The
// packages are the ones pinned in tools/webvendor, and webbuild is what folds
// this module into the files static/ serves; nothing at run time fetches a
// package from the network (CSP stays default-src 'self').
//
// JSX is the way a screen draws now: tsconfig's jsxImportSource is the package
// itself, so `<div />` becomes preact/jsx-runtime's jsx(), with no import in the
// screen. That is the same package instance this module re-exports, so a screen
// still has one copy of Preact and one set of signals. What a screen writes by
// hand goes through this file: packages_test holds every source to it, and reads
// `import type` as a type alone (it loads nothing in the browser).
import { h, type VNode } from 'preact';
import htm from 'htm';

// html is htm bound to Preact's createElement: template literals become the
// trees Preact renders, so a screen never builds a string of markup. It is what
// the screens that have not moved to JSX still draw with; a file that draws JSX
// does not need it.
export const html: (strings: TemplateStringsArray, ...values: unknown[]) => VNode | VNode[] = htm.bind(h);

// Preact itself: the element factory, the render entry, and the few things a
// screen needs to describe a tree.
export {
  h,
  createElement,
  Fragment,
  render,
  hydrate,
  cloneElement,
  createRef,
  createContext,
  toChildArray,
  Component,
  options,
} from 'preact';

// The hooks (preact/hooks), all of them: a screen picks what it needs.
export {
  useCallback,
  useContext,
  useDebugValue,
  useEffect,
  useErrorBoundary,
  useId,
  useImperativeHandle,
  useLayoutEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
} from 'preact/hooks';

// The signals (@preact/signals): state that a component reads and re-renders
// on, without a store of the screen's own.
export { signal, computed, effect, batch, untracked, useSignal, useComputed, useSignalEffect } from '@preact/signals';
