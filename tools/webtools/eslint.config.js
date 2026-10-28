// eslint.config.js is the names gate's two rules (go run ./tools/check names):
// every name the WebUI's modules use is defined in the module, imported, or one
// of the browser's own (no-undef), and every name a module defines or imports
// is read (no-unused-vars). A name a module forgot to import is a
// ReferenceError only when the line runs, and a line that runs only on one
// screen's one road (the headquarters' roles list, 3ec7db3) is one no walk may
// have opened. A name nothing reads is what a change left behind: Go has the
// deadcode gate for that, and the modules have this.
//
// Only these two rules are on: the screens are not held to a style here. An
// argument a handler is given and does not read is not counted, nor an error a
// catch does not read. No comment in a module turns a rule off: an eslint
// comment is not read, and the gate fails on the warning that says so
// (--max-warnings 0).
import globals from 'globals';

export default [
  {
    files: ['**/*.js'],
    ignores: ['static/vendor/**'],
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
      globals: { ...globals.browser },
    },
    linterOptions: { noInlineConfig: true },
    rules: {
      'no-undef': 'error',
      'no-unused-vars': ['error', { args: 'none', caughtErrors: 'none' }],
    },
  },
];
