// The one module the WebUI imports the three packages through: the screens
// write `import { h, html, signal, useSignal } from '../vendor/preact.js'`, so
// what they may use is decided here and nowhere else (webui-preact-design 3
// 章). Bun bundles this file and its dependencies into
// internal/web/static/vendor/preact.js; nothing at run time fetches a package
// from the network (CSP stays default-src 'self').
import { h } from 'preact';
import htm from 'htm';

// html is htm bound to Preact's createElement: template literals become the
// trees Preact renders, so a screen never builds a string of markup.
export const html = htm.bind(h);

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
