import type { NavItemStatus } from 'rela-components/types'
import type { InboxSummary } from '@/types/channels'

/** The plural lookup inboxStatus needs; vue-i18n's `t(key, n)` fits it. */
export type InboxPlural = (key: string, n: number) => string

/**
 * The sidebar marker of the Inbox item (TKT-9YRRM5).
 *
 * The badge counts what needs the user: waiting sessions plus unread
 * mentions and replies. Its tone is `warning` while a session waits, else
 * `new`. Unread channels alone show a dot without a count (tone `info`), so
 * chat in many rooms does not keep a number lit. Nothing at all shows no
 * marker.
 */
export function inboxStatus(s: InboxSummary, t: InboxPlural): NavItemStatus | undefined {
  const count = s.waiting + s.mentions
  if (count > 0) {
    const parts = [
      s.waiting > 0 ? t('nav.inboxWaiting', s.waiting) : '',
      s.mentions > 0 ? t('nav.inboxMentions', s.mentions) : '',
    ].filter(Boolean)
    return { tone: s.waiting > 0 ? 'warning' : 'new', label: parts.join(', '), count }
  }
  if (s.unread_channels > 0) {
    return { tone: 'info', label: t('nav.inboxUnread', s.unread_channels) }
  }
  return undefined
}
