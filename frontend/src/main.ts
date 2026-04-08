import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'

const app = createApp(App)

// Global component-error handler. Without this, errors thrown inside a
// Vue lifecycle hook, template render, or watcher are logged by Vue with
// minimal context. With it, we get component name + hook name + stack,
// which makes console errors in the stress harness attributable to a
// specific component. See frontend/stress/fuzzRunner.ts and BUG-6C3V.
app.config.errorHandler = (err, instance, info) => {
  const component =
    (instance as { $options?: { __name?: string; name?: string } } | null)?.$options?.__name ??
    (instance as { $options?: { __name?: string; name?: string } } | null)?.$options?.name ??
    '<anonymous>'
  console.error(
    `[vue-error] component=${component} hook=${info} name=${(err as Error)?.name} ` +
      `msg=${(err as Error)?.message}\n${(err as Error)?.stack}`,
  )
}

// Catch unhandled Promise rejections that escape component try/catch.
// These show up in the stress harness as bare "Error" console logs with
// no caller; having them attributed at least tells us a rejection path
// is missing its await-or-catch. See BUG-6C3V and the fuzz loop.
window.addEventListener('unhandledrejection', (ev) => {
  const reason = ev.reason as { name?: string; message?: string; stack?: string; code?: string }
  console.error(
    `[unhandledrejection] name=${reason?.name} code=${reason?.code} ` +
      `msg=${reason?.message}\n${reason?.stack ?? '(no stack)'}`,
  )
})

// Chunk-load recovery via reload.
//
// When Vite fails to load a code-split chunk (preloadError, dynamic
// import reject, "Couldn't resolve component"), the canonical recovery
// is a full page reload to refetch current assets. See
// https://vite.dev/guide/build.html#load-error-handling.
//
// We rate-limit the reload to at most once per 10 seconds using
// sessionStorage. Without this, a chunk that remains broken after the
// reload — or a preloadError that fires during the reload's own
// bootstrap — triggers an infinite reload loop. The user sees a
// browser tab that reloads forever.
function triggerChunkReload(reason: string, msg: string): void {
  try {
    const last = Number(sessionStorage.getItem('rela:lastChunkReload') ?? '0')
    const now = Date.now()
    if (now - last < 10_000) {
      console.warn(
        `[${reason}] chunk-load failure, already reloaded recently — not reloading:`,
        msg,
      )
      return
    }
    sessionStorage.setItem('rela:lastChunkReload', String(now))
  } catch {
    /* sessionStorage unavailable — fall through and reload anyway */
  }
  console.warn(`[${reason}] chunk-load failure, reloading:`, msg)
  window.location.reload()
}

window.addEventListener('vite:preloadError', (event) => {
  const ev = event as Event & { payload?: Error }
  event.preventDefault()
  triggerChunkReload('vite:preloadError', ev.payload?.message ?? String(ev.payload))
})

// Catch uncaught errors at the window level. vue-router's
// loadRouteLocation can reject with "Couldn't resolve component" when
// a dynamic import resolves but the module's default export is
// missing — typically a symptom of a half-fetched chunk during rapid
// navigation. Recovery requires a reload. This handler complements
// router.onError (which runs earlier) by catching errors that escape
// the router's own error pipeline.
window.addEventListener('error', (ev) => {
  const msg = ev.message ?? ''
  const errMsg = ev.error instanceof Error ? ev.error.message : ''
  const combined = msg || errMsg
  if (
    combined.includes("Couldn't resolve component") ||
    combined.includes('Failed to fetch dynamically imported module') ||
    combined.includes('Unable to preload CSS')
  ) {
    ev.preventDefault()
    triggerChunkReload('window-error', combined)
  }
})

app.use(createPinia())
app.use(router)

app.mount('#app')
