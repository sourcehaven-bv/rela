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
import { ref, computed } from 'vue'
import { getNextAction, sendNextActionFeedback } from '@/api'
import { useSchemaStore } from '@/stores'
import type { NextActionSuggestion, NextActionProminence, NextActionFeedbackKind } from '@/types'

const suggestion = ref<NextActionSuggestion | null>(null)
const busy = ref(false)
/** Whether the status-bar popover is expanded. */
const expanded = ref(false)
/** The suggestion whose impression has already been reported, if any. */
const shownKey = ref<string | null>(null)
let loaded = false

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
  loaded = false
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

  async function load() {
    try {
      const res = await getNextAction()
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

  /** Load once per session; repeat mounts reuse the resolved suggestion. */
  async function loadOnce() {
    if (loaded) return
    loaded = true
    await load()
  }

  async function respond(kind: NextActionFeedbackKind, duration?: string) {
    const s = suggestion.value
    if (!s || busy.value) return
    busy.value = true
    try {
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
