import { computed, nextTick } from 'vue'
import { i18n, t } from './index'
import {
  LANGUAGE_STORAGE_KEY,
  automaticLanguage,
  intlLocale,
  resetLanguageState,
  resolvedLanguage,
  setProjectLanguage,
  setUserLanguage,
} from './locale'
import { formatAmount, formatDate } from '@/utils/format'

describe('interface language state', () => {
  afterEach(() => localStorage.clear())

  it('follows the project default until the user chooses', async () => {
    resetLanguageState({ browser: ['en-US'] })
    setProjectLanguage('nl')
    expect(resolvedLanguage.value.language).toBe('nl')

    setUserLanguage('en')
    expect(resolvedLanguage.value).toMatchObject({
      language: 'en',
      locale: 'en-US',
      source: 'user',
    })
    expect(localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBe('en')
    // "Automatic" still names what the choice would fall back to.
    expect(automaticLanguage.value.language).toBe('nl')

    setUserLanguage(null)
    expect(localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBeNull()
    expect(resolvedLanguage.value.language).toBe('nl')
  })

  it('treats an empty project language as unset', () => {
    resetLanguageState({ browser: ['nl-NL'] })
    setProjectLanguage('')
    expect(resolvedLanguage.value.source).toBe('browser')
  })

  it('keeps vue-i18n and <html lang> on the resolved language', async () => {
    resetLanguageState({ project: 'nl-BE', browser: ['en-US'] })
    await nextTick()
    expect(i18n.global.locale.value).toBe('nl')
    expect(document.documentElement.lang).toBe('nl-BE')
    expect(t('account.signOut')).toBe('Afmelden')

    resetLanguageState({ browser: ['en-GB'] })
    await nextTick()
    expect(i18n.global.locale.value).toBe('en')
    expect(document.documentElement.lang).toBe('en-GB')
    expect(t('account.signOut')).toBe('Sign out')
  })

  it('formats dates and numbers in the interface locale, and re-runs computeds on a switch', () => {
    resetLanguageState({ user: 'nl', browser: ['en-US'] })
    const date = computed(() => formatDate('2024-01-15'))
    expect(intlLocale()).toBe('nl')
    expect(date.value).toBe('15 jan 2024')
    expect(formatAmount(3500, 'EUR')).toMatch(/^€\s3\.500$/)

    setUserLanguage('en')
    expect(date.value).toBe('Jan 15, 2024')
  })

  it('pluralises', () => {
    resetLanguageState({ user: 'nl' })
    expect(t('git.changes', { n: 1 }, 1)).toBe('1 wijziging')
    expect(t('git.changes', { n: 3 }, 3)).toBe('3 wijzigingen')
  })
})
