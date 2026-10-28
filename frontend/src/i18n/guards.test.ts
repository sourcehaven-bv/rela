// @vitest-environment node
/**
 * Guards for the interface text (TKT-ZL1QAO):
 *
 * - The bare-string ratchet. ESLint rejects a bare string in any template not
 *   listed in `bare-strings.json`. A listed file is held to its recorded count
 *   in both directions: more means a new bare string, fewer means the record
 *   must come down. So the list only shrinks, as `internal/archguard`'s
 *   bare-ref allowlist does.
 * - Key parity: every key in every locale file, with the same placeholders
 *   and plural forms.
 * - No unused keys: every key appears literally somewhere in `src/`.
 */
import { readFileSync, readdirSync, existsSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { ESLint } from 'eslint'
import pluginVue from 'eslint-plugin-vue'
import vueParser from 'vue-eslint-parser'
import tseslint from 'typescript-eslint'
import { MESSAGES } from './language'

const FRONTEND = fileURLToPath(new URL('../..', import.meta.url))
const BARE = JSON.parse(readFileSync(join(FRONTEND, 'src/i18n/bare-strings.json'), 'utf8')) as {
  ruleOptions: unknown
  allowlist: Record<string, number>
}
const RULE = 'vue/no-bare-strings-in-template'

// The allowlist may only shrink. These are its size when the guard was added
// (TKT-ZL1QAO); lower them as files are converted, never raise them.
const MAX_LISTED_FILES = 79
const MAX_LISTED_STRINGS = 633

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) walk(path, out)
    else out.push(path)
  }
  return out
}

/** `{a: {b: 'x'}}` → `{'a.b': 'x'}`. */
function flatten(obj: Record<string, unknown>, prefix = ''): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k
    if (v && typeof v === 'object') Object.assign(out, flatten(v as Record<string, unknown>, key))
    else out[key] = String(v)
  }
  return out
}

/**
 * Drops block comments and `//` line comments, so a key mentioned in a doc
 * comment does not count as used. A `//` straight after `:` is a URL, not a
 * comment.
 */
function stripComments(source: string): string {
  return source.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/.*$/gm, '$1')
}

describe('bare-string ratchet', () => {
  it('holds every listed file to its recorded count', async () => {
    const eslint = new ESLint({
      cwd: FRONTEND,
      overrideConfigFile: true,
      overrideConfig: [
        {
          files: ['**/*.vue'],
          plugins: { vue: pluginVue },
          languageOptions: {
            parser: vueParser,
            parserOptions: { parser: tseslint.parser, sourceType: 'module' },
          },
          rules: { [RULE]: ['error', BARE.ruleOptions] },
        },
      ],
    })
    const listed = Object.keys(BARE.allowlist)
    const missing = listed.filter((file) => !existsSync(join(FRONTEND, file)))
    expect(
      missing,
      'listed files that no longer exist: remove them from src/i18n/bare-strings.json'
    ).toEqual([])

    const results = await eslint.lintFiles(listed.map((file) => join(FRONTEND, file)))
    const drift: string[] = []
    for (const r of results) {
      const file = relative(FRONTEND, r.filePath)
      const actual = r.messages.filter((m) => m.ruleId === RULE).length
      const recorded = BARE.allowlist[file]
      if (actual > recorded) {
        drift.push(
          `${file}: ${actual} bare strings, ${recorded} allowed. Use t('area.key') for the new text.`
        )
      } else if (actual < recorded) {
        drift.push(
          `${file}: ${actual} bare strings, ${recorded} recorded. Lower the count in src/i18n/bare-strings.json` +
            (actual === 0 ? ' by removing the entry.' : ` to ${actual}.`)
        )
      }
    }
    expect(drift).toEqual([])
  }, 60_000)

  it('never grows past its starting size', () => {
    const counts = Object.values(BARE.allowlist)
    expect(counts.length, 'files in src/i18n/bare-strings.json').toBeLessThanOrEqual(MAX_LISTED_FILES)
    expect(
      counts.reduce((a, b) => a + b, 0),
      'bare strings in src/i18n/bare-strings.json'
    ).toBeLessThanOrEqual(MAX_LISTED_STRINGS)
  })

  it('rejects a bare string and a locale-less Intl call in a file that is not listed', async () => {
    // The project's own config, so this fails if the rule is switched off or
    // the allowlist stops being applied as an ignore list only.
    const eslint = new ESLint({ cwd: FRONTEND })
    const [result] = await eslint.lintText(
      [
        '<script setup lang="ts">',
        'const when = new Date().toLocaleString()',
        'const n = new Intl.NumberFormat().format(1)',
        '</script>',
        '<template>',
        '  <button title="Save now">Save {{ when }}</button>',
        '</template>',
        '',
      ].join('\n'),
      { filePath: join(FRONTEND, 'src/components/NewI18nGuardProbe.vue') }
    )
    const rules = result.messages.map((m) => m.ruleId)
    expect(rules.filter((r) => r === RULE)).toHaveLength(2)
    expect(rules.filter((r) => r === 'no-restricted-syntax')).toHaveLength(2)
  }, 60_000)
})

describe('message files', () => {
  const flat = Object.fromEntries(
    Object.entries(MESSAGES).map(([lang, messages]) => [
      lang,
      flatten(messages as Record<string, unknown>),
    ])
  )
  const en = flat.en

  it.each(Object.keys(flat).filter((lang) => lang !== 'en'))(
    '%s has exactly the English keys',
    (lang) => {
      expect(Object.keys(flat[lang]).sort()).toEqual(Object.keys(en).sort())
    }
  )

  it.each(Object.keys(flat).filter((lang) => lang !== 'en'))(
    '%s keeps the placeholders and plural forms of each English message',
    (lang) => {
      const shape = (msg: string) => ({
        placeholders: [...msg.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort(),
        forms: msg.split('|').length,
      })
      const mismatched = Object.keys(en).filter(
        (key) => JSON.stringify(shape(en[key])) !== JSON.stringify(shape(flat[lang][key] ?? ''))
      )
      expect(mismatched).toEqual([])
    }
  )

  it('uses every key literally somewhere in src/', () => {
    const sources = walk(join(FRONTEND, 'src'))
      .filter((file) => /\.(ts|vue)$/.test(file) && !/\.(test|spec)\.ts$/.test(file))
      .map((file) => stripComments(readFileSync(file, 'utf8')))
      .join('\n')
    const unused = Object.keys(en).filter(
      (key) =>
        !sources.includes(`'${key}'`) &&
        !sources.includes(`"${key}"`) &&
        !sources.includes(`\`${key}\``)
    )
    expect(
      unused,
      'remove these keys, or reference them literally (see src/i18n/index.ts)'
    ).toEqual([])
  })
})
