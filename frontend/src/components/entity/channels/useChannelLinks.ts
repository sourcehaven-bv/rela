import { toValue, type MaybeRefOrGetter } from 'vue'
import { useRouter } from 'vue-router'
import { relaBase } from '@/api/base'
import type { ChannelReadableRef } from 'rela-components/components/channel/types'

/**
 * Entity links in an open channel: ref chips and the `a.entity-ref` links
 * renderMessageMarkdown builds. Both carry the page world, and a plain
 * click on either navigates in the app instead of reloading the page. A
 * modified click (new tab, new window) stays the browser's.
 */
export function useChannelLinks(world: MaybeRefOrGetter<string | undefined>) {
  const router = useRouter()

  function entityPath(r: ChannelReadableRef) {
    const w = toValue(world)
    return { path: `/entity/${r.type}/${r.id}`, query: w ? { world: w } : {} }
  }

  /** A same-origin path with the router base, as RlRefChip requires. */
  function entityHref(r: ChannelReadableRef): string {
    return router.resolve(entityPath(r)).href
  }

  function onOpenRef(r: ChannelReadableRef, event: MouseEvent) {
    event.preventDefault()
    void router.push(entityPath(r))
  }

  /** A plain click on an entity link inside a message. */
  function onLogClick(event: MouseEvent) {
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
      return
    }
    const a = (event.target as HTMLElement | null)?.closest<HTMLAnchorElement>('a.entity-ref')
    const href = a?.getAttribute('href')
    const base = relaBase()
    if (!href || !href.startsWith(base)) return
    event.preventDefault()
    void router.push(`/${href.slice(base.length)}`)
  }

  return { entityHref, onOpenRef, onLogClick }
}
