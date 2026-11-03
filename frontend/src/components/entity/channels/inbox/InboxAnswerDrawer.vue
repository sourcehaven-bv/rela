<script setup lang="ts">
/**
 * The channel of an inbox row in a drawer (TKT-V05706X), for what the row
 * cannot do itself: a form decision, "Other answer", or reading the newer
 * messages. The channel opens at the waiting message (`revealSeq`), and a
 * `start` makes the decision or choice there once the channel is held.
 *
 * `sent` reports the successful send of the started form; the inbox then
 * closes the drawer. `gone` means the channel is no longer readable, and
 * `changed` that the channel moved in a way the inbox shows.
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import RlDrawer from 'rela-components/components/overlay/RlDrawer.vue'
import OpenChannel from '@/components/entity/channels/OpenChannel.vue'
import { useChannelSchemaStore } from '@/stores/channelSchema'
import type { ChannelStart, InboxEntry } from '@/types/channels'

const props = defineProps<{
  entry: InboxEntry | null
  start?: ChannelStart
  /** The channel's display name, as the inbox row shows it. */
  label: string
}>()

const emit = defineEmits<{ close: []; sent: []; gone: []; changed: [] }>()

const { t } = useI18n()
const channelSchema = useChannelSchemaStore()

const ct = computed(() =>
  props.entry ? channelSchema.channelType(props.entry.channel.type) : undefined
)
const reveal = computed(() => (props.entry?.message ? props.entry.session.cause : undefined))
</script>

<template>
  <RlDrawer
    :open="!!entry"
    :title="t('inbox.answerTitle', { channel: label })"
    size="lg"
    @close="emit('close')"
  >
    <OpenChannel
      v-if="entry && ct"
      :key="`${entry.channel.entity_type}/${entry.channel.anchor}/${entry.channel.type}/${entry.channel.name ?? ''}`"
      :entity-type="entry.channel.entity_type"
      :address="entry.channel.anchor"
      :channel-type="ct"
      :name="entry.channel.name || undefined"
      :reveal-seq="reveal"
      :start="start"
      @gone="emit('gone')"
      @changed="emit('changed')"
      @start-sent="emit('sent')"
    />
  </RlDrawer>
</template>
