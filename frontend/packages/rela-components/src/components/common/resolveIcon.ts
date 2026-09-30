/**
 * Turning an author-supplied icon name into a glyph.
 *
 * Sidebar entries are the one surface whose icon is chosen in project config
 * (`data-entry.yaml`) rather than in this library's own code. Everywhere else —
 * buttons, empty states, meta items — the name is written by a developer and
 * `IconName` already makes a wrong one a type error.
 *
 * So this is the boundary, and it is deliberately a LOOKUP in a closed map
 * rather than anything resembling dynamic resolution: a config value must never
 * be able to name something the library did not intend to draw.
 *
 * An unknown name falls back rather than throwing. A stale or hand-edited
 * config still renders a usable sidebar, and the author is told by whatever
 * validates the config on load, which is where a config error belongs.
 */
import { isIconName, type IconName } from './icons'

/**
 * The reserved name meaning "draw nothing".
 *
 * Distinct from an absent name, which means "use whatever this entry's kind
 * implies". Both end up drawing no glyph, but only `none` is a deliberate
 * choice by the author, and only `none` reserves the icon column so a label
 * stays aligned with its icon-bearing siblings.
 *
 * Carried as this literal rather than mapped to an empty string at any layer:
 * an empty string is indistinguishable from a field that was never set.
 */
export const NO_ICON = 'none'

export type ResolvedIcon =
  /** Draw this glyph. */
  | { kind: 'icon'; name: IconName }
  /** Draw nothing, but hold the space. The author asked for no glyph. */
  | { kind: 'none' }
  /** The author named nothing; the caller decides what a bare entry looks like. */
  | { kind: 'unset' }

/**
 * @param value the raw config string, if any
 * @param fallback the glyph to use when a name is given but not recognised
 */
export function resolveIcon(
  value: string | null | undefined,
  fallback: IconName = 'document',
): ResolvedIcon {
  if (value === NO_ICON) return { kind: 'none' }
  if (!value) return { kind: 'unset' }
  if (isIconName(value)) return { kind: 'icon', name: value }
  return { kind: 'icon', name: fallback }
}
