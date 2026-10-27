/**
 * The proving slice in Dutch (TKT-ZL1QAO): the sidebar groups, the footer, the
 * Settings language card, and the component library's own chrome text through
 * the `provideMessages` seam.
 */
import { defineComponent, h, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { provideMessages, useMessages } from 'rela-components/composables/useMessages'
import { localeMessages } from 'rela-components/composables/localeMessages'
import RlThemeToggle from 'rela-components/components/layout/RlThemeToggle.vue'
import { pilesNavGroup, toNavGroups } from '@/components/common/sidebarNav'
import SidebarFooter from '@/components/common/SidebarFooter.vue'
import LanguageSetting from '@/components/settings/LanguageSetting.vue'
import {
  LANGUAGE_STORAGE_KEY,
  resetLanguageState,
  resolvedLanguage,
  setUserLanguage,
} from './locale'

vi.mock('@/utils/markdown', () => ({ renderMarkdown: (s: string) => s }))

describe('sidebar labels', () => {
  it('follows the interface language', () => {
    resetLanguageState({ user: 'nl' })
    expect(pilesNavGroup([])).toMatchObject({ label: 'Stapels', addLabel: 'Nieuwe stapel' })
    const [group] = toNavGroups(
      [
        {
          name: 'Work',
          items: [],
          items_list: 'tickets',
          items_create: { label: 'ticket' },
        } as never,
      ],
      'a' as never
    )
    expect(group.addLabel).toBe('Nieuw: ticket')

    setUserLanguage('en')
    expect(pilesNavGroup([])).toMatchObject({ label: 'Piles', addLabel: 'New pile' })
  })
})

describe('SidebarFooter', () => {
  it('renders its links in Dutch', async () => {
    resetLanguageState({ user: 'nl' })
    setActivePinia(createPinia())
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div/>' } }],
    })
    await router.push('/')
    const w = mount(SidebarFooter, { global: { plugins: [router] } })
    expect(w.text()).toContain('Instellingen')
    expect(w.text()).toContain('Sneltoetsen')
    expect(w.text()).not.toContain('Settings')
    w.unmount()
  })
})

describe('LanguageSetting', () => {
  afterEach(() => localStorage.clear())

  it('offers Automatic with the language it resolves to, and stores a choice', async () => {
    resetLanguageState({ project: 'nl', browser: ['en-US'] })
    const w = mount(LanguageSetting)
    expect(w.text()).toContain('Taal')
    const options = w.findAll('option').map((o) => [o.attributes('value'), o.text()])
    expect(options).toEqual([
      ['', 'Automatisch (Nederlands)'],
      ['en', 'English'],
      ['nl', 'Nederlands'],
    ])

    await w.find('select').setValue('en')
    expect(localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBe('en')
    expect(resolvedLanguage.value.language).toBe('en')
    await nextTick()
    expect(w.text()).toContain('Language')

    await w.find('select').setValue('')
    expect(localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBeNull()
    expect(resolvedLanguage.value.language).toBe('nl')
    w.unmount()
  })
})

describe('component library messages', () => {
  it('resolve the language per call, so a switch needs no re-provide', () => {
    let language = 'en'
    const messages = localeMessages(() => language)
    expect(messages.chromeText({ key: 'collapseSidebar' })).toBe('Collapse sidebar')
    language = 'nl'
    expect(messages.chromeText({ key: 'collapseSidebar' })).toBe('Zijbalk inklappen')
    language = 'fr'
    expect(messages.chromeText({ key: 'collapseSidebar' })).toBe('Collapse sidebar')
  })

  it('reach library components through provideMessages', async () => {
    resetLanguageState({ user: 'nl' })
    const Host = defineComponent({
      setup() {
        provideMessages(localeMessages(() => resolvedLanguage.value.language))
        useMessages()
        return () => h(RlThemeToggle, { modelValue: 'system' })
      },
    })
    const w = mount(Host)
    expect(w.text()).toContain('Licht')
    setUserLanguage('en')
    await nextTick()
    expect(w.text()).toContain('Light')
    w.unmount()
  })
})
