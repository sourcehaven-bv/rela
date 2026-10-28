import { isIconName, type IconName } from 'rela-components/components/common/icons'
import { t } from '@/i18n'

/**
 * The glyph for a pile's icon name. The server validates icons against its
 * own allowlist; a name this build's registry cannot draw falls back to the
 * generic pile glyph rather than rendering blank.
 */
export function pileIcon(name: string): IconName {
  return isIconName(name) ? name : 'layers'
}

/**
 * Authored labels for the server's pile icon allowlist (internal/piles
 * `Icons`). An icon without one is labelled by its name as served.
 */
const PILE_ICON_LABELS = {
  layers: 'piles.icon.layers',
  star: 'piles.icon.star',
  flag: 'piles.icon.flag',
  bookmark: 'piles.icon.bookmark',
  inbox: 'piles.icon.inbox',
  folder: 'piles.icon.folder',
  tag: 'piles.icon.tag',
  heart: 'piles.icon.heart',
  clock: 'piles.icon.clock',
  target: 'piles.icon.target',
} as const

type PileIconKey = (typeof PILE_ICON_LABELS)[keyof typeof PILE_ICON_LABELS]

/** The label for a pile icon in the icon picker. */
export function pileIconLabel(name: string): string {
  const key = (PILE_ICON_LABELS as Record<string, PileIconKey | undefined>)[name]
  return key ? t(key) : name
}
