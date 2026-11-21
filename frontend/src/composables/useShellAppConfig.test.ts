import { mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent, nextTick, reactive } from 'vue'
import type { ShellAppSource } from './useShellAppConfig'

const mounted: VueWrapper[] = []

function addMeta(name: string, content: string) {
  const m = document.createElement('meta')
  m.name = name
  m.content = content
  document.head.appendChild(m)
}

/**
 * Loads the language module and the composable fresh, so `locale.ts` reads
 * the boot meta the test wrote, as it does on a real page load.
 */
async function freshModules(browser: string[]) {
  vi.resetModules()
  vi.spyOn(navigator, 'languages', 'get').mockReturnValue(browser)
  const locale = await import('@/i18n/locale')
  const { useShellAppConfig } = await import('./useShellAppConfig')
  return { locale, useShellAppConfig }
}

/** Mounts the composable as App.vue does, over a store-shaped object. */
function mountShell(
  use: (s: ShellAppSource) => { appName: () => string | undefined },
  store: ShellAppSource
) {
  let appName: () => string | undefined = () => undefined
  mounted.push(
    mount(
      defineComponent({
        setup() {
          appName = use(store).appName
          return () => null
        },
      })
    )
  )
  return () => appName()
}

function notLoaded(): ShellAppSource {
  return reactive<ShellAppSource>({ loaded: false, app: { name: 'rela' } })
}

describe('useShellAppConfig (BUG-HZS0DGQ)', () => {
  afterEach(() => {
    while (mounted.length) mounted.pop()!.unmount()
    document.head.querySelectorAll('meta[name^="rela-"]').forEach((m) => m.remove())
    vi.restoreAllMocks()
  })

  it('keeps the shell language and name while the config has not loaded', async () => {
    // The setting-up screen: the server's language, a Dutch browser.
    addMeta('rela-language', 'en')
    addMeta('rela-app-name', 'Sourcehaven Workspace')
    const { locale, useShellAppConfig } = await freshModules(['nl-NL'])

    const appName = mountShell(useShellAppConfig, notLoaded())

    expect(locale.resolvedLanguage.value).toMatchObject({ language: 'en', source: 'project' })
    expect(appName()).toBe('Sourcehaven Workspace')
  })

  it('takes the config once it loads, also a config without a language', async () => {
    addMeta('rela-language', 'en')
    addMeta('rela-app-name', 'Sourcehaven Workspace')
    const { locale, useShellAppConfig } = await freshModules(['nl-NL'])
    const store = notLoaded()
    const appName = mountShell(useShellAppConfig, store)

    store.app = { name: 'Harbourmaster', language: 'nl' }
    store.loaded = true
    await nextTick()
    expect(locale.resolvedLanguage.value).toMatchObject({ language: 'nl', source: 'project' })
    expect(appName()).toBe('Harbourmaster')

    store.app = { name: 'Harbourmaster' }
    await nextTick()
    expect(locale.resolvedLanguage.value.source).toBe('browser')
  })

  it('keeps the config through a reload that unsets loaded', async () => {
    addMeta('rela-language', 'en')
    addMeta('rela-app-name', 'Sourcehaven Workspace')
    const { locale, useShellAppConfig } = await freshModules(['en-US'])
    const store = notLoaded()
    const appName = mountShell(useShellAppConfig, store)
    store.app = { name: 'Harbourmaster', language: 'nl' }
    store.loaded = true
    await nextTick()

    store.loaded = false
    await nextTick()
    expect(locale.resolvedLanguage.value.language).toBe('nl')
    expect(appName()).toBe('Harbourmaster')
  })

  it('falls back to the store name in a shell without boot values', async () => {
    const { useShellAppConfig } = await freshModules(['en-US'])
    expect(mountShell(useShellAppConfig, notLoaded())()).toBe('rela')
  })
})
