// eslint.config.js is the undef gate's one rule (go run ./tools/check undef):
// every name the WebUI's modules use is defined in the module, imported, or one
// of the browser's own. A name a module forgot to import is a ReferenceError
// only when the line runs, and a line that runs only on one screen's one road
// (the headquarters' roles list, 3ec7db3) is one no walk may have opened.
//
// Only this rule is on: the screens are not held to a style here. No comment in
// a module turns it off: an eslint comment is not read, and the gate fails on
// the warning that says so (--max-warnings 0).
import globals from 'globals';

export default [
  {
    files: ['**/*.js'],
    ignores: ['vendor/**'],
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
      globals: { ...globals.browser },
    },
    linterOptions: { noInlineConfig: true },
    rules: { 'no-undef': 'error' },
  },
];
