/// <reference types="vite/client" />

// Compile-time flag injected by vite `define` (see vite.config.ts). True
// only for the E2E dev-mode build (`npm run build:e2e`); false for the
// production build, where references to it are tree-shaken away. Gates
// test-only hooks so they never ship in production bundles (issue #890).
declare const __E2E_TEST_HOOKS__: boolean

// The SPA build id (see relaBuildId in vite.config.ts); '' on the dev server
// and in unit tests. Sent when joining a co-editing room.
declare const __RELA_BUILD_ID__: string

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<object, object, unknown>
  export default component
}

// slim-select ships a bare CSS export at `slim-select/styles` with no
// type declarations. TypeScript 6 rejects side-effect imports that
// lack a module declaration; declare it as a side-effect-only module
// so `import 'slim-select/styles'` compiles.
declare module 'slim-select/styles'
