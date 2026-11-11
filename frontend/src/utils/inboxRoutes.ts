import { channelKey } from '@/utils/channelKey'
import type { InboxChannel } from '@/types/channels'

/**
 * The row identity and SPA route of an inbox channel, shared by the inbox
 * page and the browser notifications (TKT-48S88RB), so a notification opens
 * exactly what the inbox row links to.
 */

/** The channel's identity across inbox reads: `<type>/<anchor>/<type>/<name>`. */
export function inboxRowKey(c: InboxChannel): string {
  return `${c.entity_type}/${c.anchor}/${channelKey(c.type, c.name)}`
}

/**
 * The SPA path of the channel on its anchor's page, before the space prefix
 * (`space.href`). With a seq the page also reveals that message.
 */
export function inboxRoute(c: InboxChannel, seq?: number): string {
  return (
    `/entity/${c.entity_type}/${encodeURIComponent(c.anchor)}` +
    `?channel=${encodeURIComponent(channelKey(c.type, c.name))}` +
    (seq !== undefined ? `&seq=${seq}` : '')
  )
}
