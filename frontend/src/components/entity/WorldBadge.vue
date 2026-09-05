<script setup lang="ts">
/**
 * The stand-in badge: marks a row or card whose face is a SUBSTITUTE for the
 * world's first choice — a within-chain fallback (`chain` at a position past
 * 0) or an `otherwise: default` substitution.
 *
 * ## Existence is the server's answer; the wording is the operator's
 *
 * Whether a row is a stand-in comes from `_world` on the row (the store
 * resolved it; this reads the answer back). What the badge SAYS comes from
 * the world's `messages.stand_in` in schema.yaml — typically `{face}`, the
 * served face's label — and nothing is rendered when the operator declared
 * nothing. The badge used to print the coordinate and a tooltip in rela's
 * own words ("No published face exists for this entity — showing the default
 * state instead"), which is storage vocabulary shown to a reader who never
 * chose it (TKT-5SZG2L).
 *
 * ## Only a substitute renders
 *
 * A first-choice hit shows nothing, so the badge marks only what is
 * surprising. `_world.via` alone cannot say which: `chain` covers both the
 * world's first choice and a later candidate standing in for it, so
 * `chain_position` is the deciding fact. An older server that omits it is
 * treated as a first-choice hit — the badge does not invent a warning it has
 * no evidence for.
 */
import { computed } from 'vue'
import { useSchemaStore } from '@/stores/schema'
import { worldText } from '@/utils/worldText'
import type { EntityWorld } from '@/types'

const props = defineProps<{
  world?: EntityWorld
  /** The row's entity type, for the served face's declared label. */
  entityType?: string
}>()

const schemaStore = useSchemaStore()

const isSubstitute = computed(() => {
  const w = props.world
  if (!w) return false
  if (w.via === 'fallback-default') return true
  return w.via === 'chain' && (w.chain_position ?? 0) > 0
})

const text = computed(() => {
  const w = props.world
  if (!w || !isSubstitute.value) return ''
  const info = schemaStore.worlds.get(w.name)
  const faces = props.entityType ? schemaStore.getEntityType(props.entityType)?.faces : undefined
  // `_world.face` is the served face's DECLARED name; '' when the type names
  // none, in which case the placeholder renders empty.
  const face = w.face ? faces?.[w.face]?.label || w.face : ''
  return worldText(info?.messages?.stand_in, {
    face,
    bare_face: props.entityType ? schemaStore.faceLabel(props.entityType, '') : '',
    world: w.name,
  })
})
</script>

<template>
  <!--
    ONLY a substitute with operator text renders. `.is-fallback` is the hook
    consumers and tests use to assert "this is the warning state".
  -->
  <span v-if="text" class="world-badge is-fallback">
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

.is-fallback {
  border-color: color-mix(in srgb, var(--warning-color) 60%, transparent);
  background: color-mix(in srgb, var(--warning-color) 18%, transparent);
  color: var(--text-color);
}
</style>
