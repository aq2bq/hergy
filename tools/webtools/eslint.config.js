// tsc checks TypeScript names and private state. These built-in ESLint rules
// guard native DOM operations whose owner TypeScript cannot express. File
// scopes below identify the owner of each local scroller (d-a9ed, d-832b).
import globals from 'globals';
import parser from '@typescript-eslint/parser';

const scrollOwners = {
  'lib/page-position.ts': ['scrollBy', 'scrollTo', 'scrollIntoView', 'scrollTop', 'scrollLeft'],
  'teams/canvas/prompt-comparison.tsx': ['scrollTop'],
  'teams/canvas/yaml.tsx': ['scrollTop'],
  'lib/yamltab.tsx': ['scrollTop', 'scrollLeft'],
  'lib/prose-transfer-view.tsx': ['scrollTop'],
  'lib/hq/sheets.tsx': ['scrollTop', 'scrollIntoView'],
  'teams/role-modal.tsx': ['scrollIntoView'],
};
const scoped = [...new Set([...Object.keys(scrollOwners), 'lib/address.ts', 'lib/editing.ts', 'lib/prose-transfer.ts', 'lib/danger.ts'])];
const restriction = (selector, message) => ({ selector, message });
function rules(file) {
  const allowed = scrollOwners[file] || [];
  const syntax = ['scrollTop', 'scrollLeft'].filter((p) => !allowed.includes(p)).flatMap((p) => [
    restriction(`AssignmentExpression[left.type='MemberExpression'][left.property.name='${p}']`, '欄のスクロールは、その欄の持ち主だけが動かします。'),
    restriction(`UpdateExpression[argument.type='MemberExpression'][argument.property.name='${p}']`, '欄のスクロールは、その欄の持ち主だけが動かします。'),
  ]);
  if (file !== 'lib/page-position.ts') syntax.push(restriction("CallExpression[callee.type='MemberExpression'][callee.property.name='focus']:not(:has(Property[key.name='preventScroll'][value.value=true]))", 'focus は preventScroll: true を渡します。'));
  if (file !== 'lib/address.ts') syntax.push(restriction("CallExpression[callee.type='MemberExpression'][callee.property.name=/^(get|set|delete)$/][arguments.0.value='open']", '開いている行の住所は address の操作で読み書きします。'));
  if (!['lib/editing.ts', 'lib/prose-transfer.ts'].includes(file)) syntax.push(restriction("BinaryExpression[operator=/^(===|!==|==|!=)$/] > CallExpression[callee.object.name='JSON'][callee.property.name='stringify']", '編集中のリソースの変更は editing の読み取りで答えます。'));
  syntax.push(restriction("JSXAttribute[name.name=/^data-danger(-act)?$/]", '危険操作の印は dangerAttrs の既知の操作名から作ります。'));
  if (!['lib/danger.ts', 'lib/prose-transfer-view.tsx'].includes(file)) syntax.push(restriction("CallExpression[callee.type='MemberExpression'][callee.property.name=/^(addEventListener|removeEventListener)$/][arguments.0.value='beforeunload']", 'ページを閉じる確認は danger の操作で扱います。'));
  return {
    'no-restricted-properties': ['error', { object: 'window', property: 'confirm', message: '確認は runDanger の操作で行います。' }, ...['scrollBy', 'scrollTo', 'scrollIntoView'].filter((p) => !allowed.includes(p)).map((property) => ({ property, message: 'ページの位置は movePage の操作で動かします。' }))],
    'no-restricted-syntax': ['error', ...syntax],
    'no-restricted-globals': ['error', { name: 'confirm', message: '確認は runDanger の操作で行います。' }],
  };
}
export default [
  { files: ['**/*.js'], languageOptions: { ecmaVersion: 'latest', sourceType: 'module', globals: { ...globals.browser } }, linterOptions: { noInlineConfig: true }, rules: { 'no-undef': 'error', 'no-unused-vars': ['error', { args: 'none', caughtErrors: 'none' }] } },
  { files: ['src/**/*.{ts,tsx}'], languageOptions: { parser, parserOptions: { ecmaFeatures: { jsx: true } } }, linterOptions: { noInlineConfig: true }, rules: rules('') },
  ...scoped.map((file) => ({ files: [`src/${file}`], rules: rules(file) })),
];
