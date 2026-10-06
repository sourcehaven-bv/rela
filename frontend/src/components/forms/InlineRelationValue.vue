<script setup lang="ts">
/**
 * A relation shown as a field among an entry's properties (TKT-CADCFX):
 * `fields: - relation: <name>` in an entry properties section.
 *
 * Shows the targets' titles. When writable, a menu lists the candidates: on a
 * single-valued relation (`max_outgoing: 1`) picking one re-points the edge in
 * one PATCH; on a multi-valued one each pick adds or removes that edge.
 */
import { computed, ref, watch } from 'vue'
import RlMenu from 'rela-components/components/overlay/RlMenu.vue'
import RlMenuItem from 'rela-components/components/overlay/RlMenuItem.vue'
import RlMenuSeparator from 'rela-components/components/overlay/RlMenuSeparator.vue'
import { listAllEntities } from '@/api'
import { useEntitiesStore, useSchemaStore } from '@/stores'
import { useWorld } from '@/composables/useWorld'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import type { Entity, ModernRelationsField } from '@/types'

export interface RelationTarget {
  id: string
  title: string
}

const props = defineProps<{
  entityType: string
  entityId: string
  relation: string
  label: string
  targets: RelationTarget[]
  writable: boolean
}>()

const emit = defineEmits<{
  /** The edge set changed on the server; the host reloads. */
  changed: []
  error: [message: string]
}>()

const schemaStore = useSchemaStore()
const entitiesStore = useEntitiesStore()
const { worldParam } = useWorld()

const relationDef = computed(() => schemaStore.getRelationType(props.relation))
const single = computed(() => relationDef.value?.max_outgoing === 1)
/** The target type when the relation has exactly one; links need it. */
const targetType = computed(() =>
  relationDef.value?.to?.length === 1 ? relationDef.value.to[0] : undefined
)

// Shown at once after a pick, before the host's reload lands.
const pending = ref<RelationTarget[] | null>(null)
const shown = computed(() => pending.value ?? props.targets)
const selected = computed(() => new Set(shown.value.map((t) => t.id)))
// The host's reload is the confirmed state; drop the optimistic one.
watch(
  () => props.targets,
  () => {
    if (!saving.value) pending.value = null
  }
)

const candidates = ref<Entity[] | null>(null)
const loading = ref(false)
const saving = ref(false)

async function loadCandidates() {
  if (candidates.value || loading.value) return
  loading.value = true
  try {
    const types = relationDef.value?.to ?? []
    const params = worldParam.value ? { world: worldParam.value } : undefined
    const lists = await Promise.all(types.map((t) => listAllEntities(t, params)))
    candidates.value = lists
      .flatMap((l) => l.data)
      .sort((a, b) => entityDisplayTitle(a).localeCompare(entityDisplayTitle(b)))
  } catch (e) {
    emit('error', e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

function typeOf(id: string): string {
  return candidates.value?.find((c) => c.id === id)?.type ?? targetType.value ?? ''
}

async function save(next: RelationTarget[], body: ModernRelationsField[string]) {
  const before = pending.value
  pending.value = next
  saving.value = true
  try {
    await entitiesStore.update(props.entityType, props.entityId, {
      relations: { [props.relation]: body },
    })
    emit('changed')
  } catch (e) {
    pending.value = before
    emit('error', e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

function pick(c: Entity) {
  const target = { id: c.id, title: entityDisplayTitle(c) }
  if (single.value) {
    if (selected.value.has(c.id)) return
    // A full linkage replaces the edge set, so the old target goes as the new
    // one comes, in one write.
    void save([target], { data: [{ type: c.type, id: c.id }] })
    return
  }
  if (selected.value.has(c.id)) {
    void save(
      shown.value.filter((t) => t.id !== c.id),
      { remove: [{ type: c.type, id: c.id }] }
    )
  } else {
    void save([...shown.value, target], { add: [{ type: c.type, id: c.id }] })
  }
}

function clear() {
  if (shown.value.length === 0) return
  if (single.value) {
    void save([], { data: [] })
    return
  }
  void save([], { remove: shown.value.map((t) => ({ type: typeOf(t.id), id: t.id })) })
}
</script>

<template>
  <div class="inline-relation-value" :data-relation="relation">
    <span v-if="shown.length === 0 && !writable" class="inline-relation-empty">—</span>
    <template v-if="!writable">
      <template v-for="(t, i) in shown" :key="t.id">
        <span v-if="i > 0" class="inline-relation-sep">, </span>
        <router-link v-if="targetType" :to="`/entity/${targetType}/${t.id}`">{{ t.title }}</router-link>
        <span v-else>{{ t.title }}</span>
      </template>
    </template>

    <RlMenu v-else align="start" class="inline-relation-menu">
      <template #trigger="{ toggle, attrs }">
        <button
          type="button"
          class="inline-relation-trigger"
          v-bind="attrs"
          :aria-label="`${label}: ${shown.map((t) => t.title).join(', ') || 'none'}`"
          :disabled="saving"
          @click="
            () => {
              void loadCandidates()
              toggle()
            }
          "
        >
          <span v-if="shown.length === 0" class="inline-relation-empty">—</span>
          <span v-else>{{ shown.map((t) => t.title).join(', ') }}</span>
        </button>
      </template>

      <RlMenuItem v-if="loading || !candidates" disabled>Loading…</RlMenuItem>
      <template v-else>
        <RlMenuItem
          v-for="c in candidates"
          :key="`${c.type}/${c.id}`"
          :current="selected.has(c.id)"
          @click="pick(c)"
        >
          {{ entityDisplayTitle(c) }}
        </RlMenuItem>
        <template v-if="shown.length > 0">
          <RlMenuSeparator />
          <RlMenuItem @click="clear">Clear</RlMenuItem>
        </template>
      </template>
    </RlMenu>
  </div>
</template>

<style scoped>
.inline-relation-value {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  min-height: 28px;
}

.inline-relation-trigger {
  font: inherit;
  color: inherit;
  background: none;
  border: 1px solid transparent;
  border-radius: var(--rl-radius-sm, 4px);
  padding: 2px 6px;
  margin-left: -7px;
  cursor: pointer;
  text-align: left;
}

.inline-relation-trigger:hover,
.inline-relation-trigger:focus-visible {
  border-color: var(--rl-color-border);
}

.inline-relation-empty {
  color: var(--rl-color-text-muted);
}
</style>
