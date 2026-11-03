<script setup lang="ts">
/**
 * The channels on an entity page (TKT-91BK6F D7), below the comments.
 *
 * Self-gating: it mounts ChannelsSection, which lists and subscribes, only
 * when realtime.yaml attaches a channel type to this entity type. Until
 * then the page holds no channel subscription and sends no channel request
 * beyond the descriptor load, which the channel schema store makes once.
 *
 * The section is keyed by the entity address. EntityDetail stays mounted
 * when the route moves to another entity, and nothing of one entity's
 * channels (a draft, a selection, an edit) may carry over to the next.
 */
import { computed, watch } from 'vue'
import { useChannelSchema } from '@/stores/channelSchema'
import ChannelsSection from './channels/ChannelsSection.vue'

const props = defineProps<{
  entityType: string
  /** The entity's address, face included on a faced type. */
  entityId: string
  /** The page world, for entity links. */
  world?: string
}>()

const channelSchema = useChannelSchema()
const configured = computed(() => channelSchema.channelTypesFor(props.entityType).length > 0)

// A failed first load is tried again (after the store's backoff) when the
// page moves to another entity, since EntityDetail does not remount.
watch(
  () => [props.entityType, props.entityId],
  () => void channelSchema.ensureLoaded()
)
</script>

<template>
  <ChannelsSection
    v-if="configured"
    :key="`${entityType}/${entityId}`"
    :entity-type="entityType"
    :entity-id="entityId"
    :world="world"
  />
</template>
