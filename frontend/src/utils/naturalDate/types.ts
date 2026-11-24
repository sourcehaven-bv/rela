// A vocabulary pack holds the words of one language. The parser in parse.ts
// knows only phrase shapes; every word comes from a pack, so a new language is
// a new pack and no parser change.
//
// All words are written lowercase and without diacritics, because the parser
// folds input the same way before it matches. A word may hold spaces, such as
// `il y a`; the parser matches it as one token. Modifier and direction words
// may stand before or after what they modify (`next friday`, `vendredi
// prochain`; `3 days ago`, `vor 3 tagen`), so word order needs no code.
export interface DatePack {
  /** Language code the pack is registered under, such as `en`. */
  code: string
  /** Whole phrases that mean a day offset from today, such as `tomorrow: 1`. */
  relativeDays: Record<string, number>
  /** Seven lists of names, Monday first. */
  weekdays: string[][]
  /** Twelve lists of names, January first. */
  months: string[][]
  /** Words beside a weekday or unit: `this` is on or after today. */
  this: string[]
  /** `next` is strictly after today, or one unit ahead. */
  next: string[]
  /** `last` is strictly before today, or one unit back. */
  last: string[]
  /** Words beside `N unit` that mean ahead, such as `in`. */
  in: string[]
  /** Words beside `N unit` that mean back, such as `ago`. */
  ago: string[]
  /** Singular and plural names per unit. */
  units: Record<'day' | 'week' | 'month' | 'year', string[]>
  /** Number words, such as `one: 1`. Digits always work. */
  numbers?: Record<string, number>
  /** Suffixes allowed after a day number, such as `th`. */
  ordinals?: string[]
  /** Words dropped before matching, such as articles (`the`, `la`). */
  fillers?: string[]
  /**
   * Splits folded input into tokens. The default splits on whitespace; a
   * language written without spaces supplies its own.
   */
  tokenize?: (folded: string) => string[]
}
