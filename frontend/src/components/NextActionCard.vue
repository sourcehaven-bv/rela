<script setup lang="ts">
/**
 * The advisory next-action surface: ONE suggestion, or nothing.
 *
 * Deliberately not a list and not a queue. The server resolves exactly one
 * suggestion — message already interpolated, affordances attached — so this
 * component renders what it is given and never re-derives a ranking. There is
 * no "show all" affordance to grow into, which is the point.
 *
 * When nothing is owed the component renders NOTHING (no empty state, no
 * "you're all caught up" card). Silence is the normal condition of a
 * well-configured system, and an empty-state box would turn that quiet into
 * visual noise on every page load.
 */
import { ref, computed, onMounted } from 'vue'
import { getNextAction, sendNextActionFeedback } from '@/api'
import { useSchemaStore } from '@/stores'
import type { NextActionSuggestion, NextActionOffer } from '@/types'

const schemaStore = useSchemaStore()

const suggestion = ref<NextActionSuggestion | null>(null)
const busy = ref(false)

/** Operator-supplied label for the suggestion's band, falling back to its id. */
const bandLabel = computed(() => {
  const s = suggestion.value
  if (!s) return ''
  const band = schemaStore.nextActionBands.find((b) => b.id === s.band)
  return band?.label || s.band
})

async function load() {
  try {
    const res = await getNextAction()
    suggestion.value = res.suggestion
    // Report the impression only once it is actually on screen: this is what
    // starts the cooldown. The GET deliberately does not, so a prefetch or a
    // discarded response cannot silently consume the suggestion.
    if (res.suggestion) {
      await sendNextActionFeedback({
        source: res.suggestion.source,
        entity_id: res.suggestion.entity_id,
        variant: res.suggestion.variant,
        kind: 'shown',
      })
    }
  } catch (err) {
    // An advisory surface must never break the page it sits on. Log and stay
    // silent rather than rendering an error where a hint would go.
    console.error('next-action load failed:', err)
    suggestion.value = null
  }
}

/** Send feedback, then re-resolve so the next suggestion (if any) appears. */
async function respond(kind: 'snooze' | 'dismiss' | 'mute', duration?: string) {
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
    await load()
  } catch (err) {
    console.error('next-action feedback failed:', err)
  } finally {
    busy.value = false
  }
}

/** Acknowledge is "seen it" — no state beyond the impression already sent. */
function acknowledge() {
  suggestion.value = null
}

function offerLabel(offer: NextActionOffer, fallback: string): string {
  return offer.label || fallback
}

onMounted(load)
</script>

<template>
  <section v-if="suggestion" class="next-action" aria-label="Suggested next action">
    <div class="next-action__band">{{ bandLabel }}</div>
    <p class="next-action__message">{{ suggestion.message }}</p>

    <div class="next-action__offers">
      <template v-for="(offer, i) in suggestion.actions || []" :key="i">
        <!-- navigate: hand off to a destination -->
        <router-link
          v-if="offer.navigate"
          class="btn btn--primary"
          :to="offer.navigate.replace('{id}', suggestion.entity_id || '')"
        >
          {{ offerLabel(offer, 'Open') }}
        </router-link>

        <!-- snooze: one button per offered duration -->
        <button
          v-for="d in offer.snooze || []"
          :key="d"
          class="btn"
          :disabled="busy"
          @click="respond('snooze', d)"
        >
          Snooze {{ d }}
        </button>

        <button
          v-if="offer.dismiss"
          class="btn"
          :disabled="busy"
          @click="respond('dismiss')"
        >
          {{ offerLabel(offer, 'Not this') }}
        </button>

        <button
          v-if="offer.acknowledge"
          class="btn"
          :disabled="busy"
          @click="acknowledge"
        >
          {{ offerLabel(offer, 'Nice') }}
        </button>
      </template>

      <!-- Muting is always available, never operator-configured: without a
           one-click way to switch a source off, an annoying suggestion can
           only be escaped by complying with it. It is also the signal that
           tells an operator which source to delete. -->
      <button
        class="btn btn--quiet next-action__mute"
        :disabled="busy"
        title="Stop showing suggestions from this source"
        @click="respond('mute')"
      >
        Mute
      </button>
    </div>
  </section>
</template>

<style scoped>
.next-action {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md, 8px);
  padding: var(--space-4, 16px);
  margin-bottom: var(--space-4, 16px);
  background: var(--color-surface);
}

.next-action__band {
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--color-text-muted);
  margin-bottom: var(--space-2, 8px);
}

.next-action__message {
  margin: 0 0 var(--space-3, 12px);
  font-size: 1.05rem;
}

.next-action__offers {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2, 8px);
  align-items: center;
}

/* Push mute to the trailing edge: available, but never the obvious click. */
.next-action__mute {
  margin-left: auto;
  opacity: 0.7;
}
</style>
