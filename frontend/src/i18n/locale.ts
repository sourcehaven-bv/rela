/**
 * The live inputs to language resolution, and the resolved answer.
 *
 * Kept free of vue-i18n so formatting helpers (`utils/format.ts`) can read the
 * formatting locale without importing the message machinery. The answer is a
 * computed over refs, so a computed that formats a date re-runs when the
 * language changes.
 */
import { computed, ref } from 'vue'
import { isUiLanguage, resolveLanguage, type ResolvedLanguage, type UiLanguage } from './language'

/**
 * The localStorage key of the user's choice. Stored per browser, like the
 * theme and the display time zone on the same Settings page. The server-side
 * `_settings` values are shared by every user, so they are not a place for a
 * personal preference.
 */
export const LANGUAGE_STORAGE_KEY = 'language'

function readStoredLanguage(): UiLanguage | null {
  try {
    const stored = localStorage.getItem(LANGUAGE_STORAGE_KEY)
    // A value from a removed language, or a hand-edited one, is ignored
    // rather than wedging the app in a language it cannot show.
    return isUiLanguage(stored) ? stored : null
  } catch {
    return null
  }
}

function browserLanguages(): readonly string[] {
  if (typeof navigator === 'undefined') return []
  if (navigator.languages?.length) return navigator.languages
  return navigator.language ? [navigator.language] : []
}

const userLanguage = ref<UiLanguage | null>(readStoredLanguage())
const projectLanguage = ref<string | null>(null)
const browser = ref<readonly string[]>(browserLanguages())

/** The language and formatting locale in effect now. */
export const resolvedLanguage = computed<ResolvedLanguage>(() =>
  resolveLanguage({
    user: userLanguage.value,
    project: projectLanguage.value,
    browser: browser.value,
  })
)

/** The user's own choice, or null for "automatic". */
export const chosenLanguage = computed(() => userLanguage.value)

/**
 * The BCP 47 tag dates and numbers are formatted with. Reading it inside a
 * computed or a render makes that computed follow language changes.
 */
export function intlLocale(): string {
  return resolvedLanguage.value.locale
}

/** Set or clear (null) the user's choice, and persist it for this browser. */
export function setUserLanguage(language: UiLanguage | null): void {
  userLanguage.value = language
  try {
    if (language) localStorage.setItem(LANGUAGE_STORAGE_KEY, language)
    else localStorage.removeItem(LANGUAGE_STORAGE_KEY)
  } catch {
    // Private mode or a full quota: the choice holds for this page only.
  }
}

/** Set the project default from `app.language`; empty or absent clears it. */
export function setProjectLanguage(tag: string | null | undefined): void {
  projectLanguage.value = tag || null
}

/**
 * What the language would be without the user's choice: the answer shown
 * next to "Automatic" on the Settings page.
 */
export const automaticLanguage = computed(() =>
  resolveLanguage({ project: projectLanguage.value, browser: browser.value })
)

/** Test hook: reset every input to a known state. */
export function resetLanguageState(
  inputs: { user?: UiLanguage | null; project?: string | null; browser?: readonly string[] } = {}
): void {
  userLanguage.value = inputs.user ?? null
  projectLanguage.value = inputs.project ?? null
  browser.value = inputs.browser ?? ['en-US']
}
