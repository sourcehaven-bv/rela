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
    class="na rela-na"
    :class="`na--${prominence}`"
    :data-band="suggestion.band"
    :data-prominence="prominence"
    :data-source="suggestion.source"
    :data-entity-id="suggestion.entity_id"
    aria-label="Suggested next action"
  >
    <!--
      Tier-1 operator hook (docs/customisation.md). Inert unless custom.js
      defines `rela-slot`; when it does, the operator owns the interior and
      rela keeps `data-band` current so `attributeChangedCallback` can react —
      e.g. swapping a character image per band. Placed FIRST so a definition
      reads as a companion beside the message rather than an afterthought.
    -->
    <rela-slot name="companion" :data-band="suggestion.band" :data-prominence="prominence" />

    <!-- The band label is the "why am I seeing this?" answer, so the insistent
         tier states it and the quiet tier does not shout it. -->
    <div v-if="prominence === 'banner'" class="na__band rela-na-band">{{ bandLabel }}</div>
    <p class="na__message rela-na-message">{{ suggestion.message }}</p>
    <NextActionOffers :offers="suggestion.actions || []" :entity-id="suggestion.entity_id" />
  </section>
</template>

<style scoped>
/*
 * Two page-level tiers. They differ in how hard they are to read past, not in
 * layout: a bounded "card" variant was removed because it and the banner were
 * cleared by the same shrug — two spellings of one interruption model.
 *
 * Tokens are the app's own (--card-bg, --border-color, --accent-color,
 * --muted-text, --radius-lg, --font-size-*). An earlier pass invented names
 * like --space-4 and --color-primary that do not exist in this codebase, so
 * every rule silently ran on its fallback and nothing lined up with the
 * surrounding UI.
 */
.na {
  margin-bottom: 16px;
}

/*
 * The companion slot is inert by default: no box, no space taken, so a stock
 * deployment renders exactly as if it were absent. An operator's custom.js
 * gives it content and sizes it from custom.css.
 */
.na :deep(rela-slot) {
  display: contents;
}

/* banner — deliberately hard to skim past. For onboarding and for work
   someone else is blocked on.
   
   Emphasis comes from a full accent border, NOT a left rule: in this codebase
   a left accent bar means either a markdown blockquote or something is wrong
   (ScriptErrorPanel, EntityDetail's inaccessible-banner). Borrowing that shape
   would make a suggestion read as an error. The band chip carries the colour;
   the border just says "this one matters". */
.na--banner {
  border: 1px solid var(--accent-color);
  border-radius: var(--radius-lg);
  padding: 16px;
  background: var(--card-bg);
}

.na--banner .na__message {
  margin: 0 0 12px;
  font-size: var(--font-size-lg);
  line-height: 1.4;
}

/* notice — same position, no box, no fill. Says its piece on one line and is
   easy to read past on purpose. Sits on the page's own background so it reads
   as page furniture rather than as content. */
.na--notice {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 8px 0 12px;
  border-bottom: 1px solid var(--border-color);
}

.na--notice .na__message {
  margin: 0;
  flex: 1 1 auto;
  min-width: 240px;
  color: var(--muted-text);
  font-size: var(--font-size-base);
}

/* The band chip follows the app's existing label convention (see .cmdk-type in
   CommandPaletteModal) rather than inventing a third uppercase-label style. */
.na__band {
  display: inline-block;
  margin-bottom: 8px;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  background: var(--accent-color);
  color: white;
  font-size: var(--font-size-xs);
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

@media (max-width: 768px) {
  .na--banner {
    padding: 12px;
  }

  .na--banner .na__message {
    font-size: var(--font-size-base);
  }
}
</style>
