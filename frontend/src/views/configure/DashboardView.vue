<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
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
import { useConfigDraftStore } from '@/configure/configDraft'
import { dashboardModel, dashboardPath, dashboards } from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { listAt, newMap, positionAfterMove } from '@/configure/tree'

/**
 * The dashboard: cards in order, each a count, a table or a breakdown of
 * the records a query finds. A card shown only to some people, or one that
 * counts a list (`list:`), is listed but not edited here.
 *
 * `?name=` picks a named dashboard under `dashboards:` (TKT-5LQNLY); without
 * it, this edits the default one. A name the draft does not declare also
 * falls back to the default one, with a note saying so.
 */
const draft = useConfigDraftStore()
const route = useRoute()
const router = useRouter()

const { t } = useI18n()
const all = computed(() => dashboards(draft.currentDataEntry))
const wanted = computed(() => (typeof route.query.name === 'string' ? route.query.name : ''))
const name = computed(() => (all.value.some((d) => d.name === wanted.value) ? wanted.value : ''))
const unknownName = computed(() => (wanted.value && !name.value ? wanted.value : ''))
const base = computed(() => dashboardPath(name.value || undefined))
const dashOptions = computed(() => all.value.map((d) => ({ value: d.name, label: d.title })))
function setDashboard(next: string) {
  void router.replace(configureRoute.dashboard(next || undefined))
}

const dash = computed(() => dashboardModel(draft.currentDataEntry, name.value || undefined))
const rows = computed(() =>
  dash.value.cards.map((c) => ({ ...c, id: String(c.index), title: c.title || 'Untitled card' }))
)
const editing = ref<number | undefined>(undefined)
const card = computed(() => dash.value.cards.find((c) => c.index === editing.value))
// The open editor addresses a card by position in one dashboard.
watch(name, () => (editing.value = undefined))

const displays = [
  { value: 'count', label: 'Count' },
  { value: 'table', label: 'Table' },
  { value: 'breakdown', label: 'Breakdown' },
]
// Matrix and ratio cards are written in the project's files (TKT-2SGPJZ), so
// they are labelled here but are not choices in the segmented control.
const displayLabels: Record<string, string> = { matrix: 'Matrix', ratio: 'Ratio' }
const displayLabel = (value: string) =>
  displays.find((d) => d.value === value)?.label ?? displayLabels[value] ?? value

const cardSetting = draft.settingAt('screens', () =>
  editing.value === undefined ? undefined : [...base.value, 'cards', editing.value]
)

function addCard() {
  draft.appendAt(
    'screens',
    base.value,
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
  draft.moveAt('screens', [...base.value, 'cards'], from, to)
  if (editing.value === undefined) return
  const next = positionAfterMove(editing.value, from, to, length)
  editing.value = next < 0 ? undefined : next
}

function draftCards() {
  return listAt(draft.currentDataEntry, [...base.value, 'cards'])
}

function removeCard(index: number) {
  draft.removeAt('screens', [...base.value, 'cards'], index)
  editing.value = undefined
}
</script>

<template>
  <ConfigurePage title="Dashboard">
    <template #actions>
      <RlButton variant="secondary" icon="plus" @click="addCard">New card</RlButton>
    </template>
    <ConfigSplit>
      <RlText
        v-if="unknownName"
        size="sm"
        tone="muted"
        as="p"
        data-testid="config-dashboard-unknown"
      >
        No dashboard is named “{{ unknownName }}”. These are the default dashboard's cards.
      </RlText>
      <RlSegmentedControl
        v-if="all.length > 1"
        :model-value="name"
        label="Dashboard"
        :options="dashOptions"
        data-testid="config-dashboard-picker"
        @update:model-value="setDashboard"
      />
      <ConfigSection
        title="Cards"
        :count="rows.length"
        :description="
          name
            ? 'This dashboard shows these cards in this order.'
            : 'The home page shows these cards in this order.'
        "
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
                :disabled="item.restricted || !!item.list"
                @click="editing = item.index"
              >
                <RlText size="sm" weight="medium">{{ item.title }}</RlText>
                <RlTag :label="displayLabel(item.display)" />
                <RlTag v-if="item.restricted" label="Only some people" />
                <RlTag v-if="item.list" label="From a list" />
              </button>
              <RlIconButton
                v-if="!item.restricted && !item.list"
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
              <RlText v-else size="sm" tone="muted">{{
                t('dashboard.previewOf', {
                  display: displayLabel(c.display),
                  source: c.list
                    ? t('dashboard.previewList', { name: c.list })
                    : c.query || 'all records',
                })
              }}</RlText>
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
