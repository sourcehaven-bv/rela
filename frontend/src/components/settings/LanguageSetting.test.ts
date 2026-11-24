import { describe, it, expect, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import LanguageSetting from './LanguageSetting.vue'
import { resetLanguageState } from '@/i18n/locale'

function optionLabels(wrapper: ReturnType<typeof mount>): string[] {
  return wrapper.findAll('option').map((o) => o.text())
}

afterEach(() => {
  localStorage.clear()
  resetLanguageState()
})

describe('LanguageSetting', () => {
  it('lists every language when the app names none', () => {
    resetLanguageState({ browser: ['en-US'] })
    const labels = optionLabels(mount(LanguageSetting))
    expect(labels).toContain('English')
    expect(labels).toContain('Nederlands')
  })

  it('lists the offered languages in the order the app names them', () => {
    resetLanguageState({ browser: ['en-US'], offered: ['nl', 'en'] })
    const labels = optionLabels(mount(LanguageSetting))
    expect(labels.slice(1)).toEqual(['Nederlands', 'English'])
  })

  it('shows nothing when the app offers a single language', () => {
    resetLanguageState({ browser: ['nl-NL'], offered: ['en'] })
    expect(mount(LanguageSetting).find('[data-testid="language-setting"]').exists()).toBe(false)
  })
})
