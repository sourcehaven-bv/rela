<script setup lang="ts">
import { computed, ref } from 'vue'
import RlSortableList from 'rela-components/components/data/RlSortableList.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlNumberField from 'rela-components/components/form/RlNumberField.vue'
import RlSegmentedControl from 'rela-components/components/data/RlSegmentedControl.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import RlTag from 'rela-components/components/common/RlTag.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigSplit from '@/components/configure/ConfigSplit.vue'
import ConfigSection from '@/components/configure/ConfigSection.vue'
import ConfigPreview from '@/components/configure/ConfigPreview.vue'
import LockedNote from '@/components/configure/LockedNote.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { dashboardModel } from '@/configure/models'
import { listAt, newMap, positionAfterMove, type TreeValue } from '@/configure/tree'

/**
 * The dashboard: cards in order, each a count, a table or a breakdown of
 * the records a query finds. A card shown only to some people is listed but
 * not edited here.
 */
const draft = useConfigDraftStore()

const dash = computed(() => dashboardModel(draft.currentDataEntry))
const rows = computed(() =>
  dash.value.cards.map((c) => ({ ...c, id: String(c.index), title: c.title || 'Untitled card' }))
)
const editing = ref<number | undefined>(undefined)
const card = computed(() => dash.value.cards.find((c) => c.index === editing.value))

const displays = [
  { value: 'count', label: 'Count' },
  { value: 'table', label: 'Table' },
  { value: 'breakdown', label: 'Breakdown' },
]
const displayLabel = (value: string) => displays.find((d) => d.value === value)?.label ?? value

function cardSetting(key: string, value: TreeValue | undefined) {
  if (editing.value === undefined) return
  draft.setAt('screens', ['dashboard', 'cards', editing.value], key, value)
}

function addCard() {
  draft.appendAt(
    'screens',
    ['dashboard'],
    'cards',
    newMap({ title: 'New card', query: '', display: 'count' })
  )
  editing.value = (draftCards()?.length ?? 1) - 1
}

/**
 * Moves a card. The open editor addresses its card by list position, so it
 * follows that card to its new position rather than editing whatever card
 * lands where it was.
 */
function moveCard(from: number, to: number) {
  const length = draftCards()?.length ?? 0
  draft.moveAt('screens', ['dashboard', 'cards'], from, to)
  if (editing.value === undefined) return
  const next = positionAfterMove(editing.value, from, to, length)
  editing.value = next < 0 ? undefined : next
}

function draftCards() {
  return listAt(draft.currentDataEntry, ['dashboard', 'cards'])
}

function removeCard(index: number) {
  draft.removeAt('screens', ['dashboard', 'cards'], index)
  editing.value = undefined
}
</script>

<template>
  <ConfigurePage title="Dashboard">
    <template #actions>
      <RlButton variant="secondary" icon="plus" @click="addCard">New card</RlButton>
    </template>
    <ConfigSplit>
      <ConfigSection
        title="Cards"
        :count="rows.length"
        description="The home page shows these cards in this order."
      >
        <RlSortableList
          :items="rows"
          label="Dashboard cards"
          data-testid="config-cards"
          @move="(row, to) => moveCard(row.index, to)"
        >
          <template #item="{ item }">
            <div class="card-row">
              <button
                type="button"
                class="card-row__open"
                :disabled="item.restricted"
                @click="editing = item.index"
              >
                <RlText size="sm" weight="medium">{{ item.title }}</RlText>
                <RlTag :label="displayLabel(item.display)" />
                <RlTag v-if="item.restricted" label="Only some people" />
              </button>
              <RlIconButton
                v-if="!item.restricted"
                icon="delete"
                :label="`Remove ${item.title}`"
                @click="removeCard(item.index)"
              />
            </div>
          </template>
        </RlSortableList>
      </ConfigSection>
      <ConfigSection v-if="card" :title="`Card: ${card.title || 'Untitled card'}`">
        <LockedNote v-if="card.restricted"
          >Who sees this card is set in the project's files.</LockedNote
        >
        <RlTextField
          :model-value="card.title"
          label="Title"
          required
          @update:model-value="cardSetting('title', $event)"
        />
        <RlTextField
          :model-value="card.query"
          label="Records"
          class="mono"
          hint="A query, such as type:ticket status:ready."
          @update:model-value="cardSetting('query', $event)"
        />
        <div>
          <RlText size="sm" weight="medium" as="div" class="label">Show as</RlText>
          <RlSegmentedControl
            :model-value="card.display"
            label="Show as"
            :options="displays"
            @update:model-value="cardSetting('display', $event)"
          />
        </div>
        <RlTextField
          v-if="card.display === 'breakdown'"
          :model-value="card.groupBy"
          label="Grouped by"
          hint="The property whose values the breakdown counts."
          @update:model-value="cardSetting('group_by', $event)"
        />
        <RlNumberField
          v-if="card.display === 'table'"
          :model-value="card.limit"
          label="At most"
          :min="1"
          @update:model-value="cardSetting('limit', $event)"
        />
      </ConfigSection>

      <template #aside>
        <ConfigPreview subject="Dashboard">
          <div class="preview-grid">
            <div v-for="c in dash.cards" :key="c.index" class="preview-card">
              <RlText size="sm" weight="medium" as="div">{{ c.title || 'Untitled card' }}</RlText>
              <RlText v-if="c.display === 'count'" size="lg" weight="semibold">12</RlText>
              <RlText v-else size="sm" tone="muted"
                >{{ displayLabel(c.display) }} of {{ c.query || 'all records' }}</RlText
              >
            </div>
          </div>
        </ConfigPreview>
      </template>
    </ConfigSplit>
  </ConfigurePage>
</template>

<style scoped>
.card-row {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
}

.card-row__open {
  all: unset;
  box-sizing: border-box;
  flex: 1;
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  cursor: pointer;
}

.card-row__open:disabled {
  cursor: default;
}

.card-row__open:focus-visible {
  outline: none;
  box-shadow:
    0 0 0 2px var(--rl-color-bg),
    0 0 0 4px var(--rl-color-focus);
}

.label {
  margin-bottom: 6px;
}

.mono :deep(input) {
  font-family: var(--rl-font-family-mono);
}

.preview-card {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
  padding: var(--rl-space-3);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
}

.preview-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--rl-space-3);
}
</style>
