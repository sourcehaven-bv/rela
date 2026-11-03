/**
 * The library's message sets by language, and a `Messages` that follows the
 * app's current language.
 *
 * The library does not depend on an i18n framework. An app that has one
 * passes a getter for its current language; each string resolves the set on
 * the call, so it is right after a switch even for text built outside a render
 * (a screen-reader announcement composed in a key handler). When the getter
 * reads reactive state, a render that calls a message re-renders on a switch.
 */
import { DEFAULT_MESSAGES, type Messages } from './useMessages'
import { NL_MESSAGES } from './messages.nl'

export const LOCALE_MESSAGES = { en: DEFAULT_MESSAGES, nl: NL_MESSAGES } as const satisfies Record<
  string,
  Messages
>

export type MessagesLanguage = keyof typeof LOCALE_MESSAGES

function setFor(language: string): Messages {
  return Object.prototype.hasOwnProperty.call(LOCALE_MESSAGES, language)
    ? LOCALE_MESSAGES[language as MessagesLanguage]
    : DEFAULT_MESSAGES
}

/**
 * Messages in whatever language `getLanguage` returns at call time. An
 * unknown language gets English. Pass the result to `provideMessages`.
 */
export function localeMessages(getLanguage: () => string): Messages {
  const out = {} as Record<keyof Messages, unknown>
  for (const key of Object.keys(DEFAULT_MESSAGES) as (keyof Messages)[]) {
    out[key] = (args: never) => (setFor(getLanguage())[key] as (a: never) => string)(args)
  }
  return out as Messages
}
