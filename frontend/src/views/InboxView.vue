<script setup lang="ts">
/**
 * The inbox page (TKT-9YRRM5): what waits on the user, the unread mentions
 * and replies, and the unread channels, live on `/api/v1/_inbox`.
 *
 * Each row links to its channel on the anchor entity's page
 * (`?channel=<type>[/<name>]`), which opens it even when it is closed or
 * archived. A mention or reply row adds `&seq=<seq>`, so the page also
 * reveals that message, even when its type is hidden or collapsed.
 *
 * "Mark read" on an unread channel marks up to its last message; on a
 * mention it marks up to the mention, which also marks every earlier
 * message read and creates a marker when the channel had none.
 */
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { markChannelRead } from '@/api/channels'
import { ApiError } from '@/api/errors'
import { useInbox } from '@/composables/useInbox'
import { useChannelSchema } from '@/stores/channelSchema'
import { useSchemaStore, useUIStore } from '@/stores'
import { useSpaceStore } from '@/stores/space'
import { browserTimeZone, formatDatetime } from '@/utils/format'
import { authorView, channelKey } from '@/components/entity/channels/channelView'
import { classifyChannelError } from '@/components/entity/channels/channelErrors'
import PageLayout from '@/components/common/PageLayout.vue'
import PageTitle from '@/components/common/PageTitle.vue'
import RlInbox from 'rela-components/components/channel/RlInbox.vue'
import RlCallout from 'rela-components/components/feedback/RlCallout.vue'
import type {
  InboxMentionRow,
  InboxRowLink,
  InboxUnreadRow,
  InboxWaitingRow,
} from 'rela-components/components/channel/types'
import type { InboxChannel } from '@/types/channels'

/** The cap of the server's waiting count; `truncated` means more than this. */
const WAITING_CAP = 20000

const { t } = useI18n()
const schema = useSchemaStore()
const channelSchema = useChannelSchema()
const space = useSpaceStore()
const ui = useUIStore()
const tz = browserTimeZone()

const { inbox, error, loading, refetch } = useInbox()

function when(value: string | undefined): string {
  if (!value) return ''
  return formatDatetime(value, tz) ?? value
}

function channelLabel(c: InboxChannel): string {
  return c.title || c.name || channelSchema.channelType(c.type)?.label || c.type
}

function rowKey(c: InboxChannel): string {
  return `${c.entity_type}/${c.anchor}/${channelKey(c.type, c.name)}`
}

function link(c: InboxChannel, seq?: number): InboxRowLink {
  const path =
    `/entity/${c.entity_type}/${encodeURIComponent(c.anchor)}` +
    `?channel=${encodeURIComponent(channelKey(c.type, c.name))}` +
    (seq !== undefined ? `&seq=${seq}` : '')
  return { as: RouterLink, attrs: { to: space.href(path) } }
}

function stateLabel(c: InboxChannel, state: string, label?: string): string {
  if (label) return label
  const states = channelSchema.channelType(c.type)?.session?.states
  return states?.find((st) => st.name === state)?.label ?? state
}

const waiting = computed<InboxWaitingRow[]>(() =>
  (inbox.value?.entries ?? []).map((e) => ({
    id: `w:${rowKey(e.channel)}`,
    anchorTitle: e.channel.anchor_title || e.channel.anchor,
    entityType: schema.getEntityType(e.channel.entity_type)?.label || e.channel.entity_type,
    channel: channelLabel(e.channel),
    state: stateLabel(e.channel, e.session.state, e.session.label),
    since: when(e.since),
    sinceIso: e.since,
    link: link(e.channel),
  }))
)

/** What "Mark read" sends per row id: the channel and the seq to mark up to. */
const marks = computed(() => {
  const out = new Map<string, { channel: InboxChannel; seq: number }>()
  for (const m of inbox.value?.mentions ?? []) {
    out.set(`m:${rowKey(m.channel)}#${m.seq}`, { channel: m.channel, seq: m.seq })
  }
  for (const u of inbox.value?.unread_channels ?? []) {
    out.set(`u:${rowKey(u.channel)}`, { channel: u.channel, seq: u.last_seq })
  }
  return out
})

const mentions = computed<InboxMentionRow[]>(() =>
  (inbox.value?.mentions ?? []).map((m) => ({
    id: `m:${rowKey(m.channel)}#${m.seq}`,
    kind: m.kind,
    author: authorView(m.author).name,
    anchorTitle: m.channel.anchor_title || m.channel.anchor,
    channel: channelLabel(m.channel),
    at: when(m.at),
    atIso: m.at,
    link: link(m.channel, m.seq),
  }))
)

const unread = computed<InboxUnreadRow[]>(() =>
  (inbox.value?.unread_channels ?? []).map((u) => ({
    id: `u:${rowKey(u.channel)}`,
    anchorTitle: u.channel.anchor_title || u.channel.anchor,
    channel: channelLabel(u.channel),
    unread: u.unread,
    link: link(u.channel),
  }))
)

const waitingMore = computed(() => {
  const got = inbox.value
  if (!got) return undefined
  const shown = got.entries?.length ?? 0
  const total = got.count ?? 0
  if (!got.truncated && total <= shown) return undefined
  return { shown, total: got.truncated ? `${WAITING_CAP}+` : String(total) }
})

const loadError = computed(() => {
  const err = error.value
  if (!err) return null
  const detail = err instanceof ApiError ? err.problem?.title : undefined
  return { message: t('inbox.loadError'), detail }
})

const busy = ref(new Set<string>())

async function onMarkRead(id: string) {
  const mark = marks.value.get(id)
  if (!mark || busy.value.has(id)) return
  busy.value = new Set([...busy.value, id])
  const c = mark.channel
  try {
    await markChannelRead(
      { type: c.entity_type, address: c.anchor, channelType: c.type, name: c.name || undefined },
      mark.seq
    )
  } catch (err) {
    ui.error(classifyChannelError(err).message)
  } finally {
    const next = new Set(busy.value)
    next.delete(id)
    busy.value = next
    void refetch()
  }
}
</script>

<template>
  <PageLayout class="inbox-view" :data-testid="loading ? 'inbox-loading' : 'inbox-loaded'">
    <template #topbar>
      <PageTitle :title="t('inbox.title')" />
    </template>

    <RlCallout v-if="loadError" tone="danger" class="inbox-view__error">
      <p class="inbox-view__error-text">{{ loadError.message }}</p>
      <p v-if="loadError.detail" class="inbox-view__error-text">{{ loadError.detail }}</p>
    </RlCallout>
    <RlInbox
      v-if="inbox"
      :waiting="waiting"
      :mentions="mentions"
      :unread="unread"
      :waiting-more="waitingMore"
      :degraded="inbox.degraded"
      :busy="busy"
      @mark-read="onMarkRead"
    />
  </PageLayout>
</template>

<style scoped>
.inbox-view__error {
  margin-bottom: var(--space-lg);
}

.inbox-view__error-text {
  margin: 0;
}
</style>
