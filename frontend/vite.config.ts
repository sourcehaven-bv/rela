import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import type { Plugin } from 'vite'
import { wrapCss } from './relaCssLayer'
import { fileURLToPath, URL } from 'node:url'
import { randomBytes } from 'node:crypto'

// Get API base URL from environment variable or default to localhost:8080
// For e2e tests, VITE_API_BASE is set by global-setup.ts
const apiBase = process.env.VITE_API_BASE || 'http://localhost:8080'


// Wraps every emitted stylesheet in `@layer rela` so an operator's unlayered
// custom.css wins the cascade. The wrap logic itself lives in
// relaCssLayer.ts (unit-tested via relaCssLayer.test.ts); this is the Vite binding.
//
// Build-only: generateBundle is a Rollup build hook, so `npm run dev` has no
// layer. Verify cascade-sensitive changes against `npm run build`.
function relaCssLayer(): Plugin {
  return {
    name: 'rela-css-layer',
    enforce: 'post',
    generateBundle(_options, bundle) {
      for (const [fileName, chunk] of Object.entries(bundle)) {
        if (chunk.type !== 'asset' || !fileName.endsWith('.css')) continue
        const wrapped = wrapCss(String(chunk.source))
        if (wrapped !== null) chunk.source = wrapped
      }
    },
  }
}

// Writes the build id next to the bundle. rela-server embeds the file and
// closes a co-editing connection from another build with code 4409, because
// two builds may serialize the same document differently (TKT-CJL604).
function relaBuildId(id: string): Plugin {
  return {
    name: 'rela-build-id',
    apply: 'build',
    generateBundle() {
      this.emitFile({ type: 'asset', fileName: 'build-id.txt', source: id + '\n' })
    },
  }
}

export default defineConfig(({ mode, command }) => {
  // Log at config evaluation time so we can see what port is being used
  console.error(`[vite.config] API proxy target: ${apiBase}`)

  // __E2E_TEST_HOOKS__ gates test-only knobs (e.g. the backtick-autocomplete
  // delay override) so they are tree-shaken out of production bundles but
  // compiled in for the E2E suite. It is true only for the dev-mode build
  // (`vite build --mode development`, i.e. `npm run build:e2e`); the default
  // production `vite build` runs in 'production' mode, leaving it false.
  // import.meta.env.DEV can't be used: it is false for ANY `vite build`
  // regardless of --mode, so it would strip the hooks even from the E2E
  // bundle. See issue #890.
  const e2eTestHooks = mode === 'development'

  // RELA_SEQTRACE=1 builds the SPA for `just seqtrace-e2e`: unminified, with
  // source maps, so `seqtrace stitch` can map the stack of each API request
  // back to the src/ module that sent it. 'hidden' writes the maps without
  // a sourceMappingURL comment; stitch reads them from disk.
  const seqtrace = process.env.RELA_SEQTRACE === '1'

  // The dev server has no build id; the relay skips the check for an empty one.
  const buildId = command === 'build' ? randomBytes(8).toString('hex') : ''

  return {
    plugins: [
      vue({
        template: {
          compilerOptions: {
            // <rela-slot> and <rela-editor> are native custom elements, not Vue
            // components. Without this, an unregistered <rela-slot> logs
            // "Failed to resolve component" on every render. This only affects
            // Vue's component RESOLUTION — relaEditor.ts's
            // customElements.define() is untouched.
            isCustomElement: (tag) => tag.startsWith('rela-'),
          },
        },
      }),
      relaCssLayer(),
      relaBuildId(buildId),
    ],
    base: '/',
    define: {
      __E2E_TEST_HOOKS__: JSON.stringify(e2eTestHooks),
      __RELA_BUILD_ID__: JSON.stringify(buildId),
      // vue-i18n build flags: composition API only, so the legacy API is
      // tree-shaken; install the global $t and directives; no devtools in production.
      __VUE_I18N_FULL_INSTALL__: 'true',
      __VUE_I18N_LEGACY_API__: 'false',
      __INTLIFY_PROD_DEVTOOLS__: 'false',
    },
    build: {
      outDir: '../internal/dataentry/static/v2',
      emptyOutDir: true,
      sourcemap: seqtrace ? 'hidden' : false,
      minify: !seqtrace,
    },
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      port: 5173,
      proxy: {
        '/api': {
          target: apiBase,
          changeOrigin: true,
          // Co-editing runs over a WebSocket at /api/v1/_collab/.
          ws: true,
        },
      },
    },
  }
})
