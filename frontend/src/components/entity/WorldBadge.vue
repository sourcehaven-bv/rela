<script setup lang="ts">
/**
 * Flags that a world served a SUBSTITUTE face for one entity — and names it.
 *
 * ## Why this exists at all
 *
 * Under a world with `otherwise: default`, "the Dutch page" and "the English
 * page, because no Dutch page exists" come back BYTE-IDENTICALLY — same id,
 * same title, same body. The only thing separating them is `_world.via`. A
 * reader looking at a `site-nl` listing cannot otherwise tell which entries
 * are actually translated, and neither can the editor deciding what to
 * translate next.
 *
 * ## The badge is an EXCEPTION marker, and renders ONLY for a substitute
 *
 * A first-choice hit renders NOTHING. The badge earns attention precisely
 * because its presence is the exception: it means "what you are reading is
 * NOT the face this world would normally give you." A badge on every row is
 * noise, and noise trains people to ignore it — at which point the one row
 * that genuinely needed to be read differently is the one that gets skipped.
 *
 * This component owns that rule so every surface (list table, list mobile
 * card, EntityDetail entry + section rows, RelationPicker) gets it
 * consistently and no future consumer has to remember it. Mounting the
 * component for a non-substitute is harmless; it emits no markup.
 *
 * `unscoped` renders nothing for the same reason from the other end: it means
 * no resolution was applied, which is the default world's answer for
 * everything and carries no information.
 *
 * ## A substitute is not only the `fallback-default` arm
 *
 * `via` has two ways of saying "you are looking at a stand-in", and for a
 * while this component only handled one. `fallback-default` fires when the
 * chain matched NOTHING and the world's `otherwise:` arm supplied the
 * default state. But a chain with several candidates can also substitute
 * WITHIN itself: under `select: [published, draft]` a missing published face
 * resolves to the draft, and that reports `via: 'chain'` — identical to a
 * genuine published hit.
 *
 * That was reported live: a draft-only policy rendered under `?world=published`
 * with a "read-only" framing implying it was the published face. The badge
 * said nothing, because `via` said `chain`.
 *
 * `chain_position` is the missing fact. Position 0 is the world's first
 * choice; anything greater is a stand-in and reads exactly like
 * `fallback-default`, because to a reader they are the same statement.
 */
import { computed } from 'vue'
import type { EntityWorld } from '@/types'

const props = defineProps<{ world?: EntityWorld }>()

/**
 * Whether the reader is looking at a STAND-IN rather than the face the world
 * asked for — true for both substitute shapes (see the component doc).
 *
 * The `> 0` test is deliberately not `!== 0`: an older server omits
 * `chain_position` entirely, and `undefined > 0` is false, so a response that
 * cannot answer the question is treated as a first-choice hit. That is the
 * pre-existing behaviour, unchanged — the badge does not invent a warning it
 * has no evidence for.
 */
const isSubstitute = computed(() => {
  const w = props.world
  if (!w) return false
  if (w.via === 'fallback-default') return true
  return w.via === 'chain' && (w.chain_position ?? 0) > 0
})

const text = computed(() => {
  const w = props.world
  if (!w) return ''
  switch (w.via) {
    case 'chain':
      // The coordinate the world served. Name it, because "published" (or
      // "draft", when a draft stood in) is more useful to a reader than the
      // world's own name.
      //
      // The `|| w.name` arm should be unreachable: `chain` means a SELECTED
      // coordinate matched, and the default state is reported as `unscoped`,
      // never as `chain` with an empty face. It is a display fallback so a
      // server that broke that invariant renders a world name rather than an
      // empty badge — not a case to design around.
      return w.face || w.name
    case 'fallback-default':
      return 'default'
    default:
      return ''
  }
})

const title = computed(() => {
  const w = props.world
  if (!w) return ''
  if (w.via === 'fallback-default') {
    return `No ${w.name} face exists for this entity — showing the default state instead`
  }
  // The within-chain substitute. Naming the served coordinate matters more
  // than naming the world: "showing the draft" is the fact the reader acts
  // on, and the reason the page's content may not contain what they expect.
  return `No ${w.name} face exists for this entity — showing ${w.face} instead`
})
</script>

<template>
  <!--
    ONLY a substitute renders. There is deliberately no non-substitute visual
    state: an `is-chain` class used to style a quiet first-choice badge, and
    was removed with the badge itself (see the component doc). Reintroducing
    one would put a badge back on every row.
  -->
  <span v-if="isSubstitute" class="world-badge is-fallback" :title="title">
    {{ text }}
  </span>
</template>

<style scoped>
.world-badge {
  display: inline-block;
  padding: 0.05rem 0.35rem;
  margin-left: 0.35rem;
  border-radius: 4px;
  font-size: 0.72rem;
  line-height: 1.5;
  vertical-align: middle;
  white-space: nowrap;
  border: 1px solid var(--border-color);
  color: var(--muted-text);
}

/* The substitute is the only case that renders, so it is the only case that
   carries colour. `.is-fallback` is kept as a separate class rather than
   folded into `.world-badge` because it is the hook consumers and tests use
   to assert "this is the warning state" — and because a second, quieter
   state would go here if one ever earns its place again. */
.is-fallback {
  border-color: color-mix(in srgb, var(--warning-color) 60%, transparent);
  background: color-mix(in srgb, var(--warning-color) 18%, transparent);
  color: var(--text-color);
}
</style>
