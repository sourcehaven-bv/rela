import { ref, watch } from 'vue'
import { setProjectLanguage } from '@/i18n'
import { bootMeta } from '@/i18n/bootMeta'

/** The part of the schema store the app shell reads its identity from. */
export interface ShellAppSource {
  /** Whether `/_config` has loaded; until then `app` is a placeholder. */
  loaded: boolean
  app?: { name?: string; language?: string }
}

/**
 * Feeds the language resolution `app.language`, and returns the app name for
 * the tab title.
 *
 * Until the config first loads, the store's `app` is the placeholder
 * `{ name: 'rela' }`. Reading it then would replace what the server wrote into
 * the shell (`bootMeta`) with "no language" and "rela". On the tenant
 * setting-up screen the config never loads, so the screen followed the browser
 * language (BUG-HZS0DGQ). The shell's values therefore hold until the config
 * has loaded once; from then on the config wins, including a config with no
 * language, and a `reload()` that briefly unsets `loaded` changes nothing.
 */
export function useShellAppConfig(store: ShellAppSource): { appName: () => string | undefined } {
  const configSeen = ref(store.loaded)
  watch(
    () => store.loaded,
    (loaded) => {
      if (loaded) configSeen.value = true
    }
  )
  watch(
    () => ({ seen: configSeen.value, language: store.app?.language }),
    ({ seen, language }) => {
      if (seen) setProjectLanguage(language)
    },
    { immediate: true }
  )
  return {
    appName: () =>
      configSeen.value ? store.app?.name : (bootMeta('rela-app-name') ?? store.app?.name),
  }
}
