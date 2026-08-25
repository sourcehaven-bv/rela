<script setup lang="ts">
/**
 * Renders which FACE a world served for one entity, and which rule chose it.
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
 * So the `fallback-default` case is the whole point. `chain` is rendered too
 * (quietly), because "this IS the Dutch face" is only reassuring if its
 * absence is meaningful — a badge that appears only on fallbacks trains the
 * reader to read no-badge as "fine", which is exactly the silence-shaped
 * signal this epic keeps removing.
 *
 * `unscoped` renders NOTHING: it means no resolution was applied, which is
 * the default world's answer for everything and carries no information.
 */
import { computed } from 'vue'
import type { EntityWorld } from '@/types'

const props = defineProps<{ world?: EntityWorld }>()

const kind = computed(() => props.world?.via)

const text = computed(() => {
  const w = props.world
  if (!w) return ''
  switch (w.via) {
    case 'chain':
      // The coordinate the world selected exists. Name it, because "published"
      // is more useful to a reader than the world's own name.
      //
      // The `|| w.name` arm should be unreachable: `chain` means a SELECTED
      // coordinate matched, and the default state is reported as `unscoped`,
      // never as `chain` with an empty pointer. It is a display fallback so a
      // server that broke that invariant renders a world name rather than an
      // empty badge — not a case to design around.
      return w.pointer || w.name
    case 'fallback-default':
      return 'default'
    default:
      return ''
  }
})

const title = computed(() => {
  const w = props.world
  if (!w) return ''
  return w.via === 'fallback-default'
    ? `No ${w.name} face exists for this entity — showing the default state instead`
    : `Resolved through ${w.name}`
})
</script>

<template>
  <span
    v-if="kind === 'chain' || kind === 'fallback-default'"
    class="world-badge"
    :class="kind === 'fallback-default' ? 'is-fallback' : 'is-chain'"
    :title="title"
  >
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

/* The substitute case is the one a reader must not miss, so it is the only
   one that carries colour. `chain` stays deliberately quiet — it is the
   expected state, and styling it loudly would drown the signal. */
.is-fallback {
  border-color: color-mix(in srgb, var(--warning-color) 60%, transparent);
  background: color-mix(in srgb, var(--warning-color) 18%, transparent);
  color: var(--text-color);
}
</style>
