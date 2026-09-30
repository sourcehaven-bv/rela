/*
 * Guards the line between a component library and an application.
 *
 * A component in this library takes props and emits events. It does not know
 * where its data came from, where the answer goes, or what else is on screen.
 * That is what lets the same component appear in a story, in an example page
 * and in a consuming app without any of the three arranging anything first.
 *
 * The failure this catches never announces itself. A component reaches for a
 * store, a router, `fetch` or `localStorage`, and everything still works: the
 * story renders, the app renders, the tests pass, because in every one of
 * those contexts the thing it reached for happens to be there. The coupling
 * only surfaces later, when a second app wants the component and has a
 * different store, or a story wants a second instance and the two share state
 * they never agreed to share. By then the import is load-bearing and the
 * component is no longer a component.
 *
 * It is also the kind of thing that arrives by accident. "Just read the
 * current user here" is one import and it is obviously convenient. Nobody
 * decides to make the library depend on an application; it happens one
 * reasonable-looking line at a time, which is why this is a check and not a
 * convention.
 *
 * So the rules are:
 *
 *   1. A component imports only from Vue, from this library, and from the
 *      rendering dependencies the library already declares. Anything else is
 *      an application concern arriving through the back door.
 *   2. A component does not touch ambient browser state: no `fetch`, no
 *      storage, no cookies, no URL. Reading any of those is reading something
 *      the props did not say.
 *   3. Reactive state lives in a component instance, not in a module. A `ref`
 *      in a plain `.ts` module is created once for the whole page, so every
 *      component reading it shares one value. That is a store with no name,
 *      and it is the thing a component library most wants not to grow.
 *
 *      The body of `<script setup>` runs per instance, so a `ref` there is
 *      exactly right and is not what this looks at. A plain `<script>` block
 *      alongside it runs once, and so counts as a module.
 *
 * Stories and fixtures are exempt from the import rule: their whole job is to
 * be the application, supplying the data a component refuses to fetch.
 *
 * Run with `npm run check:purity`.
 */

import { readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join, relative } from 'node:path'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const src = join(root, 'src')

/**
 * Packages a component may import from.
 *
 * Every entry is here because it renders something. `vue` is the framework,
 * `@floating-ui/vue` positions the panels, `@milkdown/kit` and its ProseMirror
 * internals are the markdown editor's engine, and `lucide-vue-next` is the
 * icon set. A state manager, a router, a data client or an HTTP library does
 * not belong on this list, and adding one is the decision this check exists
 * to make deliberate.
 */
const ALLOWED_PACKAGES = [
  'vue',
  '@floating-ui/vue',
  '@milkdown/kit',
  'lucide-vue-next',
  'remark-stringify',
  /*
   * The boards' drag and drop. It renders nothing: it attaches the browser's
   * own drag events to an element the component already owns, which is why a
   * dragged card stays whatever the `card` slot rendered instead of becoming
   * something the library draws. Kept off the components themselves and
   * behind `useBoardDnd`, so the boards import a composable rather than a
   * vendor, and swapping it is one file.
   */
  '@atlaskit/pragmatic-drag-and-drop',
  '@atlaskit/pragmatic-drag-and-drop-auto-scroll',
]

/**
 * Module-level reactive state a component is allowed to keep, keyed by file.
 *
 * Every entry needs a reason, because shared state in a component library is
 * the exact shape of the problem this check is about.
 */
const ALLOWED_MODULE_STATE = {
  /*
   * The toast queue is deliberately one queue. A toast is raised by the code
   * that knows something happened, and shown by the corner of the screen that
   * owns the notifications; those are never the same component, and passing a
   * host handle down to every possible raiser is the coupling this avoids.
   * It stays acceptable because the state is a display queue, not domain data:
   * nothing in it outlives the message being read.
   */
  'src/components/feedback/useToasts.ts': 'the toast queue is shared by design',

  /*
   * The overlay stack is one stack because the screen has one. Two modals
   * cannot each believe they are topmost, so Escape and the scrim need a
   * single ordering that no individual overlay can own.
   */
  'src/composables/useOverlayStack.ts': 'the overlay order is a property of the screen',
}

/**
 * Ambient browser state a component may not read.
 *
 * These are the ways a component learns something its props did not tell it.
 * Each pattern is paired with what the component should do instead, because
 * the fix is always the same shape: take it as a prop, emit it as an event.
 */
const AMBIENT = [
  [/\bfetch\s*\(/, 'fetches data', 'take the data as a prop'],
  [/\bXMLHttpRequest\b/, 'makes a request', 'take the data as a prop'],
  [/\b(?:local|session)Storage\b/, 'reads or writes storage', 'take the value as a prop and emit changes'],
  [/\bdocument\s*\.\s*cookie\b/, 'reads cookies', 'take the value as a prop'],
  [/\bwindow\s*\.\s*location\b/, 'reads the URL', 'take the value as a prop'],
  [/\bhistory\s*\.\s*(?:push|replace)State\b/, 'navigates', 'emit the intent and let the app route'],
  /*
   * `navigator.clipboard` is exempt, alongside the three capability reads.
   *
   * This rule is about a component LEARNING something its props did not tell
   * it, and then rendering differently because of it — that is what makes two
   * apps with the same props disagree. Writing to the clipboard is the reverse:
   * a user asked for it, nothing is read back, and the component's output does
   * not depend on what happened. A copy button that had to take the clipboard
   * as a prop would make every consumer pass `navigator.clipboard` through.
   *
   * `readText` is deliberately NOT exempt. Reading the clipboard IS learning
   * ambient state, and a component that pasted on its own would be exactly the
   * coupling this check exists to catch.
   */
  [
    /\bnavigator\s*\.\s*(?!userAgent|platform|maxTouchPoints|clipboard\s*\??\s*\.\s*writeText|clipboard\s*\??\s*\.\s*write\b)/,
    'reads device state',
    'take it as a prop',
  ],
]

/** Reactive declarations that make state when written outside a setup block. */
const REACTIVE = /\b(?:ref|shallowRef|reactive|computed|shallowReactive)\s*[(<]/

/** Every source file under src/, recursively. */
function* files(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) yield* files(path)
    else if (/\.(vue|ts)$/.test(entry.name)) yield path
  }
}

/** Stories and fixtures are the application standing in for one. */
const isStoryOrFixture = (where) =>
  where.includes('.stories.') || where.startsWith('src/fixtures') || where.startsWith('src/pages')

/** Source with comments stripped, so a dependency named in prose never matches. */
function uncommented(source) {
  return source.replace(/\/\*[\s\S]*?\*\//g, ' ').replace(/\/\/[^\n]*/g, ' ')
}

/**
 * Source with string literals blanked too, for the rules that look for code
 * rather than for imports. Without this, a component naming `localStorage` in
 * a message it renders would read as a component using it.
 */
function code(source) {
  return uncommented(source)
    .replace(/'(?:[^'\\\n]|\\.)*'/g, "''")
    .replace(/"(?:[^"\\\n]|\\.)*"/g, '""')
    .replace(/`(?:[^`\\]|\\.)*`/g, '``')
}

/**
 * Every `<script>` block of a .vue file, or the whole of a .ts file, as
 * `{ body, perInstance }`.
 *
 * `perInstance` marks a `<script setup>` block, whose body re-runs for each
 * component instance. A plain `<script>` block, and any .ts module, runs once
 * for the page, so anything reactive declared there is shared.
 *
 * A component's template is not searched: `fetch` in an attribute name or a
 * slot called `location` is not a dependency.
 */
function scripts(source, path) {
  if (!path.endsWith('.vue')) return [{ body: source, perInstance: false }]

  return [...source.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script[^>]*>/gi)].map((match) => ({
    body: match[2],
    perInstance: /\bsetup\b/.test(match[1]),
  }))
}

/** Whether a package specifier is one of the allowed rendering dependencies. */
const isAllowedPackage = (specifier) =>
  ALLOWED_PACKAGES.some((pkg) => specifier === pkg || specifier.startsWith(`${pkg}/`))

/**
 * Lines of a module that sit outside any function, by brace depth.
 *
 * Only these create state when the module is first imported. A `ref` inside a
 * composable's exported function is made fresh on each call, which is the
 * ordinary way to share behaviour without sharing a value.
 */
function moduleScopeLines(body) {
  const lines = []
  let depth = 0
  let lineStart = 0
  let lineDepth = 0

  for (let i = 0; i <= body.length; i++) {
    const char = body[i]
    if (char === '\n' || i === body.length) {
      if (lineDepth === 0) lines.push(body.slice(lineStart, i))
      lineStart = i + 1
      lineDepth = depth
    } else if (char === '{' || char === '(' || char === '[') depth++
    else if (char === '}' || char === ')' || char === ']') {
      depth--
      if (depth < lineDepth) lineDepth = depth
    }
  }

  return lines
}

const failures = []

for (const path of files(src)) {
  const where = relative(root, path)
  if (where.startsWith('src/styles')) continue

  // Stories and fixtures are the application standing in for one.
  if (isStoryOrFixture(where)) continue

  const source = readFileSync(path, 'utf8')

  for (const { body: raw, perInstance } of scripts(source, path)) {
    const body = code(raw)

    // 1. Imports reach only Vue, this library, and the rendering dependencies.
    for (const match of uncommented(raw).matchAll(
      /*
       * The three forms a specifier arrives in: `… from '…'`, a bare
       * `import '…'` for side effects, and a dynamic `import('…')`. Spelled
       * out rather than as one pattern with optional parts, because an
       * optional `from` matches `import` against any later string in the file.
       */
      /\b(?:import|export)\b[^'"\n;]*\bfrom\s*['"]([^'"]+)['"]|\bimport\s+['"]([^'"]+)['"]|\bimport\s*\(\s*['"]([^'"]+)['"]/g,
    )) {
      // One capture group per form above; exactly one of them matched.
      const specifier = match[1] ?? match[2] ?? match[3]
      const bare = !specifier.startsWith('.') && !specifier.startsWith('/')
      if (!bare || isAllowedPackage(specifier)) continue
      failures.push(
        `${where}: imports \`${specifier}\`, which is not one of the ` +
          `library's rendering dependencies. A component takes what it needs ` +
          `as props.`,
      )
    }

    // 2. No ambient browser state.
    for (const [pattern, does, instead] of AMBIENT) {
      if (pattern.test(body)) {
        failures.push(`${where}: ${does}, which its props did not ask for. Instead, ${instead}.`)
      }
    }

    // 3. Reactive state belongs to an instance, not to the module.
    if (perInstance || ALLOWED_MODULE_STATE[where]) continue

    for (const line of moduleScopeLines(body)) {
      if (!REACTIVE.test(line)) continue
      // A type position names a ref without making one.
      if (/^\s*(?:export\s+)?(?:type|interface)\b/.test(line)) continue
      failures.push(
        `${where}: makes reactive state when the module loads ` +
          `(\`${line.trim().slice(0, 60)}\`), so every component reading it ` +
          `shares one value. Move it inside the function that returns it, or ` +
          `add the file to ALLOWED_MODULE_STATE with a reason.`,
      )
    }
  }
}

if (failures.length) {
  console.error('Purity check failed:\n')
  for (const failure of failures) console.error(`  - ${failure}`)
  console.error(
    '\nA component that reaches for a store, a route or the network still\n' +
      'renders in its story and still renders in the app that has one. The\n' +
      'coupling only shows up in the second app, or the second instance, by\n' +
      'which point the import is load-bearing.',
  )
  process.exit(1)
}

console.log(
  'Purity check passed: components take props and emit events, and reach for ' +
    'nothing else.',
)
