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
import { ref, computed, watch } from 'vue'
import { getNextAction, sendNextActionFeedback } from '@/api'
import { useSchemaStore } from '@/stores'
import { useWorld } from '@/composables/useWorld'
import type { NextActionSuggestion, NextActionProminence, NextActionFeedbackKind } from '@/types'

const suggestion = ref<NextActionSuggestion | null>(null)
const busy = ref(false)
/** Whether the status-bar popover is expanded. */
const expanded = ref(false)
/** The suggestion whose impression has already been reported, if any. */
const shownKey = ref<string | null>(null)
/**
 * The DISPLAY world the current suggestion was resolved for, or null when
 * nothing has been resolved yet.
 *
 * The latch is the world rather than a boolean because a source may declare
 * `visible_worlds:`, so the answer to "what is the one thing?" differs per
 * world. A plain "already loaded" flag would keep serving the first world's
 * answer for the rest of the session — see loadFor.
 */
let loadedWorld: string | null = null
/**
 * Whether the world watcher has been registered.
 *
 * Registered ONCE for the singleton, not once per consumer. The watcher has
 * no component scope to be disposed with — `useNextAction()` is called from
 * two long-lived components and from tests — so a per-consumer watch would
 * accumulate live watchers for the session, each firing its own reload on a
 * world change.
 */
let worldWatchRegistered = false
/** Stops the world watcher. Test-only, paired with __resetNextActionForTest. */
let stopWorldWatch: (() => void) | null = null

/** Identity of a suggestion for impression de-duplication. */
function suggestionKey(s: NextActionSuggestion): string {
  return `${s.source}\u0000${s.entity_id ?? ''}\u0000${s.variant ?? ''}`
}

/**
 * Reset the module singleton. Test-only: the state is deliberately shared
 * across consumers for the whole session, so a suite without this would leak
 * one test's suggestion into the next.
 */
export function __resetNextActionForTest() {
  suggestion.value = null
  busy.value = false
  expanded.value = false
  shownKey.value = null
  loadedWorld = null
  stopWorldWatch?.()
  stopWorldWatch = null
  worldWatchRegistered = false
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

  async function load(world = worldParam.value) {
    loadedWorld = world ?? ''
    try {
      const res = await getNextAction(world)
      suggestion.value = res.suggestion
      // The impression is NOT reported here. `load()` also runs after
      // feedback, so reporting from it would mark the REPLACEMENT suggestion
      // as shown before it had been rendered — starting a 24h cooldown for
      // something the user never saw. `markShown()` is called by the
      // component that actually displays it.
    } catch (err) {
      // An advisory surface must never break the page it sits on.
      console.error('next-action load failed:', err)
      suggestion.value = null
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
   * Resolve for the world currently being browsed, reusing the resolved
   * suggestion when that world has already been answered.
   *
   * Once per WORLD, not once per session. Two components render the same
   * suggestion (page card + status bar), so a second resolve for the same
   * world would double the impression and could surface two different
   * suggestions at once. But a source may be scoped by `visible_worlds:`, so
   * a different world is a genuinely different question — reusing the
   * previous world's answer there would leave a stale suggestion on screen.
   */
  async function loadOnce() {
    const world = worldParam.value ?? ''
    if (loadedWorld === world) return
    await load(worldParam.value)
  }

  // Re-resolve whenever the browsed world changes, so switching world does not
  // leave the previous world's suggestion on screen. Once for the singleton —
  // see worldWatchRegistered.
  if (!worldWatchRegistered) {
    worldWatchRegistered = true
    stopWorldWatch = watch(worldParam, () => {
      void loadOnce()
    })
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
      expanded.value = false
      await load()
    } catch (err) {
      console.error('next-action feedback failed:', err)
    } finally {
      busy.value = false
    }
  }

  /** "Seen it" — the impression is already recorded, so just clear the slot. */
  function acknowledge() {
    suggestion.value = null
    expanded.value = false
  }

  return {
    suggestion,
    busy,
    expanded,
    bandLabel,
    prominence,
    isPageLevel,
    isStatusBar,
    loadOnce,
    markShown,
    respond,
    acknowledge,
  }
}
