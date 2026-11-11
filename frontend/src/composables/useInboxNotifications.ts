import { computed, onBeforeUnmount, watch, type Ref } from 'vue'
import { useQueryCache } from '@pinia/colada'
import { useRouter } from 'vue-router'
import { t } from '@/i18n'
import { useMe } from '@/queries/me'
import { useSchemaStore } from '@/stores'
import { useChannelSchema } from '@/stores/channelSchema'
import { useSpaceStore } from '@/stores/space'
import { UNKNOWN_USER } from '@/utils/userToken'
import { authorView } from '@/components/entity/channels/channelView'
import { readInboxIntoSummary } from '@/composables/useInboxSummary'
import { isPageHidden, refetchJitter } from '@/composables/visibleRefetch'
import { inboxRoute, inboxRowKey } from '@/utils/inboxRoutes'
import { diffInbox, inboxCandidates, type Candidate } from '@/utils/inboxNotify'
import {
  clearSeen,
  focusChannelName,
  loadSeen,
  markUnsupported,
  notificationScope,
  notifierLockName,
  saveSeen,
  support,
  usePrefsRef,
  watchSupport,
  DEFAULT_PREFS,
} from '@/utils/notificationPrefs'
import { startTabFocus, type TabFocus } from '@/utils/tabFocus'

/** The wait before a hidden tab reads, so a burst of hints costs one read. */
export const HIDDEN_DEBOUNCE_MS = 2000
/** The least time between two reads of a hidden tab. */
export const HIDDEN_MIN_INTERVAL_MS = 10_000
/** The wait before a visible tab reads, so a burst of hints costs one read. */
export const VISIBLE_DEBOUNCE_MS = 250

/** What the notifier needs from useInboxSummary. */
export interface SummaryHandle {
  hidden: Ref<boolean>
  setBackground(fn: (() => void) | null): void
}

/**
 * Browser notifications for the inbox (TKT-48S88RB). Mount once, beside the
 * inbox summary, in a component that lives as long as the app.
 *
 * One tab per project and user notifies: the holder of a Web Lock. It takes
 * the summary's live hint, also while hidden, reads the default inbox page
 * (which keeps the badge and the title current too), diffs it against the
 * stored seen set and shows what is new, unless a tab of the project has
 * focus. Every other tab keeps the visible-only refetch of TASK-550RD.
 *
 * Notifications come only from the gated inbox read, so they show nothing
 * the inbox page would not show the same principal now, and never a
 * message body.
 */
export function useInboxNotifications(enabled: Ref<boolean>, summary: SummaryHandle) {
  const cache = useQueryCache()
  const router = useRouter()
  const space = useSpaceStore()
  const schema = useSchemaStore()
  const channelSchema = useChannelSchema()
  const me = useMe()

  watchSupport()

  const scope = computed(() => {
    const user = me.data.value?.user
    return user && user !== UNKNOWN_USER ? notificationScope(user) : null
  })
  const prefs = computed(() => (scope.value ? usePrefsRef(scope.value).value : DEFAULT_PREFS))
  const active = computed(
    () =>
      enabled.value &&
      !summary.hidden.value &&
      scope.value !== null &&
      prefs.value.enabled &&
      support.value === 'granted' &&
      typeof navigator !== 'undefined' &&
      'locks' in navigator
  )

  let focus: TabFocus | null = null
  let abort: AbortController | null = null
  let release: (() => void) | null = null
  let timer: ReturnType<typeof setTimeout> | null = null
  let lastRead = 0
  // The generation whose read is in flight, or -1. A stale read still on
  // its way does not hold back the first read of a new holding.
  let readingGen = -1
  let again = false
  // Bumped by start() and stop(). A read or timer from an earlier holding
  // ends without effect, so a tab that handed the lock over cannot notify
  // beside the new holder.
  let generation = 0
  // The notifications this tab showed, by tag, so a refusal can take back
  // what the user may no longer read.
  const shown = new Map<string, Notification>()

  function closeShown() {
    for (const n of shown.values()) n.close()
    shown.clear()
  }

  function stateLabel(c: Candidate & { kind: 'waiting' }): string {
    if (c.session.label) return c.session.label
    const states = channelSchema.channelType(c.channel.type)?.session?.states
    return states?.find((st) => st.name === c.session.state)?.label ?? c.session.state
  }

  function text(c: Candidate): { title: string; body: string } {
    const type = schema.getEntityType(c.channel.entity_type)?.label || c.channel.entity_type
    const body = t('notifications.body', {
      type,
      title: c.channel.anchor_title || c.channel.anchor,
    })
    if (c.kind === 'waiting') {
      return { title: t('notifications.waiting', { state: stateLabel(c) }), body }
    }
    const author = authorView(c.author).name
    const title =
      c.kind === 'reply'
        ? t('notifications.reply', { author })
        : t('notifications.mention', { author })
    return { title, body }
  }

  function open(path: string) {
    window.focus()
    void router.push(space.href(path))
  }

  function show(title: string, body: string, tag: string, path: string) {
    try {
      const n = new Notification(title, { body, tag })
      shown.set(tag, n)
      n.onclick = () => {
        open(path)
        n.close()
        shown.delete(tag)
      }
      n.onclose = () => {
        if (shown.get(tag) === n) shown.delete(tag)
      }
    } catch (err) {
      // Chrome on Android grants permission but allows notifications only
      // from a service worker. Say so once, and stop trying.
      console.warn('inbox notifications: cannot show a notification', err)
      markUnsupported()
    }
  }

  async function read() {
    const s = scope.value
    const gen = generation
    if (!s || !active.value) return
    if (readingGen === gen) {
      again = true
      return
    }
    readingGen = gen
    lastRead = Date.now()
    try {
      const inbox = await readInboxIntoSummary(cache)
      if (gen !== generation || scope.value !== s) return
      if (!inbox) {
        // Refused: forget what was seen and take back what was shown.
        clearSeen(s)
        closeShown()
        return
      }
      if (!active.value) return
      const diff = diffInbox(inboxCandidates(inbox), loadSeen(s), prefs.value)
      // Saved also while a tab has focus: what the user can see on screen
      // counts as seen, and does not notify later.
      saveSeen(s, diff.seen)
      if (focus?.anyFocused() || support.value !== 'granted') return
      for (const c of diff.show) {
        const { title, body } = text(c)
        const seq = c.kind === 'waiting' ? undefined : c.seq
        show(title, body, inboxRowKey(c.channel), inboxRoute(c.channel, seq))
      }
      if (diff.more > 0) {
        show(t('notifications.more', { n: diff.more }, diff.more), '', 'rela-inbox-more', '/inbox')
      }
    } catch (err) {
      // The seen set is untouched, so the next hint catches up.
      console.warn('inbox notifications: inbox read failed', err)
    } finally {
      if (readingGen === gen) readingGen = -1
      if (again && gen === generation) {
        again = false
        schedule()
      }
    }
  }

  function schedule() {
    if (timer) return
    const gen = generation
    const wait = isPageHidden()
      ? Math.max(HIDDEN_DEBOUNCE_MS, lastRead + HIDDEN_MIN_INTERVAL_MS - Date.now()) +
        refetchJitter()
      : VISIBLE_DEBOUNCE_MS
    timer = setTimeout(() => {
      timer = null
      if (gen === generation) void read()
    }, wait)
  }

  function start(s: string) {
    generation++
    focus = startTabFocus(focusChannelName())
    const ac = new AbortController()
    abort = ac
    navigator.locks
      .request(notifierLockName(s), { signal: ac.signal }, () => {
        if (ac.signal.aborted) return undefined
        summary.setBackground(schedule)
        void read()
        // Hold the lock until stop() resolves this promise.
        return new Promise<void>((resolve) => {
          release = resolve
        })
      })
      .catch(() => {
        // Aborted while waiting for the lock: another tab notifies.
      })
  }

  function stop() {
    generation++
    again = false
    abort?.abort()
    abort = null
    if (release) {
      summary.setBackground(null)
      release()
      release = null
    }
    if (timer) clearTimeout(timer)
    timer = null
    focus?.stop()
    focus = null
  }

  watch(
    () => (active.value ? scope.value : null),
    (s) => {
      stop()
      if (s) start(s)
    },
    { immediate: true }
  )

  onBeforeUnmount(stop)

  return { active }
}
