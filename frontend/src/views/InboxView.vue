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
 *
 * A waiting row also shows the message that moved the session and its
 * decisions (TKT-V05706X, InboxWaitingItem). A one-click decision posts
 * from the row. A form decision, "Other answer" or "Open channel" opens
 * the channel in a drawer (InboxAnswerDrawer). After a post from the row
 * or a send in the drawer, the inbox reads itself again and the focus
 * moves to the next waiting row, or to the waiting heading when none is
 * left.
 */
import { computed, nextTick, ref } from 'vue'
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
import InboxWaitingItem from '@/components/inbox/InboxWaitingItem.vue'
import InboxAnswerDrawer from '@/components/inbox/InboxAnswerDrawer.vue'
import type {
  InboxMentionRow,
  InboxRowLink,
  InboxUnreadRow,
  InboxWaitingRow,
} from 'rela-components/components/channel/types'
import type { ChannelStart, InboxChannel, InboxEntry } from '@/types/channels'

/** The cap of the server's waiting count; `truncated` means more than this. */
const WAITING_CAP = 20000

const { t } = useI18n()
const schema = useSchemaStore()
const channelSchema = useChannelSchema()
const space = useSpaceStore()
const ui = useUIStore()
const tz = browserTimeZone()

const { inbox, error, loading, refetch } = useInbox()

/** The waiting entries by row id, for the waiting-detail slot. */
const entries = computed(() => {
  const out = new Map<string, InboxEntry>()
  for (const e of inbox.value?.entries ?? []) out.set(`w:${rowKey(e.channel)}`, e)
  return out
})

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

// --- Answering from the inbox ------------------------------------------------

/**
 * The row whose channel the drawer shows, what it starts there, and the
 * waiting rows' order when it opened, to find the next row once it left.
 */
const answering = ref<{
  id: string
  entry: InboxEntry
  start?: ChannelStart
  order: string[]
} | null>(null)

function openDrawer(id: string, start?: ChannelStart) {
  const entry = entries.value.get(id)
  if (entry) answering.value = { id, entry, ...(start ? { start } : {}), order: waitingIds() }
}

/** The waiting row ids in their order, read from the page. */
function waitingIds(): string[] {
  return waiting.value.map((w) => w.id)
}

function rowElement(id: string): HTMLElement | null {
  return document.querySelector<HTMLElement>(`[data-row-id="${CSS.escape(id)}"]`)
}

/** Focuses the row's link, or the waiting heading when id is undefined. */
function focusRow(id: string | undefined) {
  const row = id ? rowElement(id)?.querySelector<HTMLElement>('a') : null
  const target = row ?? document.querySelector<HTMLElement>('[data-inbox-heading="waiting"]')
  target?.focus()
}

/**
 * Focuses the first row that came after `id` in `before`, else the last
 * row, else the waiting heading.
 */
function focusAfter(id: string, before: string[]) {
  const at = before.indexOf(id)
  const after = waitingIds().filter((x) => x !== id)
  focusRow(after.find((x) => before.indexOf(x) > at) ?? after[after.length - 1])
}

/** Reads the inbox again, then moves the focus past the answered row. */
async function afterAnswer(id: string, before = waitingIds()) {
  await refetch()
  await nextTick()
  focusAfter(id, before)
}

/**
 * Returns the focus to the row that opened the drawer. The drawer gives it
 * back to the button that opened it; when that is gone, the row's link
 * takes it, and when the row has left, the next row.
 */
async function restoreFocus(a: { id: string; order: string[] }) {
  await nextTick()
  const row = rowElement(a.id)
  if (!row) focusAfter(a.id, a.order)
  else if (!row.contains(document.activeElement)) focusRow(a.id)
}

function onDrawerClose() {
  const a = answering.value
  answering.value = null
  if (a) void restoreFocus(a)
}

function onDrawerSent() {
  const a = answering.value
  answering.value = null
  if (a) void afterAnswer(a.id, a.order)
}

async function onDrawerGone() {
  const a = answering.value
  answering.value = null
  await refetch()
  if (a) await restoreFocus(a)
}

const answeringLabel = computed(() =>
  answering.value ? channelLabel(answering.value.entry.channel) : ''
)

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
    >
      <template #waiting-detail="{ row }">
        <InboxWaitingItem
          v-if="entries.get(row.id)"
          :entry="entries.get(row.id)!"
          :after-post="() => afterAnswer(row.id)"
          :refetch="refetch"
          @answer="(start) => openDrawer(row.id, start)"
          @open="openDrawer(row.id)"
        />
      </template>
    </RlInbox>
    <InboxAnswerDrawer
      :entry="answering?.entry ?? null"
      :start="answering?.start"
      :label="answeringLabel"
      @close="onDrawerClose"
      @sent="onDrawerSent"
      @gone="onDrawerGone"
      @changed="refetch()"
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
