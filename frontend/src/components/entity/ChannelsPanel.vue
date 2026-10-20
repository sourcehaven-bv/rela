<script setup lang="ts">
/**
 * The channels on an entity page (TKT-91BK6F D7), below the comments.
 *
 * Self-gating: it mounts ChannelsSection, which lists and subscribes, only
 * when realtime.yaml attaches a channel type to this entity type. Until
 * then the page holds no channel subscription and sends no channel request
 * beyond the descriptor load, which the channel schema store makes once.
 */
import { computed } from 'vue'
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
</script>

<template>
  <ChannelsSection
    v-if="configured"
    :entity-type="entityType"
    :entity-id="entityId"
    :world="world"
  />
</template>
