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
let loaded = false

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
    () => !!suggestion.value && (prominence.value === 'banner' || prominence.value === 'notice'),
  )

  /** True when the suggestion belongs in the status bar. */
  const isStatusBar = computed(() => !!suggestion.value && prominence.value === 'statusbar')

  async function load() {
    try {
      const res = await getNextAction()
      suggestion.value = res.suggestion
      // Report the impression only once resolved: this starts the cooldown.
      // The GET deliberately does not, so a prefetch or a discarded response
      // cannot silently consume the suggestion.
      if (res.suggestion) {
        await sendNextActionFeedback({
          source: res.suggestion.source,
          entity_id: res.suggestion.entity_id,
          variant: res.suggestion.variant,
          kind: 'shown',
        })
      }
    } catch (err) {
      // An advisory surface must never break the page it sits on.
      console.error('next-action load failed:', err)
      suggestion.value = null
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
    respond,
    acknowledge,
  }
}
