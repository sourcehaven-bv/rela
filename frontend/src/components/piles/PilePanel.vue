<script setup lang="ts">
/**
 * One pile, slid out from its sidebar row (TKT-K3RJLH).
 *
 * The items are grouped by type so a mixed pile still scans, newest first
 * within each group, as the server orders them. Ticking rows narrows what the
 * head offers: with nothing ticked it is the pile as a whole (Step through,
 * export, actions on every item); with rows ticked it is those rows (Remove,
 * actions on the ticked ones).
 *
 * Every item shown is one the owner may read in the current world; the
 * server leaves the rest out, so the count here, the sidebar count and the
 * step-through total agree.
 */
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { usePile, usePiles } from '@/composables/usePiles'
import { useActionFanout } from '@/composables/useListActions'
import { useConfirm } from '@/composables/useConfirm'
import { useWorld } from '@/composables/useWorld'
import { useSchemaStore, useUIStore } from '@/stores'
import { pileExportUrl, type PileItem } from '@/api/piles'
import { shouldDropHeldContent } from '@/api/errors'
import type { ActionConfig } from '@/types'
import { pileItemRoute } from './pileRoute'
import ExportMenu from '@/components/entity/ExportMenu.vue'
import NewPileDialog from './NewPileDialog.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIcon from 'rela-components/components/common/RlIcon.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlMenu from 'rela-components/components/overlay/RlMenu.vue'
import RlMenuItem from 'rela-components/components/overlay/RlMenuItem.vue'
import RlMenuSection from 'rela-components/components/overlay/RlMenuSection.vue'
import RlMenuSeparator from 'rela-components/components/overlay/RlMenuSeparator.vue'
import RlPanelListRow from 'rela-components/components/layout/RlPanelListRow.vue'
import RlSectionHeading from 'rela-components/components/layout/RlSectionHeading.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'

const props = defineProps<{ pileId: string }>()

const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const piles = usePiles()
const fanout = useActionFanout()
const { confirm } = useConfirm()
const { worldParam } = useWorld()

const pileId = computed<string | null>(() => props.pileId)
const query = usePile(pileId)
// A refused refetch drops the pile it held (CLAUDE.md, "Held content is
// read-side ACL's blind spot"): a 404 means the pile, or every item on it,
// is no longer the principal's to see.
const pile = computed(() =>
  query.error.value && shouldDropHeldContent(query.error.value) ? null : (query.data.value ?? null)
)
const items = computed<PileItem[]>(() => pile.value?.items ?? [])

/** Ticked rows, by address. */
const ticked = ref<Set<string>>(new Set())

// A ticked row that left the pile (removed here, deleted, or no longer
// readable) is no longer something to act on.
watch(items, (next) => {
  const present = new Set(next.map((i) => i.address))
  const kept = [...ticked.value].filter((a) => present.has(a))
  if (kept.length !== ticked.value.size) ticked.value = new Set(kept)
})
watch(pileId, () => (ticked.value = new Set()))

function setTicked(address: string, on: boolean) {
  const next = new Set(ticked.value)
  if (on) next.add(address)
  else next.delete(address)
  ticked.value = next
}

function clearTicked() {
  ticked.value = new Set()
}

function typeLabel(type: string): string {
  const def = schemaStore.getEntityType(type)
  return def?.label_plural || def?.label || type
}

const groups = computed(() => {
  const out = new Map<string, PileItem[]>()
  for (const item of items.value) {
    const list = out.get(item.type)
    if (list) list.push(item)
    else out.set(item.type, [item])
  }
  return [...out].map(([type, rows]) => ({ type, label: typeLabel(type), rows }))
})

/** What an action runs on: the ticked rows, else the whole pile. */
const targets = computed(() =>
  ticked.value.size ? items.value.filter((i) => ticked.value.has(i.address)) : items.value
)
const targetLabel = computed(() =>
  ticked.value.size ? `Run on ${ticked.value.size} selected` : `Run on all ${items.value.length}`
)

const pileActions = computed(() => {
  const out: { id: string; config: ActionConfig }[] = []
  for (const id of schemaStore.pilesConfig?.actions ?? []) {
    const config = schemaStore.getAction(id)
    if (config) out.push({ id, config })
  }
  return out
})

const exportAllowed = computed(() => schemaStore.pilesConfig?.export ?? [])

function exportUrlFor(transform: string): string {
  return pileExportUrl(props.pileId, transform, worldParam.value)
}

const firstItemRoute = computed(() => {
  const first = items.value[0]
  return first ? pileItemRoute(first, props.pileId, worldParam.value) : undefined
})

function itemRoute(item: PileItem) {
  return pileItemRoute(item, props.pileId, worldParam.value)
}

async function runAction(id: string, config: ActionConfig, event?: MouseEvent) {
  const runOn = targets.value.map((i) => ({ address: i.address, type: i.type }))
  if (runOn.length === 0 || fanout.processing.value) return
  const triggerEl = event?.currentTarget instanceof HTMLElement ? event.currentTarget : null
  if (config.confirm) {
    const ok = await confirm({
      title: config.label || id,
      message:
        typeof config.confirm === 'string'
          ? config.confirm
          : `Run ${config.label || id} on ${runOn.length} item${runOn.length === 1 ? '' : 's'}?`,
      confirmLabel: 'Run',
    })
    if (!ok) return
  }
  await fanout.run(id, config, runOn, triggerEl)
  clearTicked()
  // An action can change what the owner may read, and so what the pile shows.
  await piles.refresh()
}

async function removeTicked() {
  const p = pile.value
  if (!p || ticked.value.size === 0) return
  const addresses = [...ticked.value]
  if (await piles.removeItems(p, addresses)) clearTicked()
}

async function copyIds() {
  const text = items.value.map((i) => i.address).join('\n')
  try {
    await navigator.clipboard.writeText(text)
    uiStore.success(`Copied ${items.value.length} ID${items.value.length === 1 ? '' : 's'}`)
  } catch {
    uiStore.error('Could not copy to the clipboard')
  }
}

async function deletePile() {
  const p = pile.value
  if (!p) return
  const ok = await confirm({
    title: 'Delete pile',
    message: `Delete the pile ${p.name}? The entities on it are not affected.`,
    confirmLabel: 'Delete',
    danger: true,
  })
  if (ok) await piles.remove(p)
}

const renaming = ref(false)
</script>

<template>
  <div class="pile-panel" data-testid="pile-panel">
    <RlText
      v-if="query.isPending.value && !pile"
      as="p"
      tone="subtle"
      size="sm"
      class="pile-panel__note"
    >
      Loading…
    </RlText>
    <RlText
      v-else-if="query.error.value && !pile"
      as="p"
      tone="subtle"
      size="sm"
      class="pile-panel__note"
    >
      This pile could not be loaded.
    </RlText>

    <template v-else-if="pile">
      <!-- Nothing ticked: the pile as a whole. Ticked rows: act on those. -->
      <div v-if="!ticked.size" class="pile-panel__head">
        <RlText tone="subtle" size="sm"
          >{{ items.length }} item{{ items.length === 1 ? '' : 's' }}</RlText
        >
        <div class="pile-panel__head-actions">
          <ExportMenu
            v-if="items.length"
            :url-for="exportUrlFor"
            :allowed="exportAllowed"
            size="sm"
          />
          <RlButton
            v-if="firstItemRoute"
            :as="RouterLink"
            :to="firstItemRoute"
            variant="primary"
            size="sm"
            icon="play"
            data-testid="pile-step-through"
          >
            Step through
          </RlButton>
          <RlMenu>
            <template #trigger="{ toggle, attrs }">
              <RlIconButton icon="ellipsis" label="Pile actions" v-bind="attrs" @click="toggle" />
            </template>
            <template v-if="pileActions.length">
              <RlMenuSection :label="`Actions · ${targetLabel.toLowerCase()}`" />
              <RlMenuItem
                v-for="a in pileActions"
                :key="a.id"
                :disabled="!items.length || fanout.processing.value"
                data-testid="pile-action"
                @click="runAction(a.id, a.config, $event)"
              >
                {{ a.config.label || a.id }}
              </RlMenuItem>
              <RlMenuSeparator />
            </template>
            <RlMenuItem icon="edit" data-testid="pile-rename" @click="renaming = true"
              >Rename</RlMenuItem
            >
            <RlMenuItem icon="copy" :disabled="!items.length" @click="copyIds">Copy IDs</RlMenuItem>
            <RlMenuSeparator />
            <RlMenuItem icon="trash-2" tone="danger" data-testid="pile-delete" @click="deletePile">
              Delete pile
            </RlMenuItem>
          </RlMenu>
        </div>
      </div>
      <div v-else class="pile-panel__head">
        <RlText size="sm" weight="medium" data-testid="pile-ticked"
          >{{ ticked.size }} selected</RlText
        >
        <div class="pile-panel__head-actions">
          <RlMenu v-if="pileActions.length">
            <template #trigger="{ toggle, attrs }">
              <RlButton size="sm" icon="play" v-bind="attrs" @click="toggle">
                Run action
                <template #trailing
                  ><RlIcon name="chevron-down" :size="14" aria-hidden="true"
                /></template>
              </RlButton>
            </template>
            <RlMenuSection :label="targetLabel" />
            <RlMenuItem
              v-for="a in pileActions"
              :key="a.id"
              :disabled="fanout.processing.value"
              @click="runAction(a.id, a.config, $event)"
            >
              {{ a.config.label || a.id }}
            </RlMenuItem>
          </RlMenu>
          <RlButton
            size="sm"
            variant="secondary"
            tone="danger"
            icon="remove"
            data-testid="pile-remove"
            @click="removeTicked"
          >
            Remove {{ ticked.size }}
          </RlButton>
          <RlIconButton icon="x" label="Clear selection" @click="clearTicked" />
        </div>
      </div>

      <RlEmptyState
        v-if="!items.length"
        icon="layers"
        title="This pile is empty"
        description="Select rows in a list and choose Add to pile, or use Add to pile on a search or an entity page."
        size="sm"
      />
      <section v-for="g in groups" :key="g.type" class="pile-panel__group" data-testid="pile-group">
        <RlSectionHeading :title="g.label" :count="g.rows.length" size="sm" :level="3" />
        <RlPanelListRow
          v-for="item in g.rows"
          :key="item.address"
          :label="item.title || item.id"
          :meta="item.address"
          checkable
          :checked="ticked.has(item.address)"
          :as="RouterLink"
          :attrs="{ to: itemRoute(item) }"
          data-testid="pile-row"
          @update:checked="setTicked(item.address, $event)"
        />
      </section>
    </template>

    <NewPileDialog v-if="renaming && pile" :pile="pile" @close="renaming = false" />
  </div>
</template>

<style scoped>
.pile-panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--rl-space-3);
  min-height: 52px;
  padding: var(--rl-space-3) var(--rl-space-4);
}

.pile-panel__head-actions {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
}

.pile-panel__note {
  padding: var(--rl-space-4);
}

.pile-panel__group + .pile-panel__group {
  margin-top: var(--rl-space-3);
}
</style>
