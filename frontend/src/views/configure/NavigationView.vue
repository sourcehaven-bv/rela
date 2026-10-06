<script setup lang="ts">
import IconPicker from '@/components/configure/IconPicker.vue'
import NavIcon from '@/components/common/NavIcon.vue'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import RlSegmentedControl from 'rela-components/components/data/RlSegmentedControl.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigSplit from '@/components/configure/ConfigSplit.vue'
import ConfigSection from '@/components/configure/ConfigSection.vue'
import ConfigPreview from '@/components/configure/ConfigPreview.vue'
import NavGroupEditor from '@/components/configure/NavGroupEditor.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { boards, entityTypes, lists, spaces } from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { getIn, keys, newMap } from '@/configure/tree'

/**
 * The sidebar of each space: groups of entries that open lists, boards and
 * other pages. A project without spaces has one sidebar.
 */
const draft = useConfigDraftStore()
const route = useRoute()
const router = useRouter()

const all = computed(() => spaces(draft.currentDataEntry))
const space = computed(() => {
  const wanted = typeof route.query.space === 'string' ? route.query.space : ''
  return all.value.find((s) => s.id === wanted) ?? all.value[0]
})
const spaceOptions = computed(() => all.value.map((s) => ({ value: s.id, label: s.label || s.id })))
const navPath = computed(() =>
  space.value && space.value.index >= 0
    ? ['spaces', space.value.index, 'navigation']
    : ['navigation']
)

/**
 * What an entry can open. `name` is the entry's own label; a records entry
 * has none, because each record's title is its label.
 */
const targetChoices = computed(() => [
  ...lists(draft.currentDataEntry).map((l) => ({
    value: `list:${l.name}`,
    label: `List: ${l.title}`,
    name: l.title,
  })),
  ...boards(draft.currentDataEntry).map((b) => ({
    value: `kanban:${b.name}`,
    label: `Board: ${b.title}`,
    name: b.title,
  })),
  ...keys(getIn(draft.currentDataEntry, ['calendars'])).map((n) => ({
    value: `calendar:${n}`,
    label: `Calendar: ${n}`,
    name: n,
  })),
  ...keys(getIn(draft.currentDataEntry, ['gantts'])).map((n) => ({
    value: `gantt:${n}`,
    label: `Timeline: ${n}`,
    name: n,
  })),
  ...entityTypes(draft.currentSchema).map((t) => ({
    value: `entities:${t.name}`,
    label: `Records: ${t.plural || t.label} (in a group)`,
    name: '',
  })),
  { value: 'dashboard:', label: 'Dashboard', name: 'Dashboard' },
  { value: 'search:', label: 'Search', name: 'Search' },
])
const targets = computed(() => targetChoices.value.map(({ value, label }) => ({ value, label })))
const needsGroup = computed(() => target.value.startsWith('entities:'))
const target = ref('')
const groupOptions = computed(() => [
  { value: '', label: 'Not in a group' },
  ...(space.value?.groups ?? [])
    .filter((g) => g.index >= 0 && !g.locked && !g.fromList)
    .map((g) => ({ value: String(g.index), label: g.label })),
])
const intoGroup = ref('')

/** Where the navigation list sits: its parent mapping and its key. */
const navParent = computed(() => navPath.value.slice(0, -1))

/** Adds an entry to a group, or to the end of the sidebar. */
function addEntry() {
  if (!target.value) return
  if (needsGroup.value && !intoGroup.value) return
  const [kind, name] = target.value.split(':')
  const label = targetChoices.value.find((t) => t.value === target.value)?.name || undefined
  const entry = newMap({ label, [kind]: name || true })
  if (intoGroup.value)
    draft.appendAt('screens', [...navPath.value, Number(intoGroup.value)], 'items', entry)
  else draft.appendAt('screens', navParent.value, 'navigation', entry)
  target.value = ''
}

const newGroup = ref('')
function addGroup() {
  const label = newGroup.value.trim()
  if (!label) return
  draft.appendAt('screens', navParent.value, 'navigation', newMap({ group: label }))
  newGroup.value = ''
}

function setSpace(id: string) {
  void router.replace(configureRoute.navigation(id))
}
</script>

<template>
  <ConfigurePage title="Navigation">
    <RlEmptyState v-if="!space" title="No navigation yet" />
    <ConfigSplit v-else aside="preview">
      <RlSegmentedControl
        v-if="all.length > 1"
        :model-value="space.id"
        label="Space"
        :options="spaceOptions"
        @update:model-value="setSpace"
      />
      <ConfigSection v-if="space.index >= 0" title="Space">
        <div class="grid">
          <RlTextField
            :model-value="space.label"
            label="Name"
            @update:model-value="draft.setAt('screens', ['spaces', space.index], 'label', $event)"
          />
          <IconPicker
            :model-value="space.icon"
            @update:model-value="draft.setAt('screens', ['spaces', space.index], 'icon', $event)"
          />
        </div>
      </ConfigSection>
      <ConfigSection
        title="Sidebar"
        description="Drag entries to change their order within a group."
      >
        <NavGroupEditor v-for="(g, i) in space.groups" :key="`${g.index}:${i}`" :group="g" />
        <RlText v-if="!space.groups.length" size="sm" tone="muted">The sidebar is empty.</RlText>
      </ConfigSection>
      <ConfigSection title="Add to the sidebar">
        <form class="add" @submit.prevent="addEntry">
          <RlSelect v-model="target" label="Opens" :options="targets" placeholder="Choose a page" />
          <RlSelect
            v-model="intoGroup"
            label="In group"
            :options="groupOptions"
            :error="needsGroup && !intoGroup ? 'Records are listed inside a group.' : undefined"
          />
          <RlButton
            variant="secondary"
            size="sm"
            icon="plus"
            type="submit"
            :disabled="!target || (needsGroup && !intoGroup)"
            >Add entry</RlButton
          >
        </form>
        <form class="add add--2" @submit.prevent="addGroup">
          <RlTextField v-model="newGroup" label="New group" />
          <RlButton
            variant="secondary"
            size="sm"
            icon="plus"
            type="submit"
            :disabled="!newGroup.trim()"
            >Add group</RlButton
          >
        </form>
      </ConfigSection>

      <template #aside>
        <ConfigPreview subject="Sidebar" sticky>
          <nav class="preview" aria-label="Sidebar preview">
            <div v-for="(g, i) in space.groups" :key="i" class="preview__group">
              <RlText v-if="g.label" size="xs" tone="subtle" weight="semibold" as="div">{{
                g.label
              }}</RlText>
              <RlText
                v-for="e in g.entries"
                :key="e.path.join('.')"
                size="sm"
                as="div"
                class="preview__entry"
                data-testid="config-nav-preview-entry"
              >
                <NavIcon :name="e.icon || null" />
                <span>{{ e.label }}</span>
              </RlText>
            </div>
          </nav>
        </ConfigPreview>
      </template>
    </ConfigSplit>
  </ConfigurePage>
</template>

<style scoped>
.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--rl-space-4);
}

.add {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  align-items: end;
  gap: var(--rl-space-3);
}

.add--2 {
  grid-template-columns: 1fr auto;
  max-width: 480px;
}

.preview {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-3);
}

.preview__group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.preview__entry {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  padding: 4px 8px;
}
</style>
