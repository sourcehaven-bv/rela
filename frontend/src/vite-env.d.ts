/// <reference types="vite/client" />

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
