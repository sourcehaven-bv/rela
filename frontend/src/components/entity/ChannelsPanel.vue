<script setup lang="ts">
/**
 * One channels block on an entity page (TKT-91BK6F D7).
 *
 * A page has the block at the end, after the comments, and a block before
 * the first view section that does not show the entry when data-entry.yaml
 * places a channel type there (`channels.<type>.placement:
 * above_relations`, TKT-VXBE16H). `placement` says which block this is; it
 * shows the channel types placed there.
 *
 * Self-gating: it mounts ChannelsSection, which lists and subscribes, only
 * when realtime.yaml attaches a channel type in this block to this entity
 * type. Until then the page holds no channel subscription and sends no
 * channel request beyond the descriptor load, which the channel schema store
 * makes once.
 *
 * With one block on the page the section shows every type, as before
 * placements existed. With two, the upper block shows the types placed
 * there and the lower one every other type, including one the held channel
 * schema does not know yet. A block with one type is then headed with that
 * type's label so the two are told apart. "Two blocks" is decided from the
 * types realtime.yaml attaches, not from what this reader may read.
 *
 * The section is keyed by the entity address and its types. EntityDetail
 * stays mounted when the route moves to another entity, and nothing of one
 * entity's channels (a draft, a selection, an edit) may carry over to the
 * next; a reload that moves a type to the other block starts both afresh.
 */
import { computed, watch } from 'vue'
import { useChannelSchema } from '@/stores/channelSchema'
import { useSchemaStore } from '@/stores/schema'
import ChannelsSection from './channels/ChannelsSection.vue'
import { placementOf, typesAt, type ChannelPlacement } from './channels/channelPlacement'

const props = withDefaults(
  defineProps<{
    entityType: string
    /** The entity's address, face included on a faced type. */
    entityId: string
    /** The page world, for entity links. */
    world?: string
    /** Which block of the page this is. */
    placement?: ChannelPlacement
  }>(),
  { world: undefined, placement: 'end' }
)

const channelSchema = useChannelSchema()
const schema = useSchemaStore()

const attached = computed(() => channelSchema.channelTypesFor(props.entityType))
const presentation = (name: string) => schema.channelPresentations.get(name)

const types = computed(() =>
  typesAt(
    attached.value.map((ct) => ct.name),
    presentation,
    props.placement
  )
)

/** Whether the page has a second block, so this one is narrowed. */
const split = computed(
  () => new Set(attached.value.map((ct) => placementOf(presentation(ct.name)))).size > 1
)

const heading = computed(() => {
  if (!split.value || types.value.length !== 1) return undefined
  return attached.value.find((ct) => ct.name === types.value[0])?.label || undefined
})

/** The types placed above the relations, which the end block leaves out. */
const placedAbove = computed(() =>
  props.placement === 'end' && split.value
    ? typesAt(
        attached.value.map((ct) => ct.name),
        presentation,
        'above_relations'
      )
    : undefined
)

const sectionKey = computed(
  () => `${props.entityType}/${props.entityId}/${[...types.value].sort().join(',')}`
)

// A failed first load is tried again (after the store's backoff) when the
// page moves to another entity, since EntityDetail does not remount.
watch(
  () => [props.entityType, props.entityId],
  () => void channelSchema.ensureLoaded()
)
</script>

<template>
  <ChannelsSection
    v-if="types.length > 0"
    :key="sectionKey"
    :entity-type="entityType"
    :entity-id="entityId"
    :world="world"
    :types="split && placement === 'above_relations' ? types : undefined"
    :exclude="placedAbove"
    :heading="heading"
  />
</template>
