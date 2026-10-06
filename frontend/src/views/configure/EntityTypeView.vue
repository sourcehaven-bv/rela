<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import RlSortableList from 'rela-components/components/data/RlSortableList.vue'
import RlSectionHeading from 'rela-components/components/layout/RlSectionHeading.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlTag from 'rela-components/components/common/RlTag.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import PropertyDrawer from '@/components/configure/PropertyDrawer.vue'
import EntityTypeSettings from '@/components/configure/EntityTypeSettings.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { useConfirm } from '@/composables/useConfirm'
import {
  entityTypeModel,
  relations,
  relationSentence,
  type PropertyModel,
} from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { getIn, has, isMap, moveKey, remove } from '@/configure/tree'

/**
 * One entity type: its properties as a sortable list, the relations it takes
 * part in as sentences, and its settings beside them. A property opens in a
 * drawer; `?property=` names it, so a change or a problem can link to it.
 */
const props = defineProps<{ name: string }>()

const draft = useConfigDraftStore()
const route = useRoute()
const router = useRouter()
const { confirm } = useConfirm()

const model = computed(() => entityTypeModel(draft.currentSchema, props.name))
const baseProperties = computed(() =>
  getIn(draft.base?.schema, ['entities', props.name, 'properties'])
)

interface Row extends PropertyModel {
  id: string
  title: string
  added: boolean
}

const rows = computed<Row[]>(() =>
  (model.value?.properties ?? []).map((p) => ({
    ...p,
    id: p.name,
    title: p.label,
    added: !has(baseProperties.value, draft.originalName(props.name, p.name) ?? p.name),
  }))
)

const outgoing = computed(() =>
  relations(draft.currentSchema).filter((r) => r.from.includes(props.name))
)
const incoming = computed(() =>
  relations(draft.currentSchema).filter(
    (r) => r.to.includes(props.name) && !r.from.includes(props.name)
  )
)

/** The property the drawer shows: a name, `new` for a new one, or none. */
const editing = computed<string | null | undefined>(() => {
  const q = route.query.property
  if (typeof q !== 'string') return undefined
  return q === '' ? null : q
})

function openProperty(name: string | null) {
  void router.replace({ query: { ...route.query, property: name ?? '' } })
}

function closeProperty() {
  const { property: _drop, ...rest } = route.query
  void router.replace({ query: rest })
}

function move(item: Row, toIndex: number) {
  draft.edit('schema', (tree) => {
    const properties = getIn(tree, ['entities', props.name, 'properties'])
    if (isMap(properties)) moveKey(properties, item.name, toIndex)
  })
}

async function removeType() {
  const label = model.value?.label ?? props.name
  const ok = await confirm({
    title: `Remove ${label}?`,
    message:
      'Forms, lists, boards and relations that use it must be changed too, or saving is refused.',
    confirmLabel: 'Remove',
    danger: true,
  })
  if (!ok) return
  draft.edit('schema', (tree) => {
    const entities = getIn(tree, ['entities'])
    if (isMap(entities)) remove(entities, props.name)
  })
  void router.push(configureRoute.entityTypes())
}

function typeText(p: Row): string {
  return p.list ? `${p.typeLabel}, several` : p.typeLabel
}
</script>

<template>
  <ConfigurePage
    :title="model?.label ?? name"
    back-label="Entity types"
    :back-to="configureRoute.entityTypes()"
  >
    <template v-if="model" #actions>
      <RlButton variant="ghost" tone="danger" icon="delete" @click="removeType">Remove</RlButton>
    </template>

    <RlEmptyState
      v-if="!model"
      title="No such entity type"
      description="It may have been removed in your draft."
    />

    <div v-else class="config-columns">
      <div class="config-main">
        <section class="config-stack">
          <RlSectionHeading title="Properties" :count="rows.length" :level="2" />
          <RlText size="sm" tone="muted" as="p" class="config-para">
            The order here is the order new forms and the detail page start from.
          </RlText>
          <RlSortableList
            :items="rows"
            :label="`Properties of ${model.label}`"
            data-testid="config-properties"
            @move="move"
          >
            <template #item="{ item }">
              <button type="button" class="property-row" @click="openProperty(item.name)">
                <RlText size="sm" weight="medium">{{ item.title }}</RlText>
                <RlText size="sm" tone="subtle">{{ typeText(item) }}</RlText>
                <span class="property-row__tags">
                  <RlTag v-if="item.added" label="New" color="green" />
                  <RlTag v-if="item.required" label="Required" color="blue" />
                  <RlTag v-if="item.managed" label="Set by the server" />
                </span>
              </button>
            </template>
          </RlSortableList>
          <div>
            <RlButton
              variant="secondary"
              icon="plus"
              size="sm"
              data-testid="config-add-property"
              @click="openProperty(null)"
            >
              Add property
            </RlButton>
          </div>
        </section>

        <section class="config-stack">
          <RlSectionHeading
            title="Relations"
            :count="outgoing.length + incoming.length"
            :level="2"
          />
          <RlText size="sm" tone="muted" as="p" class="config-para">
            Relations are shared between entity types and edited on their own page.
          </RlText>
          <ul class="relation-list">
            <li v-for="r in outgoing" :key="`out-${r.name}`">
              <RouterLink :to="configureRoute.relation(r.name)">
                <RlText size="sm">{{ relationSentence(draft.currentSchema, r) }}</RlText>
              </RouterLink>
              <RlTag v-if="r.minOutgoing" :label="`at least ${r.minOutgoing}`" color="blue" />
            </li>
            <li v-for="r in incoming" :key="`in-${r.name}`">
              <RouterLink :to="configureRoute.relation(r.name)">
                <RlText size="sm">{{ relationSentence(draft.currentSchema, r, true) }}</RlText>
              </RouterLink>
            </li>
          </ul>
          <RlText v-if="!outgoing.length && !incoming.length" size="sm" tone="muted">
            No relations yet.
          </RlText>
        </section>
      </div>

      <EntityTypeSettings :name="name" />
    </div>

    <PropertyDrawer :entity-type="name" :property="editing" @close="closeProperty" />
  </ConfigurePage>
</template>

<style scoped>
.config-columns {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 40px;
  align-items: start;
  max-width: 1100px;
}

@media (max-width: 900px) {
  .config-columns {
    grid-template-columns: minmax(0, 1fr);
  }
}

.config-main {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-8);
}

.config-stack {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
}

.config-para {
  margin: 0;
}

.property-row {
  all: unset;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  width: 100%;
  cursor: pointer;
}

.property-row:focus-visible {
  outline: none;
  box-shadow:
    0 0 0 2px var(--focus-ring-gap),
    0 0 0 4px var(--focus-ring);
}

.property-row__tags {
  margin-left: auto;
  display: flex;
  gap: 6px;
}

.relation-list {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
  margin: 0;
  padding: 0;
  list-style: none;
}

.relation-list li {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
}
</style>
