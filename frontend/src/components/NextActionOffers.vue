<script setup lang="ts">
/**
 * The affordance row for a suggestion. Extracted so the page-level banner and
 * the status-bar popover render identical controls — a snooze that behaved
 * differently depending on where it was clicked would be a bug nobody thinks
 * to test for.
 *
 * The defer affordances (snooze durations, dismiss, mute) collapse into ONE
 * "Not now" menu. Flat, they read as three or four equal choices competing
 * with the thing the operator actually wants you to do; and only one of them
 * — mute — is about the source rather than this suggestion, which a row of
 * sibling buttons cannot express.
 */
import { computed, onMounted, onUnmounted, ref, useTemplateRef } from 'vue'
import { useNextAction } from '@/composables/useNextAction'
import type { NextActionOffer } from '@/types'

const props = defineProps<{ offers: NextActionOffer[]; entityId?: string }>()

const { busy, respond, acknowledge } = useNextAction()

const deferOpen = ref(false)
const deferRef = useTemplateRef<HTMLElement>('deferRef')

/** Snooze durations the operator offered, flattened across offers. */
const snoozeDurations = computed(() => props.offers.flatMap((o) => o.snooze || []))
const hasDismiss = computed(() => props.offers.some((o) => o.dismiss))

/** Affordances that ACT rather than defer — these stay as visible buttons. */
const actOffers = computed(() =>
  props.offers.filter((o) => o.navigate || o.acknowledge),
)

function offerLabel(offer: NextActionOffer, fallback: string): string {
  return offer.label || fallback
}

function handleClickOutside(e: MouseEvent) {
  if (deferOpen.value && deferRef.value && !deferRef.value.contains(e.target as Node)) {
    deferOpen.value = false
  }
}

async function defer(kind: 'snooze' | 'dismiss' | 'mute', duration?: string) {
  deferOpen.value = false
  await respond(kind, duration)
}

onMounted(() => document.addEventListener('click', handleClickOutside))
onUnmounted(() => document.removeEventListener('click', handleClickOutside))
</script>

<template>
  <div class="na-offers rela-na-offers">
    <template v-for="(offer, i) in actOffers" :key="i">
      <router-link
        v-if="offer.navigate"
        class="btn btn-sm btn-primary"
        :to="offer.navigate.replace('{id}', entityId || '')"
      >
        {{ offerLabel(offer, 'Open') }}
      </router-link>

      <button
        v-if="offer.acknowledge"
        class="btn btn-sm btn-secondary"
        :disabled="busy"
        @click="acknowledge()"
      >
        {{ offerLabel(offer, 'Nice') }}
      </button>
    </template>

    <!--
      One "Not now" control for every way of NOT acting. Mute is always
      present, never operator-configured: without a one-click way to switch a
      source off, an annoying suggestion can only be escaped by complying with
      it — and the mute rate is the signal telling an operator which source to
      delete. It sits last, separated, because it is about the source rather
      than this suggestion.
    -->
    <div ref="deferRef" class="na-defer">
      <button
        type="button"
        class="na-defer__trigger rela-na-defer"
        :disabled="busy"
        aria-haspopup="menu"
        :aria-expanded="deferOpen"
        @click.stop="deferOpen = !deferOpen"
      >
        Not now
        <span class="na-defer__caret" aria-hidden="true">&#9662;</span>
      </button>

      <ul v-if="deferOpen" class="na-defer__menu" role="menu">
        <li v-for="d in snoozeDurations" :key="d" role="none">
          <button type="button" class="na-defer__item" role="menuitem" @click.stop="defer('snooze', d)">
            Remind me in {{ d }}
          </button>
        </li>
        <li v-if="hasDismiss" role="none">
          <button type="button" class="na-defer__item" role="menuitem" @click.stop="defer('dismiss')">
            Not this one
          </button>
        </li>
        <li role="none">
          <button
            type="button"
            class="na-defer__item na-defer__item--sep"
            role="menuitem"
            @click.stop="defer('mute')"
          >
            Stop suggesting this
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>

<style scoped>
.na-offers {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

/* The defer control is pushed to the trailing edge: reachable, never the
   obvious click. */
.na-defer {
  position: relative;
  display: inline-block;
  margin-left: auto;
}

/* Quieter than .btn-secondary on purpose — declining should not look like an
   equal-weight alternative to acting. */
.na-defer__trigger {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 6px 10px;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  background: none;
  color: var(--muted-text);
  font-size: var(--font-size-dense);
  cursor: pointer;
  transition: all 0.15s ease;
}

.na-defer__trigger:hover:not(:disabled) {
  border-color: var(--border-color);
  color: var(--text-color);
}

.na-defer__trigger:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.na-defer__caret {
  font-size: 11px;
  line-height: 1;
  opacity: 0.75;
}

/* Mirrors .status-menu (StatusControl) so the app has one dropdown look. */
.na-defer__menu {
  position: absolute;
  right: 0;
  z-index: 20;
  margin: 4px 0 0;
  padding: 4px;
  list-style: none;
  min-width: 190px;
  background: var(--input-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
}

.na-defer__item {
  display: flex;
  align-items: center;
  width: 100%;
  padding: 8px 10px;
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-color);
  cursor: pointer;
  font-size: var(--font-size-base);
  text-align: left;
  white-space: nowrap;
}

.na-defer__item:hover {
  background: var(--hover-bg);
}

/* Mute is a different kind of act — it governs the source, not this
   suggestion — so it is separated and muted rather than listed as a peer. */
.na-defer__item--sep {
  margin-top: 4px;
  border-top: 1px solid var(--border-color);
  padding-top: 8px;
  color: var(--muted-text);
}
</style>
