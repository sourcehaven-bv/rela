import { isIconName, type IconName } from 'rela-components/components/common/icons'

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
const PILE_ICON_LABELS: Record<string, string> = {
  layers: 'Layers',
  star: 'Star',
  flag: 'Flag',
  bookmark: 'Bookmark',
  inbox: 'Inbox',
  folder: 'Folder',
  tag: 'Tag',
  heart: 'Heart',
  clock: 'Clock',
  target: 'Target',
}

/** The label for a pile icon in the icon picker. */
export function pileIconLabel(name: string): string {
  return PILE_ICON_LABELS[name] ?? name
}
