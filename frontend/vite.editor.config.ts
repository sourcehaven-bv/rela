import { defineConfig, type Plugin } from 'vite'
import { fileURLToPath, URL } from 'node:url'

// Standalone build for the <rela-editor> Custom Element (TKT-5F9V56).
//
// Produces ONE self-contained IIFE served at the reserved per-app path
// /api/v1/_apps/<id>/_rela-editor.js, plus a sibling rela-editor.css served at
// _rela-editor.css. Apps opt in with <script src="_rela-editor.js">. Kept
// separate from the SPA build (and from the tiny bridge _rela.js) so only apps
// that use the editor pay the bundle.
//
// Output goes to a dedicated dir that the Go side embeds (apps_editor.go).
//
// The editor is Milkdown (ProseMirror), the same one the SPA form runs
// (TKT-D2JML7). It ships no webfont at all: the toolbar glyphs are inline SVG
// built from a shared geometry table. The EasyMDE build this replaced carried
// Font Awesome, which needed a third served asset (_rela-editor.woff2), a
// build plugin to stop Vite base64-inlining five font formats, and a second
// plugin to re-point @font-face at the app base so the CSP's `font-src <base>`
// would permit it. All three are gone.

// Re-anchor scales.css's `:root` blocks onto the editor's own shell.
//
// The spacing/radius/typography ramp is NOT part of `_rela.css`, the token
// contract apps are served — `tokens.css` is colour-only and deliberately
// separate. Emitting `scales.css` verbatim would define `--space-*` and
// `--radius-*` on the app's `:root` as a side effect of linking the editor
// stylesheet, so an app author could come to depend on tokens rela never
// promised and a later build could take away. Scoping them to the editor keeps
// the contract exactly as documented while still giving the editor's chrome the
// values it is written against.
//
// The `.dark` variant has to be carried across too: the SDK toggles `dark` on
// the app's <html>, so the selector must stay anchored there rather than on the
// shell.
function scopeScalesToEditor(css: string): string {
  let matched = 0
  const out = css
    .replace(/^:root\.dark\b/gm, () => {
      matched++
      return ':root.dark .rela-editor-shell'
    })
    .replace(/^:root(?!\.)/gm, () => {
      matched++
      return '.rela-editor-shell'
    })
  // A regex over a stylesheet another part of the tree owns is exactly the
  // coupling that rots quietly: reformat `scales.css` so `:root` no longer
  // starts a line, wrap it in `@layer`, or add `:root:not(.dark)`, and this
  // silently stops matching. The only symptom would be that every scale token
  // lands on the APP's `:root` and app authors quietly gain tokens rela never
  // promised. Counting the substitutions turns that into a build failure.
  if (matched < 2) {
    throw new Error(
      `emit-editor-css: expected to re-anchor at least 2 \`:root\` blocks in ` +
        `scales.css but matched ${matched} — its structure changed, and the ` +
        `scale tokens would leak onto the app's :root as an unpromised contract.`
    )
  }
  return out
}

// emitEditorCSS concatenates the editor's stylesheets into a single
// rela-editor.css asset, which rela serves at the app-relative reserved path
// _rela-editor.css.
//
// The CSS is a separate file rather than a string inlined into the IIFE because
// the app CSP has no 'unsafe-inline': a <style> element injected at runtime is
// blocked outright (element in the DOM, .sheet null, editor completely
// unstyled), whereas a <link> is an ordinary resource load the path-scoped
// style-src permits.
//
// Order is load-bearing:
//   1. ProseMirror's own base, table and gap-cursor styles.
//   2. scales.css — the spacing/radius/typography ramp, re-anchored onto the
//      editor's shell (see scopeScalesToEditor). _rela.css carries the COLOUR
//      tokens (it is tokens.css verbatim) but not these, and the editor's
//      chrome is written against both.
//   3. markdown-content.css — the SPA's single source of truth for how a
//      rendered body looks. The writing surface wears `md-body`, so it is
//      styled by the exact rules that will render the text later. The EasyMDE
//      build hand-mirrored a subset of this file and needed a test to catch the
//      drift; concatenating the real thing removes the second copy.
//   4. relaEditorTheme.css LAST, so the editor's own chrome wins.
function emitEditorCSS(): Plugin {
  return {
    name: 'emit-editor-css',
    async generateBundle() {
      const { createRequire } = await import('node:module')
      const require = createRequire(import.meta.url)
      const { readFile } = await import('node:fs/promises')

      const resolvePkg = (spec: string, what: string): string => {
        try {
          return require.resolve(spec)
        } catch {
          throw new Error(
            `emit-editor-css: could not resolve ${what} (${spec}) — is the dependency installed?`
          )
        }
      }
      const local = (rel: string): string => fileURLToPath(new URL(rel, import.meta.url))

      // `@milkdown/kit/prose/**/style/*.css` are RE-EXPORT SHIMS: each is a
      // one-line `@import '@milkdown/prose/...'`. Reading one and concatenating
      // it emits a bare specifier no browser can resolve, so the editor ships
      // with no ProseMirror styling at all and nothing errors. Resolve through
      // `@milkdown/prose` to reach the real declarations.
      const parts = await Promise.all([
        readFile(
          resolvePkg('@milkdown/prose/view/style/prosemirror.css', 'ProseMirror CSS'),
          'utf8'
        ),
        readFile(
          resolvePkg('@milkdown/prose/tables/style/tables.css', 'ProseMirror table CSS'),
          'utf8'
        ),
        readFile(
          resolvePkg('@milkdown/prose/gapcursor/style/gapcursor.css', 'ProseMirror gapcursor CSS'),
          'utf8'
        ),
        readFile(local('./src/styles/scales.css'), 'utf8').then(scopeScalesToEditor),
        readFile(local('./src/styles/markdown-content.css'), 'utf8'),
        readFile(local('./src/app-editor/relaEditorTheme.css'), 'utf8'),
      ])

      const source = parts.join('\n')
      // Fail the build rather than serve a stylesheet the browser will drop. An
      // unresolvable `@import` is silent at runtime: the rule is discarded and
      // the editor renders unstyled, which is the exact failure the shims above
      // would have caused.
      if (/^\s*@import/m.test(source)) {
        throw new Error(
          'emit-editor-css: the concatenated stylesheet contains an @import — ' +
            'a bare specifier cannot be resolved by the browser and the rule is ' +
            'silently dropped. Resolve the real CSS file instead of a re-export shim.'
        )
      }

      assertNoWebfont('rela-editor.css', source)

      this.emitFile({
        type: 'asset',
        fileName: 'rela-editor.css',
        source,
      })
    },
  }
}

// The JS half of the webfont gate.
//
// A separate plugin rather than a second hook on emitEditorCSS: Rollup takes
// ONE `generateBundle` per plugin object, so declaring a second silently
// replaces the first — which stopped the stylesheet being emitted at all, with
// a clean build and no warning.
function assertBundleShipsNoFont(): Plugin {
  return {
    name: 'assert-bundle-ships-no-font',
    generateBundle(_options, bundle) {
      for (const [name, output] of Object.entries(bundle)) {
        if (output.type === 'chunk') assertNoWebfont(name, output.code)
      }
    },
  }
}

// Fail the build if a webfont reached the shipped artifacts.
//
// The editor's toolbar is inline SVG and it is meant to ship no font at all,
// which removed a third served asset (`_rela-editor.woff2`) and the CORS
// exception a sandboxed, null-origin iframe needed to fetch it.
//
// The tests that a font is gone all assert the SERVED PATH 404s, and none of
// them can catch a regression: `assetsInlineLimit` is deliberately infinite so
// the bundle is self-contained, so a font pulled in by any future dependency
// gets base64-inlined into the JS or CSS and never becomes a path at all. It
// would ship, every test would stay green, and the only signal would be the
// artifact quietly growing. This is the assertion that actually pins the claim.
function assertNoWebfont(artifact: string, content: string): void {
  const offenders = [
    ['@font-face', /@font-face/],
    ['an inlined font data: URI', /data:(?:font\/|application\/(?:x-)?font)/],
    ['a font file reference', /url\([^)]*\.(?:woff2?|ttf|otf|eot)/i],
  ] as const
  for (const [what, pattern] of offenders) {
    if (pattern.test(content)) {
      throw new Error(
        `emit-editor-css: ${artifact} contains ${what}. The editor ships no ` +
          `webfont — its glyphs are inline SVG — and an inlined one would not ` +
          `show up as a served path, so nothing else would catch it.`
      )
    }
  }
}

export default defineConfig({
  plugins: [emitEditorCSS(), assertBundleShipsNoFont()],
  // No public/ asset copying — this is a standalone lib build, not the SPA.
  publicDir: false,
  define: {
    // The editor build must not pull in the SPA's E2E test-hook flag.
    __E2E_TEST_HOOKS__: JSON.stringify(false),
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    outDir: '../internal/dataentry/app_editor_dist',
    // Do NOT empty the dir: it contains the committed .gitkeep that keeps the
    // Go glob-embed compiling on a clean checkout (the build artifacts
    // themselves are gitignored). emptyOutDir:true would delete .gitkeep and
    // leave the work tree dirty after `npm run build` (CI "work tree clean"
    // gate). We only emit rela-editor.js + .css, which Vite overwrites in
    // place, so emptying is unnecessary.
    emptyOutDir: false,
    // Inline all CSS/assets into the single JS file so the served
    // _rela-editor.js is fully self-contained (no sibling chunks the app would
    // have to also fetch). The editor imports no stylesheet of its own — the
    // served CSS is emitted by emitEditorCSS above — so this only covers any
    // CSS a dependency pulls in.
    cssCodeSplit: false,
    assetsInlineLimit: Number.MAX_SAFE_INTEGER,
    lib: {
      entry: fileURLToPath(new URL('./src/app-editor/relaEditor.ts', import.meta.url)),
      name: 'RelaEditor',
      formats: ['iife'],
      fileName: () => 'rela-editor.js',
    },
    rollupOptions: {
      output: {
        // Single bundle; no code splitting.
        inlineDynamicImports: true,
      },
    },
  },
})
