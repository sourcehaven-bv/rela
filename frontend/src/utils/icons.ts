/** Icon resolution — the ONLY place a config string becomes an icon component.
 *
 * Icon names arrive from project-authored data-entry.yaml (navigation entries,
 * kanban columns/swimlanes). Resolution is a STATIC ALLOWLIST lookup, never
 * dynamic component resolution from the string: a config value must never be
 * able to name an arbitrary component. Unknown names fall back to a default
 * rather than throwing, so a stale or hand-edited config still renders — the
 * server rejects unknown names at load, which is where an author gets told.
 *
 * Icons are Lucide (ISC), imported by name so the bundler tree-shakes the rest.
 * They render as inline SVG with `stroke="currentColor"`, which is the whole
 * point of the migration: the emoji they replace could not take the theme's
 * colour, rendered differently per OS, and sat on an inconsistent baseline.
 *
 * The registry itself (imports + the ICONS map) is GENERATED into
 * ./iconRegistry.generated.ts from internal/dataentryconfig/icondefs, which is
 * also where the Go allowlist and the documentation table come from. This file
 * stays hand-written because the lookup below is the security boundary, and a
 * security boundary should not be something a template emits.
 *
 * Adding an icon: append to the Go table and run `just generate-icons`. Nothing
 * here changes.
 */
import type { Component } from 'vue'
import { ICONS } from './iconRegistry.generated'

export { ICONS }
export * from './iconRegistry.generated'

/** NO_ICON is the reserved name meaning "draw nothing".
 *
 * Distinct from an absent/empty name, which means "use the icon derived from
 * this entry's kind". Both render no glyph on their own, but only NO_ICON is a
 * deliberate choice by the author, and only NO_ICON reserves the icon column so
 * a label stays aligned with its icon-bearing siblings.
 *
 * It is deliberately NOT a member of ICONS: it names no component, and
 * isKnownIcon must keep reporting false for it. Mirrors icondefs.NoIcon in Go. */
export const NO_ICON = 'none'

/** DEFAULT_ICON renders for any name not in the allowlist. */
export const DEFAULT_ICON: Component = ICONS.document

/** iconNames lists every valid name, for tests and error messages. */
export const iconNames = (): string[] => Object.keys(ICONS).sort()

/**
 * hasIcon reports whether a name should render a glyph at all.
 *
 * The single decision point for both surfaces that draw icons — the sidebar and
 * the kanban board. They render the "no glyph" case differently (the sidebar
 * reserves the column to keep labels aligned; a kanban column header has no
 * column to reserve), but they must AGREE on when there is no glyph, or the two
 * drift apart again the next time a case is added.
 */
export function hasIcon(name?: string | null): boolean {
  return !!name && name !== NO_ICON
}

/**
 * resolveIcon returns the component for a config-supplied name.
 *
 * Falls back to DEFAULT_ICON for an unknown or absent name — never throws and
 * never returns undefined, so a caller can bind the result straight to
 * `<component :is>` without a guard.
 *
 * Callers that can render nothing should gate on hasIcon FIRST: passing
 * NO_ICON here yields the fallback glyph, which is the opposite of what the
 * author asked for.
 */
export function resolveIcon(name?: string | null): Component {
  // isKnownIcon, not `ICONS[name] ?? DEFAULT_ICON`: a bare index lookup finds
  // INHERITED Object.prototype members, so a config naming `toString` or
  // `constructor` would yield a function that is not a component and blow up
  // the render. An own-property check is the difference between a fallback
  // icon and a crash.
  if (!isKnownIcon(name)) return DEFAULT_ICON
  return ICONS[name]
}

/** isKnownIcon reports whether a name resolves to a real icon (not the
 * fallback). A type predicate so callers can index ICONS afterwards. Used by
 * tests; the server is what validates author input.
 *
 * Reports false for NO_ICON, which is a valid thing to write in config but not
 * a glyph. */
export function isKnownIcon(name?: string | null): name is string {
  // Not Object.hasOwn — the project's TS lib target predates it.
  return !!name && Object.prototype.hasOwnProperty.call(ICONS, name)
}
