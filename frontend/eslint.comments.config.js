import tseslint from 'typescript-eslint'
import vueParser from 'vue-eslint-parser'
import noBodyComments from './eslint-rules/no-body-comments.js'

const plugins = { rela: { rules: { 'no-body-comments': noBodyComments } } }
const rules = { 'rela/no-body-comments': 'error' }

export default [
  {
    ignores: [
      'dist/**',
      'node_modules/**',
      'packages/**',
      '**/*.d.ts',
      '**/*.test.ts',
      '**/*.spec.ts',
      'e2e/**',
    ],
  },
  {
    files: ['**/*.ts', '**/*.tsx'],
    languageOptions: { parser: tseslint.parser },
    plugins,
    rules,
  },
  {
    files: ['**/*.vue'],
    languageOptions: {
      parser: vueParser,
      parserOptions: { parser: tseslint.parser, ecmaVersion: 'latest', sourceType: 'module' },
    },
    plugins,
    rules,
  },
]
