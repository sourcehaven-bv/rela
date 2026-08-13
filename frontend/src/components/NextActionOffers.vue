<script setup lang="ts">
/**
 * The affordance row for a suggestion. Extracted so the page-level banner and
 * the status-bar popover render identical controls — a snooze that behaved
 * differently depending on where it was clicked would be a bug nobody thinks
 * to test for.
 */
import { useNextAction } from '@/composables/useNextAction'
import type { NextActionOffer } from '@/types'

defineProps<{ offers: NextActionOffer[]; entityId?: string }>()

const { busy, respond, acknowledge } = useNextAction()

function offerLabel(offer: NextActionOffer, fallback: string): string {
  return offer.label || fallback
}
</script>

<template>
  <div class="na-offers">
    <template v-for="(offer, i) in offers" :key="i">
      <router-link
        v-if="offer.navigate"
        class="btn btn--primary"
        :to="offer.navigate.replace('{id}', entityId || '')"
      >
        {{ offerLabel(offer, 'Open') }}
      </router-link>

      <button
        v-for="d in offer.snooze || []"
        :key="d"
        class="btn"
        :disabled="busy"
        @click="respond('snooze', d)"
      >
        Snooze {{ d }}
      </button>

      <button v-if="offer.dismiss" class="btn" :disabled="busy" @click="respond('dismiss')">
        {{ offerLabel(offer, 'Not this') }}
      </button>

      <button v-if="offer.acknowledge" class="btn" :disabled="busy" @click="acknowledge()">
        {{ offerLabel(offer, 'Nice') }}
      </button>
    </template>

    <!-- Muting is always available, never operator-configured: without a
         one-click way to switch a source off, an annoying suggestion can only
         be escaped by complying with it. It is also the signal that tells an
         operator which source to delete. -->
    <button
      class="btn na-offers__mute"
      :disabled="busy"
      title="Stop showing suggestions from this source"
      @click="respond('mute')"
    >
      Mute
    </button>
  </div>
</template>

<style scoped>
.na-offers {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2, 8px);
  align-items: center;
}

/* Reachable, never the obvious click. */
.na-offers__mute {
  margin-left: auto;
  opacity: 0.7;
}
</style>
