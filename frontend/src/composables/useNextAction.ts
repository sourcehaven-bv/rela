/**
 * Shared next-action state.
 *
 * A module-level singleton rather than per-component state, because two
 * components render the same suggestion at different prominences: the
 * page-level banner/notice and the status-bar chip. Resolving twice would
 * double the impression (starting a cooldown for a suggestion shown once) and
 * could show two different suggestions at once, which breaks the one-slot
 * promise the whole design rests on.
 */
import { ref, computed, watch, effectScope } from 'vue'
import { useRoute } from 'vue-router'
import { getNextAction, sendNextActionFeedback } from '@/api'
import { useSchemaStore } from '@/stores'
import { useWorld } from '@/composables/useWorld'
import {
  nextActionPlacement,
  placementKey,
  type NextActionPlacement,
} from '@/utils/nextActionPlacement'
import type { NextActionCurrent } from '@/api/entities'
import type { NextActionSuggestion, NextActionProminence, NextActionFeedbackKind } from '@/types'

const suggestion = ref<NextActionSuggestion | null>(null)
const busy = ref(false)
/**
 * Whether an `action:` offer is running (BUG-G2BASF).
 *
 * Module-level for the same reason as `busy`: the two surfaces render the SAME
 * singleton suggestion, so a per-component ref would let the banner and the
 * status-bar popover run the same mutation concurrently, with no undo.
 * Separate FROM `busy`, which tracks feedback (snooze/dismiss/mute) — the two
 * disable different controls and must not mask each other.
 */
const acting = ref(false)
/** Whether the status-bar popover is expanded. */
const expanded = ref(false)
/** The suggestion whose impression has already been reported, if any. */
const shownKey = ref<string | null>(null)
/**
 * The question the current suggestion answers, as `world + placement`, or
 * null when nothing has been resolved yet.
 *
 * The answer to "what is the one thing?" differs per DISPLAY world (a source
 * may declare `visible_worlds:`) and per PAGE (placement, and context sources
 * about the viewed entity). A plain "already loaded" flag would keep serving
 * the first page's answer for the rest of the session — see loadOnce.
 */
let loadedKey: string | null = null
/** Cancels the request in flight when a newer one supersedes it. */
let inFlight: AbortController | null = null
/**
 * Bumped per request. A response is applied only if it is still the latest,
 * so a slow answer for the previous page cannot overwrite the current one
 * even where the abort did not land in time.
 */
let requestSeq = 0
/**
 * The entity type whose changes can change the current answer, from the
 * server's `context_type`. Empty when no context source applies to the page,
 * so a graph change triggers no refresh there.
 */
let refreshType = ''
/** Pending debounced refresh, see refreshForChange. */
let refreshTimer: ReturnType<typeof setTimeout> | null = null
/** How long refreshForChange waits for more changes before it resolves. */
export const REFRESH_DEBOUNCE_MS = 1000
/**
 * The suggestion each page last showed, by request key, most recent last.
 *
 * Sent back as `current` when the reader returns to that page. One
 * on-screen value is not enough once pages differ (`visible_on`): after a
 * list, a kanban and the list again, the list's suggestion is no longer on
 * screen, its impression puts it in cooldown, and without this the list
 * would offer the next one on every round trip.
 */
const lastByPage = new Map<string, NextActionCurrent>()
/** Bound on lastByPage: entity pages give one key per entity. */
const LAST_BY_PAGE_CAP = 20
/**
 * Whether the page watcher has been registered.
 *
 * Registered ONCE for the singleton, not once per consumer, and in a
 * detached effect scope so it outlives the component that happened to
 * register it. A per-consumer watch would accumulate live watchers for the
 * session, each firing its own reload on a navigation.
 */
let pageWatchRegistered = false
/** Stops the page watcher. Test-only, paired with __resetNextActionForTest. */
let stopPageWatch: (() => void) | null = null

/** Identity of a suggestion for impression de-duplication. */
function suggestionKey(s: NextActionSuggestion | NextActionCurrent): string {
  return `${s.source}\u0000${s.entity_id ?? ''}\u0000${s.variant ?? ''}`
}

function asCurrent(s: NextActionSuggestion): NextActionCurrent {
  return { source: s.source, entity_id: s.entity_id, variant: s.variant }
}

function rememberForPage(key: string, s: NextActionSuggestion) {
  lastByPage.delete(key)
  lastByPage.set(key, asCurrent(s))
  if (lastByPage.size > LAST_BY_PAGE_CAP) {
    const oldest = lastByPage.keys().next().value
    if (oldest !== undefined) lastByPage.delete(oldest)
  }
}

/** Forget s on every page: the reader answered it, so no page should keep it. */
function forgetAnswered(s: NextActionSuggestion) {
  const k = suggestionKey(s)
  for (const [page, cur] of lastByPage) {
    if (suggestionKey(cur) === k) lastByPage.delete(page)
  }
}

/**
 * Reset the module singleton. Test-only: the state is deliberately shared
 * across consumers for the whole session, so a suite without this would leak
 * one test's suggestion into the next.
 */
export function __resetNextActionForTest() {
  suggestion.value = null
  busy.value = false
  acting.value = false
  expanded.value = false
  shownKey.value = null
  loadedKey = null
  inFlight?.abort()
  inFlight = null
  requestSeq = 0
  refreshType = ''
  if (refreshTimer) clearTimeout(refreshTimer)
  refreshTimer = null
  lastByPage.clear()
  stopPageWatch?.()
  stopPageWatch = null
  pageWatchRegistered = false
}

export function useNextAction() {
  const schemaStore = useSchemaStore()

  const band = computed(() => {
    const s = suggestion.value
    if (!s) return undefined
    return schemaStore.nextActionBands.find((b) => b.id === s.band)
  })

  const bandLabel = computed(() => band.value?.label || suggestion.value?.band || '')

  /**
   * How much this suggestion interrupts. Defaults to 'statusbar' — matching
   * the server-side default — so an unconfigured band stays out of the way.
   */
  const prominence = computed<NextActionProminence>(() => band.value?.prominence || 'statusbar')

  /** True when the suggestion belongs on the page rather than the status bar. */
  const isPageLevel = computed(
    () => !!suggestion.value && (prominence.value === 'banner' || prominence.value === 'notice')
  )

  /** True when the suggestion belongs in the status bar. */
  const isStatusBar = computed(() => !!suggestion.value && prominence.value === 'statusbar')

  /**
   * The world to RESOLVE in: the one the reader is browsing. It is the
   * DISPLAY world only — it decides which sources may surface here
   * (`visible_worlds:`), never which world a source queries. That is
   * `source_world:`, which is operator config and never travels from the
   * client.
   */
  const { worldParam } = useWorld()
  // Undefined outside a router (a component test), like useWorld's: the page
  // is then `other`.
  const route = useRoute() as ReturnType<typeof useRoute> | undefined

  /** The page being viewed, as the server's placement needs it. */
  const placement = computed<NextActionPlacement>(() =>
    route ? nextActionPlacement(route) : { surface: 'other' }
  )
  const requestKey = computed(
    () => `${worldParam.value ?? ''}\u0001${placementKey(placement.value)}`
  )

  /**
   * Resolve for the current world and page.
   *
   * `sticky` sends a suggestion as `current`, so the server keeps it if it is
   * still eligible here: the one this page last showed, else the one on
   * screen. A navigation is sticky: the reader has not answered the
   * suggestion, and swapping it on every click would use up the queue through
   * the impression cooldown. A reload after feedback or an action is not: the
   * reader answered it, and the slot should move on.
   */
  async function load(sticky = false) {
    const key = requestKey.value
    loadedKey = key
    inFlight?.abort()
    const controller = new AbortController()
    inFlight = controller
    const seq = ++requestSeq
    const s = suggestion.value
    const current = sticky ? (lastByPage.get(key) ?? (s ? asCurrent(s) : undefined)) : undefined
    try {
      const res = await getNextAction(
        worldParam.value,
        { placement: placement.value, current },
        controller.signal
      )
      if (seq !== requestSeq) return
      suggestion.value = res.suggestion
      refreshType = res.context_type ?? ''
      if (res.suggestion) rememberForPage(key, res.suggestion)
      // The impression is NOT reported here. `load()` also runs after
      // feedback, so reporting from it would mark the REPLACEMENT suggestion
      // as shown before it had been rendered — starting a 24h cooldown for
      // something the user never saw. `markShown()` is called by the
      // component that actually displays it.
    } catch (err) {
      if (seq !== requestSeq) return
      // An advisory surface must never break the page it sits on.
      console.error('next-action load failed:', err)
      suggestion.value = null
      refreshType = ''
      // Not latched: the page has not been answered, so the next loadOnce
      // for it tries again.
      if (loadedKey === key) loadedKey = null
    } finally {
      if (inFlight === controller) inFlight = null
    }
  }

  /**
   * Report that the current suggestion was actually displayed, starting its
   * cooldown. Idempotent per suggestion: the two surfaces share this state,
   * and a re-render must not count twice.
   */
  async function markShown() {
    const s = suggestion.value
    if (!s || shownKey.value === suggestionKey(s)) return
    shownKey.value = suggestionKey(s)
    try {
      await sendNextActionFeedback({
        source: s.source,
        entity_id: s.entity_id,
        variant: s.variant,
        kind: 'shown',
      })
    } catch (err) {
      console.error('next-action impression failed:', err)
    }
  }

  /**
   * Resolve for the current world and page, reusing the resolved suggestion
   * when that question has already been answered.
   *
   * Two components render the same suggestion (page card + status bar), so a
   * second resolve for the same page would double the impression and could
   * surface two different suggestions at once. A different world or page is a
   * genuinely different question, so it resolves again.
   */
  async function loadOnce() {
    if (loadedKey === requestKey.value) return
    await load(true)
  }

  // Re-resolve whenever the world or the page changes, so a navigation does
  // not leave the previous page's suggestion on screen. Once for the
  // singleton; see pageWatchRegistered.
  if (!pageWatchRegistered) {
    pageWatchRegistered = true
    const scope = effectScope(true)
    scope.run(() => {
      watch(requestKey, () => {
        void loadOnce()
      })
    })
    stopPageWatch = () => scope.stop()
  }

  /**
   * Re-resolve after an entity of `type` changed, when a context source can
   * apply to the page and is about that type (the server's `context_type`).
   * An edit may have settled the suggestion about the viewed entity, or made
   * one owed. Every other page keeps its answer until the next navigation.
   *
   * Debounced: a bulk import sends one change per entity, and every open
   * page would otherwise resolve once per change.
   */
  function refreshForChange(type: string) {
    if (!refreshType) return
    if (type && type !== refreshType) return
    if (refreshTimer) clearTimeout(refreshTimer)
    refreshTimer = setTimeout(() => {
      refreshTimer = null
      void load(true)
    }, REFRESH_DEBOUNCE_MS)
  }

  async function respond(kind: NextActionFeedbackKind, duration?: string) {
    const s = suggestion.value
    if (!s || busy.value) return
    busy.value = true
    try {
      // No world on the POST. Feedback records a per-user decision about a
      // suggestion key; it is not a read of any world, and the endpoint
      // refuses a `?world=` on a write outright (world_read_only). The GET
      // below re-resolves in the browsed world as usual.
      await sendNextActionFeedback({
        source: s.source,
        entity_id: s.entity_id,
        variant: s.variant,
        kind,
        duration,
      })
      forgetAnswered(s)
      expanded.value = false
      await load()
    } catch (err) {
      console.error('next-action feedback failed:', err)
    } finally {
      busy.value = false
    }
  }

  /**
   * "Seen it": clear the slot and tell the server. For most suggestions the
   * impression already hides it and the server does nothing; a context
   * suggestion has no impression, so the server snoozes it for the source's
   * cooldown. Without that, the next refresh would bring it straight back.
   */
  function acknowledge() {
    const s = suggestion.value
    suggestion.value = null
    expanded.value = false
    if (!s) return
    forgetAnswered(s)
    sendNextActionFeedback({
      source: s.source,
      entity_id: s.entity_id,
      variant: s.variant,
      kind: 'seen',
    }).catch((err) => console.error('next-action acknowledge failed:', err))
  }

  return {
    suggestion,
    busy,
    acting,
    expanded,
    bandLabel,
    prominence,
    isPageLevel,
    isStatusBar,
    loadOnce,
    markShown,
    refreshForChange,
    respond,
    acknowledge,
    // Re-resolve the slot for a caller that has just INVALIDATED the current
    // answer by acting on it — an `action:` offer that ran (BUG-G2BASF).
    //
    // Unconditional where loadOnce is latched: the page has not changed,
    // which is exactly why loadOnce would decline, but the graph underneath it
    // has. Not sticky: the reader acted on the suggestion. No impression is
    // reported; see load().
    reload: () => {
      if (suggestion.value) forgetAnswered(suggestion.value)
      return load()
    },
  }
}
