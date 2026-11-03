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
export function aboveRelationsIndex(sections: readonly { source?: string }[]): number {
  const i = sections.findIndex((s) => s.source !== 'entry')
  return i === -1 ? sections.length : i
}

/**
 * The key of the above_relations block among the section keys. Section ids
 * are slugs of `[a-z0-9-]`, so a key with a colon cannot collide with one.
 */
export const ABOVE_RELATIONS_KEY = 'channels:above_relations'

/** One item on the page: a view section, or the above_relations block. */
export type PageItem<S> =
  | { key: string; section: S; sectionIndex: number }
  | { key: typeof ABOVE_RELATIONS_KEY; section: null; sectionIndex: -1 }

/**
 * The view sections with the above_relations block among them, at
 * [aboveRelationsIndex]. The page renders them in one keyed list, so when
 * the sections change the block moves with its own key rather than
 * remounting, and keeps an open channel and its draft.
 */
export function withAboveRelationsBlock<S extends { sectionId: string; source?: string }>(
  sections: readonly S[]
): PageItem<S>[] {
  const items: PageItem<S>[] = sections.map((section, sectionIndex) => ({
    key: section.sectionId,
    section,
    sectionIndex,
  }))
  items.splice(aboveRelationsIndex(sections), 0, {
    key: ABOVE_RELATIONS_KEY,
    section: null,
    sectionIndex: -1,
  })
  return items
}
