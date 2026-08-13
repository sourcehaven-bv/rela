<script setup lang="ts">
/**
 * The page-level next-action surface: ONE suggestion, or nothing.
 *
 * Renders only the `banner` and `notice` prominences — the tiers that belong
 * on the page. A `statusbar` suggestion is rendered by StatusBar instead, so
 * this component stays empty for it rather than duplicating the slot.
 *
 * When nothing is owed it renders NOTHING: no empty state, no "you're all
 * caught up" card. Silence is the normal condition of a well-configured
 * system, and a placeholder would turn that quiet into noise on every page.
 */
import { onMounted } from 'vue'
import { useNextAction } from '@/composables/useNextAction'
import NextActionOffers from './NextActionOffers.vue'

const { suggestion, bandLabel, prominence, isPageLevel, loadOnce } = useNextAction()

onMounted(loadOnce)
</script>

<template>
  <section
    v-if="isPageLevel && suggestion"
    class="na"
    :class="`na--${prominence}`"
    aria-label="Suggested next action"
  >
    <!-- The band label is the "why am I seeing this?" answer, so the insistent
         tier states it and the quiet tier does not shout it. -->
    <div v-if="prominence === 'banner'" class="na__band">{{ bandLabel }}</div>
    <p class="na__message">{{ suggestion.message }}</p>
    <NextActionOffers :offers="suggestion.actions || []" :entity-id="suggestion.entity_id" />
  </section>
</template>

<style scoped>
/*
 * Two page-level tiers. They differ in how hard they are to read past, not in
 * layout: a bounded "card" variant was removed because it and the banner were
 * cleared by the same shrug — two spellings of one interruption model.
 */
.na {
  margin-bottom: var(--space-4, 16px);
}

/* banner — accented, filled, deliberately hard to skim past. For onboarding
   and for work someone else is blocked on. */
.na--banner {
  border: 1px solid var(--color-primary, #3b82f6);
  border-left: 4px solid var(--color-primary, #3b82f6);
  border-radius: var(--radius-md, 8px);
  padding: var(--space-4, 16px);
  background: color-mix(in srgb, var(--color-primary, #3b82f6) 8%, var(--color-surface));
}

.na--banner .na__message {
  font-weight: 600;
}

/* notice — same position, no accent, no fill, muted text. Says its piece and
   is easy to read past on purpose. */
.na--notice {
  padding: var(--space-2, 8px) 0;
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  gap: var(--space-3, 12px);
  flex-wrap: wrap;
  color: var(--color-text-muted);
  font-size: var(--font-size-sm, 0.875rem);
}

.na--notice .na__message {
  margin: 0;
  flex: 1 1 auto;
}

.na__band {
  font-size: var(--font-size-sm, 0.75rem);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--color-primary, #3b82f6);
  font-weight: 600;
  margin-bottom: var(--space-2, 8px);
}

.na__message {
  margin: 0 0 var(--space-3, 12px);
  font-size: 1.05rem;
}
</style>
