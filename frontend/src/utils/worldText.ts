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

const KEYS: (keyof WorldTextVars)[] = ['face', 'bare_face', 'world', 'title']

/**
 * Renders an operator template. Returns '' for an undeclared template, which
 * every caller treats as "render nothing".
 */
export function worldText(template: string | undefined, vars: WorldTextVars): string {
  if (!template) return ''
  let out = template
  for (const key of KEYS) {
    const value = vars[key]
    if (value === undefined) continue
    out = out.split(`{${key}}`).join(value)
  }
  return out
}
