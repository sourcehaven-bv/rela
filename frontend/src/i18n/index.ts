/**
 * The SPA's interface text (TKT-ZL1QAO).
 *
 * Components call `const { t } = useI18n()`; code outside a component's setup
 * (stores, pure helpers that build labels) imports `t` from here. Both read the
 * same global locale, so either re-renders on a language switch when called
 * from a render or a computed.
 *
 * Rules for keys:
 * - Keyed by area: `nav.search`, `account.signOut`.
 * - Write every key literally where it is used. The unused-key check in
 *   `guards.test.ts` finds keys by their literal text, so a key built at run
 *   time (`` `language.${code}` ``) would look unused. Map a value to a key with
 *   a literal record instead.
 * - Every key exists in every locale file; `guards.test.ts` checks that too.
 *
 * Operator text (schema labels, list and form titles) is not translated: it is
 * already written in the operator's language.
 */
import { watch } from 'vue'
import { createI18n } from 'vue-i18n'
import en from './locales/en.json'
import { FALLBACK_LANGUAGE, MESSAGES } from './language'
import { resolvedLanguage } from './locale'

export type MessageSchema = typeof en

declare module 'vue-i18n' {
  // Types every `t('...')` key against the English file, so a misspelt key
  // fails `vue-tsc` instead of rendering the key itself.
  // eslint-disable-next-line @typescript-eslint/no-empty-object-type
  export interface DefineLocaleMessage extends MessageSchema {}
}

export const i18n = createI18n({
  legacy: false,
  locale: resolvedLanguage.value.language,
  fallbackLocale: FALLBACK_LANGUAGE,
  messages: MESSAGES,
  // A missing key is caught by guards.test.ts; at run time the fallback to
  // English is the right answer and a console warning per render is noise.
  missingWarn: false,
  fallbackWarn: false,
})

/** `t` for code outside a component's setup. */
export const t = i18n.global.t

/**
 * Keep vue-i18n and `<html lang>` on the resolved language. `<html lang>`
 * takes the formatting locale (`nl-BE`), which also tells screen readers and
 * the browser's hyphenation which variant the text is in.
 */
watch(
  resolvedLanguage,
  ({ language, locale }) => {
    i18n.global.locale.value = language
    if (typeof document !== 'undefined') document.documentElement.lang = locale
  },
  // Synchronous, so a t() call straight after a switch already answers in
  // the new language rather than one tick later.
  { immediate: true, flush: 'sync' }
)

export { intlLocale, setProjectLanguage, setUserLanguage, resolvedLanguage } from './locale'
