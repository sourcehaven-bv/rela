/*
 * Guards against a prop that exists but cannot be reached.
 *
 * A wrapper component renders a child and re-exposes the child's props as its
 * own. When a prop is added to the child and not to the wrapper, everything
 * still compiles: Vue drops an unknown attribute silently, and passing
 * `:some-prop="false"` from a consumer typechecks clean because the attribute
 * simply falls through as a plain attribute instead. The child keeps its
 * default, the feature is absent, and nothing anywhere reports it.
 *
 * That shape is invisible to the tests most likely to be written for it. A DOM
 * assertion about the child passes when the child is driven directly in its own
 * story; an end-to-end test passes because the missing feature usually only
 * changes layout. It shows up by reading rendered HTML, which is not a thing
 * anyone does on purpose.
 *
 * So the rule is: where a wrapper renders a child and already forwards some of
 * the child's props, it forwards all of them. A prop the wrapper deliberately
 * owns is opted out by name below.
 *
 * What this deliberately does not catch: a component binding few or none of a
 * child's props is a plain user of that child rather than a wrapper
 * re-exposing it, so it falls under the threshold and is not judged. That is
 * the line that stops the check firing on every incidental use in the
 * codebase, and it means a wrapper that forwards almost nothing reads as not
 * a wrapper at all. Adding the first few forwarded props is what brings it
 * into scope.
 *
 * Run with `npm run check:forwarding`.
 */

import { readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join, relative } from 'node:path'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const src = join(root, 'src')

/**
 * Props a wrapper is allowed not to forward, keyed by `Wrapper>Child`.
 *
 * Every entry needs a reason. The default is to forward, because "the wrapper
 * does not need this one" is exactly what the bug looks like from the inside.
 */
const ALLOWED = {
  // The section is one of many; the table hands it the one it is iterating.
  'RlTable>RlTableSection': ['section'],

  /*
   * `labelMode` picks whether the shell wraps the control in a `<label>` or
   * points at it with `for`. Only the field knows which its own markup needs:
   * a native input takes `for`, a composite control has to be wrapped.
   * Forwarding it would let a consumer break the field's label association
   * from the outside.
   */
  'RlDateField>RlFieldShell': ['labelMode'],
  'RlNumberField>RlFieldShell': ['labelMode'],
  'RlSelect>RlFieldShell': ['labelMode'],
  'RlTextField>RlFieldShell': ['labelMode'],
  'RlTextarea>RlFieldShell': ['labelMode'],
  'RlFileField>RlFieldShell': ['labelMode'],
  'RlMultiSelect>RlFieldShell': ['labelMode'],
  'RlRadioGroup>RlFieldShell': ['labelMode'],
  'RlDateRangeField>RlFieldShell': ['labelMode'],

  /*
   * A switch wraps its label and is never required: off is a valid answer,
   * so an asterisk would promise a validation that cannot fire. `labelMode`
   * is fixed for the same reason as the fields above.
   */
  'RlSwitch>RlFieldShell': ['labelMode', 'required'],

  /*
   * The person field owns a focusable box, so `for` is the only mode that
   * makes sense: a wrapping label would put a second click target around a
   * control that already has one, and clicking the name would open the list.
   */
  'RlPersonField>RlFieldShell': ['labelMode'],

  /*
   * The dot's `size` is a pixel diameter and the heading's own `size` prop
   * is a scale step. Exposing the first would let the two disagree, and the
   * dot is meant to track the heading rather than be set against it.
   */
  'RlSectionHeading>RlStatusDot': ['size'],

  /*
   * The sidebar places the theme picker in its footer at one size. A
   * consumer wanting it elsewhere, icon-only, or writing to a different
   * element uses `RlThemeToggle` directly through the `footer` slot.
   */
  'RlSidebar>RlThemeToggle': ['iconOnly', 'size', 'target', 'label'],

  // Both render the icon `aria-hidden`, so it is decorative and takes no name.
  'RlAvatar>RlIcon': ['label'],
  'RlNavIcon>RlIcon': ['label'],
}

/** Every .vue file under src/, recursively. */
function* vueFiles(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) yield* vueFiles(path)
    else if (entry.name.endsWith('.vue')) yield path
  }
}

const sources = new Map()
for (const path of vueFiles(src)) sources.set(path, readFileSync(path, 'utf8'))

const byName = new Map()
for (const path of sources.keys()) byName.set(path.split('/').pop().replace(/\.vue$/, ''), path)

/** The text inside `defineProps<{ ... }>()`, brace-matched. */
function propsBlock(source) {
  const start = source.indexOf('defineProps<{')
  if (start === -1) return null
  let depth = 0
  for (let i = source.indexOf('{', start); i < source.length; i++) {
    if (source[i] === '{') depth++
    else if (source[i] === '}' && --depth === 0) return source.slice(source.indexOf('{', start) + 1, i)
  }
  return null
}

/**
 * Declared prop names. Comments are stripped first so a prop named in prose
 * is never counted, and nested object literals are skipped so that a prop
 * whose type has members does not contribute those members as props.
 */
function declaredProps(source) {
  const block = propsBlock(source)
  if (block === null) return []

  const cleaned = block.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/[^\n]*/g, '')
  const names = []
  let depth = 0
  let atStart = true

  for (let i = 0; i < cleaned.length; i++) {
    const char = cleaned[i]
    if (char === '{' || char === '(' || char === '[') depth++
    else if (char === '}' || char === ')' || char === ']') depth--
    else if (char === ';' || char === ',' || char === '\n') atStart = true
    else if (depth === 0 && atStart && /[A-Za-z_$]/.test(char)) {
      const match = cleaned.slice(i).match(/^([A-Za-z_$][\w$]*)\s*\??\s*:/)
      if (match) names.push(match[1])
      atStart = false
    } else if (!/\s/.test(char)) atStart = false
  }

  return names
}

const kebab = (name) => name.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`)

const failures = []

for (const [path, source] of sources) {
  const wrapper = path.split('/').pop().replace(/\.vue$/, '')
  const template = source.slice(source.indexOf('<template>'))

  for (const [, child] of template.matchAll(/<(Rl[A-Za-z]+)[\s/>]/g)) {
    const childPath = byName.get(child)
    if (!childPath || childPath === path) continue

    const childProps = declaredProps(sources.get(childPath))
    if (childProps.length === 0) continue

    /*
     * Only the attributes on this child's own tags. Scanning the whole
     * template counts a prop bound on any sibling as bound here, which
     * inflates the forwarded count and can suppress a real finding: a
     * sidebar binding `label` on an icon button would be credited with
     * forwarding the theme toggle's `label` too.
     */
    const tags = [...template.matchAll(new RegExp(`<${child}[\\s/>][^>]*>`, 'g'))]
      .map((match) => match[0])
      .join(' ')

    /*
     * Only wrappers, not arbitrary users. A component that renders a child
     * for its own purposes binds a few of the child's props and owns the
     * rest; a wrapper re-exposes them. The signal is that the wrapper
     * declares and forwards most of what the child takes.
     */
    const bound = childProps.filter((prop) =>
      new RegExp(`[:\\s](?:${prop}|${kebab(prop)})=`).test(tags) ||
      // `v-bind="obj"` forwards whatever the object holds.
      /v-bind="[^"]+"/.test(tags),
    )
    const own = new Set(declaredProps(source))
    const forwarded = bound.filter((prop) => own.has(prop))
    if (forwarded.length < childProps.length / 2) continue

    const allowed = new Set(ALLOWED[`${wrapper}>${child}`] ?? [])
    const missing = childProps.filter(
      (prop) => !bound.includes(prop) && !allowed.has(prop),
    )

    for (const prop of missing) {
      failures.push(
        `${relative(root, path)}: \`${wrapper}\` forwards ` +
          `${forwarded.length} of \`${child}\`'s props but not \`${prop}\`, ` +
          `so a consumer of \`${wrapper}\` cannot reach it.`,
      )
    }
  }
}

if (failures.length) {
  console.error('Forwarding check failed:\n')
  for (const failure of failures) console.error(`  - ${failure}`)
  console.error(
    '\nAn unforwarded prop compiles, typechecks and renders. Vue drops the\n' +
      'unknown attribute, the child keeps its default, and the feature is\n' +
      'simply absent. Forward it, or add it to ALLOWED with a reason.',
  )
  process.exit(1)
}

console.log('Forwarding check passed: every wrapper reaches all of its child\'s props.')
