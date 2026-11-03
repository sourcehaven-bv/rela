/**
 * Which interface language to show, and which locale to format with.
 *
 * Pure: the inputs are passed in, so the order of precedence can be tested
 * without a browser. `locale.ts` feeds it the live values.
 */
import en from './locales/en.json'
import nl from './locales/nl.json'

/**
 * The message files, by base language. The supported languages are the keys
 * of this record, so a language is selectable exactly when it has a file.
 * `TestSupportedUILanguagesMatchLocaleFiles` (Go) pins the server's list of
 * valid `app.language` values to the same files.
 */
export const MESSAGES = { en, nl } as const

export type UiLanguage = keyof typeof MESSAGES

export const SUPPORTED_LANGUAGES = Object.keys(MESSAGES) as UiLanguage[]

/** The language every key is written in first, and the last fallback. */
export const FALLBACK_LANGUAGE: UiLanguage = 'en'

export function isUiLanguage(value: unknown): value is UiLanguage {
  return typeof value === 'string' && (SUPPORTED_LANGUAGES as string[]).includes(value)
}

export interface LanguageInputs {
  /** The user's own choice from the Settings page, if any. */
  user?: string | null
  /** The project default, `app.language` in data-entry.yaml. */
  project?: string | null
  /** The browser's preferred languages, most preferred first. */
  browser: readonly string[]
}

export interface ResolvedLanguage {
  /** Which message file to use. */
  language: UiLanguage
  /** The BCP 47 tag to format dates and numbers with, and to put on `<html lang>`. */
  locale: string
  /** Which input decided the language. */
  source: 'user' | 'project' | 'browser' | 'fallback'
}

/** A canonical tag, or undefined when the input is not a valid BCP 47 tag. */
function canonical(tag: string): string | undefined {
  try {
    return Intl.getCanonicalLocales(tag)[0]
  } catch {
    return undefined
  }
}

function baseOf(tag: string): string {
  return tag.split('-')[0].toLowerCase()
}

function hasRegionOrScript(tag: string): boolean {
  return tag.includes('-')
}

/**
 * First match wins: the user's choice, the project default, the browser's
 * languages in order, then English. A tag matches when its base language has
 * a message file, so `nl-BE` selects Dutch.
 *
 * The formatting locale keeps the winning tag when it names a region
 * (`nl-BE`). Otherwise it borrows the first browser tag in the same language,
 * so a British browser with English chosen still gets day-month dates and a
 * 24-hour clock. Failing both, it is the bare language.
 *
 * When nothing matched and the text falls back to English, formatting follows
 * the browser's first language, as it did before the interface had languages:
 * a German browser keeps `15.01.2024` and `3.500 €`. Month and weekday names
 * then come out in that language too, which beats a date order the reader
 * does not use.
 */
export function resolveLanguage(inputs: LanguageInputs): ResolvedLanguage {
  const browser = inputs.browser.map(canonical).filter((t): t is string => !!t)

  const candidates: [string | null | undefined, ResolvedLanguage['source']][] = [
    [inputs.user, 'user'],
    [inputs.project, 'project'],
    ...browser.map((tag): [string, ResolvedLanguage['source']] => [tag, 'browser']),
  ]

  for (const [raw, source] of candidates) {
    if (!raw) continue
    const tag = canonical(raw)
    if (!tag) continue
    const base = baseOf(tag)
    if (!isUiLanguage(base)) continue
    return { language: base, locale: formattingLocale(tag, base, browser), source }
  }
  return {
    language: FALLBACK_LANGUAGE,
    locale: browser[0] ?? FALLBACK_LANGUAGE,
    source: 'fallback',
  }
}

function formattingLocale(tag: string, base: UiLanguage, browser: readonly string[]): string {
  if (hasRegionOrScript(tag)) return tag
  return browser.find((b) => baseOf(b) === base && hasRegionOrScript(b)) ?? base
}
