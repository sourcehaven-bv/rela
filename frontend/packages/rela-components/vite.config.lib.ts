/*
 * The library build, kept apart from `vite.config.ts`.
 *
 * That config builds the example app and carries the Storybook/Vitest wiring,
 * which a consumer's bundle has no business seeing. Two configs rather than one
 * with a mode switch, because the two builds share almost nothing: this one has
 * an entry, externals and no HTML.
 *
 * Types are emitted separately by `vue-tsc` (see the `build:types` script)
 * rather than by a dts plugin, so the declarations come from the same compiler
 * that typechecks the source and there is no extra dependency to keep in step.
 */
import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue()],
  /*
   * No `public/` copy. That directory is the example app's favicon and icon
   * sheet, which have no business in a component library's package.
   */
  publicDir: false,
  build: {
    outDir: 'dist/lib',
    /*
     * Left alone deliberately, and the `build:lib` script clears the directory
     * before it runs instead. Vite empties `outDir` as the bundle starts, which
     * is after `vue-tsc` has written the declarations there — so emptying here
     * would delete the types the same script just produced.
     */
    emptyOutDir: false,
    lib: {
      entry: fileURLToPath(new URL('./src/index.ts', import.meta.url)),
      formats: ['es'],
      fileName: 'index',
    },
    rollupOptions: {
      /*
       * Vue and the editor kit stay external. Vue for the usual reason: two
       * copies give two reactivity systems. `@milkdown/kit` because it is
       * already a peer dependency, and for a sharper reason — a second copy
       * would hand the app a different ProseMirror `Schema` class, and the
       * app's own nodes would be rejected by the document the editor builds.
       */
      external: [
        'vue',
        /^@milkdown\//,
        /^@atlaskit\//,
        '@floating-ui/vue',
      ],
      output: {
        /*
         * CSS lands beside the JS as `index.css`. Components use scoped CSS and
         * the token files are plain CSS, so a consumer imports one stylesheet
         * and needs no CSS tooling of their own — which is the promise the
         * README already makes.
         */
        assetFileNames: 'index[extname]',
      },
    },
  },
})
