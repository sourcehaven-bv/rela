/**
 * Operator-authored chrome text for worlds, faces and copies (TKT-5SZG2L).
 *
 * The web app has NO sentence of its own for any of the world chrome — the
 * read-only note, the absent note, the projection note, the stand-in badge,
 * the copy toast. Every one it used to have was rela's storage vocabulary
 * ("face", "world", "bare", "default") shown to a reader who never chose those
 * words. So: an operator declares the text in schema.yaml and it is rendered
 * verbatim, or nothing is rendered.
 *
 * The one thing the app does to the text is substitute an allowlisted set of
 * `{placeholders}`. Anything else in braces is left as written — the text is
 * the operator's, and a typo in a placeholder name should show up on screen
 * rather than vanish.
 */
export interface WorldTextVars {
  /** The served face's label. */
  face?: string
  /** The type's bare face label. */
  bare_face?: string
  /** The world's name. */
  world?: string
  /** The entity's display title. */
  title?: string
}

// The allowlist. internal/metamodel.ChromePlaceholders is the same list on
// the Go side, and TestChromePlaceholdersInSyncWithFrontend reads this line
// to pin the two together.
const KEYS: (keyof WorldTextVars)[] = ['face', 'bare_face', 'world', 'title']

const PLACEHOLDER = new RegExp(`\\{(${KEYS.join('|')})\\}`, 'g')

/**
 * Renders an operator template. Returns '' for an undeclared template, which
 * every caller treats as "render nothing".
 *
 * One pass, so a substituted VALUE is never re-scanned for placeholders. A
 * var the caller did not supply (`undefined`) leaves its placeholder as
 * written, like an unknown name — the surface has no such fact. A var
 * supplied as '' substitutes to nothing: the fact exists and is empty (a
 * face with no label), and printing `{face}` there would be a rela word.
 */
export function worldText(template: string | undefined, vars: WorldTextVars): string {
  if (!template) return ''
  return template.replace(PLACEHOLDER, (match, key: keyof WorldTextVars) => vars[key] ?? match)
}
