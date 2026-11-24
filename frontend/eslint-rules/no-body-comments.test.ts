import { Linter } from 'eslint'
import tseslint from 'typescript-eslint'
import { describe, expect, it } from 'vitest'
import rule from './no-body-comments.js'

function count(code: string): number {
  const linter = new Linter()
  const messages = linter.verify(
    code,
    [
      {
        files: ['**/*.ts'],
        languageOptions: { parser: tseslint.parser },
        plugins: { rela: { rules: { 'no-body-comments': rule } } },
        rules: { 'rela/no-body-comments': 'error' },
      },
    ],
    'x.ts'
  )
  return messages.length
}

const longDoc = '/**\n' + ' * line\n'.repeat(11) + ' */\n'
const okDoc = '/**\n' + ' * line\n'.repeat(10) + ' */\n'

describe('no-body-comments', () => {
  const cases: [string, string, number][] = [
    ['body comment', 'function f() {\n  // explain\n  return 1\n}', 1],
    ['arrow body comment', 'const f = () => {\n  return 1 // explain\n}', 1],
    ['coverage directive', 'function f() {\n  // istanbul ignore next\n  return 1\n}', 0],
    ['ts directive', 'function f() {\n  // @ts-expect-error\n  return 1\n}', 0],
    ['why with reason', 'function f() {\n  // why: library bug 42\n  return 1\n}', 0],
    ['why without reason', 'function f() {\n  // why:\n  return 1\n}', 1],
    ['tag-only jsdoc', 'function f() {\n  /** @type {number} */\n  const a = 1\n  return a\n}', 0],
    [
      'two-line why group',
      'function f() {\n  // why: library bug 42\n  // see the upstream issue\n  return 1\n}',
      0,
    ],
    [
      'prose after a directive',
      'function f() {\n  // @ts-expect-error\n  // explain\n  return 1\n}',
      1,
    ],
    ['arrow const long doc', longDoc + 'export const f = () => {}', 1],
    ['method long doc', 'class A {\n' + longDoc + '  m() {}\n}', 1],
    ['line-comment doc over cap', '// line\n'.repeat(11) + 'function f() {}', 1],
    ['detached long comment', longDoc + '\n\n\nfunction f() {}', 0],
    [
      'long doc then directive',
      longDoc + '// istanbul ignore next\nfunction f() {}',
      1,
    ],
    ['comment outside body', '// top\nconst a = 1', 0],
    ['short doc', okDoc + 'function f() {}', 0],
    ['long doc', longDoc + 'function f() {}', 1],
    ['long doc on exported function', longDoc + 'export function f() {}', 1],
  ]
  it.each(cases)('%s', (_name, code, want) => {
    expect(count(code)).toBe(want)
  })
})
