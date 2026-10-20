<script setup lang="ts">
/**
 * The channel section of an entity page, mounted by ChannelsPanel once
 * realtime.yaml attaches a channel type to the entity type. It renders
 * nothing when the server lists no type this user may read.
 *
 * With exactly one channel and nothing to create it opens that channel
 * directly. Otherwise RlEntityChannels lists them and the chosen one opens
 * below the list. One channel is open at a time, and while one is open the
 * list drops its live subscription (R18), so a page holds one channel
 * subscription and the entity's own; the list is read again when the open
 * channel reports a lifecycle change or goes away.
 *
 * Channel routes take no `?world=`; the address carries the face (R14).
 * The page world goes on entity links only.
 */
import { computed, ref, watch } from 'vue'
import { createChannel, type ChannelAnchor } from '@/api/channels'
import { useEntityChannels } from '@/composables/useEntityChannels'
import { useChannelSchemaStore } from '@/stores/channelSchema'
import RlEntityChannels from 'rela-components/components/channel/RlEntityChannels.vue'
import OpenChannel from './OpenChannel.vue'
import ChannelTextDialog from './ChannelTextDialog.vue'
import { channelGroups, channelKey, parseChannelKey } from './channelView'
import { classifyChannelError } from './channelErrors'
import { useUIStore } from '@/stores'

const props = defineProps<{
  entityType: string
  /** The entity's address, face included on a faced type. */
  entityId: string
  /** The page world, for entity links. */
  world?: string
}>()

const channelSchema = useChannelSchemaStore()
const ui = useUIStore()

const anchor = computed<ChannelAnchor>(() => ({ type: props.entityType, address: props.entityId }))

const filter = ref<'open' | 'all'>('open')
// Mirrors whether a channel is open, set by the watch below: the list is
// built before the open channel can be known.
const channelOpen = ref(false)
const selected = ref<string | null>(null)

const list = useEntityChannels(anchor, {
  state: () => (filter.value === 'open' ? 'open' : undefined),
  live: () => !channelOpen.value,
})

const groups = computed(() =>
  channelGroups(list.types.value, (name) => channelSchema.channelType(name))
)
const rows = computed(() => groups.value.flatMap((g) => g.channels))
const canCreate = computed(() => groups.value.some((g) => g.canCreate))

/** Whether the only channel opens without a list above it. */
const single = computed(() => rows.value.length === 1 && !canCreate.value)

const openKey = computed<string | null>(() => {
  if (selected.value !== null) return selected.value
  return single.value ? rows.value[0].key : null
})

watch(openKey, (key) => (channelOpen.value = key !== null), { immediate: true })

const open = computed(() => {
  const key = openKey.value
  if (key === null) return null
  const { channelType, name } = parseChannelKey(key)
  const ct = channelSchema.channelType(channelType)
  return ct ? { key, ct, name } : null
})

const visible = computed(() => groups.value.length > 0)

function onSelect(key: string) {
  selected.value = selected.value === key ? null : key
}

/** The open channel is gone: back to the list, read afresh. */
function onGone() {
  selected.value = null
  void list.refetch()
}

// --- Creating a channel ------------------------------------------------------

const creating = ref<string | null>(null)
const createBusy = ref(false)
const createError = ref<string>()

const createLabel = computed(() => {
  const g = groups.value.find((x) => x.type === creating.value)
  return g?.createLabel || `New ${g?.label ?? 'channel'}`
})

function onCreate(type: string) {
  createError.value = undefined
  creating.value = type
}

async function submitCreate(title: string) {
  const type = creating.value
  const a = anchor.value
  if (!type || !a) return
  createBusy.value = true
  try {
    const channel = await createChannel(a, type, title || undefined)
    creating.value = null
    filter.value = 'open'
    selected.value = channelKey(type, channel.name)
    await list.refetch()
  } catch (err) {
    const out = classifyChannelError(err)
    if (out.then !== 'none') void list.refetch()
    if (out.place === 'form') createError.value = out.message
    else {
      creating.value = null
      ui.error(out.message)
    }
  } finally {
    createBusy.value = false
  }
}
</script>

<template>
  <section v-if="visible" class="channels-panel" aria-labelledby="channels-panel-title">
    <header class="channels-panel__header">
      <h2 id="channels-panel-title">Channels</h2>
    </header>
    <div class="channels-panel__body">
      <RlEntityChannels
        v-if="!single"
        :groups="groups"
        :active-key="openKey ?? undefined"
        :filter="filter"
        @select="onSelect"
        @create="onCreate"
        @update:filter="filter = $event"
      />
      <OpenChannel
        v-if="open"
        :key="open.key"
        :entity-type="entityType"
        :address="entityId"
        :channel-type="open.ct"
        :name="open.name"
        :world="world"
        @gone="onGone"
        @changed="list.refetch()"
      />
    </div>
    <ChannelTextDialog
      v-if="creating !== null"
      :title="createLabel"
      label="Title"
      confirm-label="Create"
      pending-label="Creating…"
      :busy="createBusy"
      :error="createError"
      @close="creating = null"
      @submit="submitCreate"
    />
  </section>
</template>

<style scoped>
.channels-panel {
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: 8px;
  margin-bottom: 24px;
  overflow: hidden;
}

.channels-panel__header {
  padding: 16px 24px;
  border-bottom: 1px solid var(--rl-color-border);
}

.channels-panel__header h2 {
  margin: 0;
  font-size: 18px;
  color: var(--rl-color-text);
}

.channels-panel__body {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
  padding: 16px 24px;
}
</style>
