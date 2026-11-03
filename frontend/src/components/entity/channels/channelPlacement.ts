/**
 * Where the channel types of an entity sit on its page (TKT-VXBE16H).
 *
 * `channels.<type>.placement` in data-entry.yaml moves a channel type from
 * the channels block at the end of the page to a block before the first
 * view section that does not show the entry. Channel types with the same
 * placement share one block.
 */
import type { ChannelPresentation } from '@/types'

/** A channels block on an entity page. `end` is the default block. */
export type ChannelPlacement = 'above_relations' | 'end'

/**
 * The placement of one channel type. A value this app does not know, say
 * from a newer server, keeps the type in the default block.
 */
export function placementOf(p: ChannelPresentation | undefined): ChannelPlacement {
  return p?.placement === 'above_relations' ? 'above_relations' : 'end'
}

/** The channel types, in the given order, that sit in the block at `at`. */
export function typesAt(
  typeNames: readonly string[],
  presentation: (type: string) => ChannelPresentation | undefined,
  at: ChannelPlacement
): string[] {
  return typeNames.filter((name) => placementOf(presentation(name)) === at)
}

/**
 * The index of the view section the above_relations block goes before: the
 * first section that does not show the entry. When every section shows the
 * entry, or there is none, it is the number of sections, which puts the
 * block after the last one.
 */
export function aboveRelationsIndex(sections: readonly { entry?: boolean }[]): number {
  const i = sections.findIndex((s) => !s.entry)
  return i === -1 ? sections.length : i
}
