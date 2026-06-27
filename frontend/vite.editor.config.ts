import { defineConfig, type Plugin } from 'vite'
import { fileURLToPath, URL } from 'node:url'

// Strip Font Awesome's bundled @font-face rule (which lists eot/ttf/woff/woff2/
// svg sources) from the inlined FA CSS. Without this, Vite base64-inlines all
// five fallback font files into the bundle (~1.5MB of waste). We supply our own
// single woff2 @font-face via relaEditorFont.css pointing at the reserved app
// path, so FA's own @font-face is dead weight. Runs before Vite's CSS asset
// handling so the url()s never get inlined.
function stripFontAwesomeFontFace(): Plugin {
  return {
    name: 'strip-fontawesome-fontface',
    enforce: 'pre',
    transform(code, id) {
      // Match the FA stylesheet whether imported plain or with ?inline (the
      // query suffix means .endsWith('.css') won't match — check includes).
      if (!id.includes('font-awesome') || !id.includes('.css')) return null
      // FA ships exactly one @font-face{...} block listing eot/ttf/woff/woff2/
      // svg sources; remove it so Vite doesn't base64-inline all five. The
      // [^}]* match relies on FA's block having no nested braces (true for 4.7).
      const stripped = code.replace(/@font-face\s*\{[^}]*\}/g, '')
      // Fail loud if nothing was stripped: a silent no-op (FA restructured its
      // CSS, or this matched the wrong module) would ship ~1.5MB of base64 fonts
      // unnoticed. The override @font-face in relaEditorFont.css is what we keep.
      if (stripped === code) {
        throw new Error(
          'strip-fontawesome-fontface: expected an @font-face block in ' +
            id +
            ' but found none — Font Awesome CSS structure changed; verify the strip.',
        )
      }
      return { code: stripped, map: null }
    },
  }
}

// Standalone build for the <rela-editor> Custom Element (TKT-5F9V56).
//
// Produces ONE self-contained IIFE (JS with CSS inlined) served at the reserved
// per-app path /api/v1/_apps/<id>/_rela-editor.js. Apps opt in with
// <script src="_rela-editor.js">. Kept separate from the SPA build (and from
// the tiny bridge _rela.js) so only apps that use the editor pay the bundle.
//
// Output goes to a dedicated dir that the Go side embeds (apps_editor.go).
// The Font Awesome webfont is served separately under the same app base as
// _rela-editor.woff2; the bundled @font-face is overridden (relaEditorFont.css)
// to point there, so the app CSP's `font-src <base>` permits it with no widening.
// Emit the Font Awesome glyph webfont (woff2 only) alongside the bundle, with a
// stable name (rela-editor.woff2). The Go side serves it at the reserved app
// path _rela-editor.woff2 that the bundle's @font-face points at. Reading from
// node_modules keeps it in lockstep with the font-awesome version the bundle
// was built against.
function emitEditorFont(): Plugin {
  return {
    name: 'emit-editor-font',
    async generateBundle() {
      // Resolve via package resolution rather than a hardcoded ./node_modules
      // path, so it survives hoisting / pnpm / monorepo layouts. Fail loudly
      // with a clear message if the font can't be found.
      const { createRequire } = await import('node:module')
      const require = createRequire(import.meta.url)
      let fontPath: string
      try {
        fontPath = require.resolve('font-awesome/fonts/fontawesome-webfont.woff2')
      } catch {
        throw new Error(
          'emit-editor-font: could not resolve font-awesome/fonts/fontawesome-webfont.woff2 — ' +
            'is the font-awesome dependency installed?',
        )
      }
      const { readFile } = await import('node:fs/promises')
      this.emitFile({
        type: 'asset',
        fileName: 'rela-editor.woff2',
        source: await readFile(fontPath),
      })
    },
  }
}

export default defineConfig({
  plugins: [stripFontAwesomeFontFace(), emitEditorFont()],
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
    emptyOutDir: true,
    // Inline all CSS/assets into the single JS file so the served
    // _rela-editor.js is fully self-contained (no sibling chunks the app
    // would have to also fetch). The FA font is the one intentional
    // exception — it's served separately under the app base.
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
