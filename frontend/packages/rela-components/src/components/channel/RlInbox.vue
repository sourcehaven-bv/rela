<script setup lang="ts">
/**
 * The user's inbox: the sessions that wait on the user, the unread mentions
 * and replies, and the channels with unread messages.
 *
 * Presentational only. The app fetches, formats dates and builds each row's
 * link (`as` plus `attrs`, like a nav item), so the library needs no router.
 * Each section shows an empty state when it has no rows. Mention and unread
 * rows offer "Mark read", which emits `markRead` with the row id; a row id in
 * `busy` shows that button as working.
 */
import { useId } from 'vue'
import RlButton from '../common/RlButton.vue'
import RlHeading from '../common/RlHeading.vue'
import RlIcon from '../common/RlIcon.vue'
import RlCallout from '../feedback/RlCallout.vue'
import RlNavStatus from '../layout/RlNavStatus.vue'
import type { NavItemStatus } from '../../types'
import type { InboxMentionRow, InboxUnreadRow, InboxWaitingRow } from './types'
import { useMessages } from '../../composables/useMessages'

withDefaults(
  defineProps<{
    waiting: InboxWaitingRow[]
    mentions: InboxMentionRow[]
    unread: InboxUnreadRow[]
    /** Set when the waiting list holds fewer rows than exist. `total` is preformatted. */
    waitingMore?: { shown: number; total: string }
    /** The server could not load every channel setting. */
    degraded?: boolean
    /** Row ids whose mark read is in flight. */
    busy?: Set<string>
    /** The level of the section headings, so the page keeps its outline. */
    headingLevel?: 2 | 3 | 4 | 5 | 6
  }>(),
  { waitingMore: undefined, degraded: false, busy: () => new Set(), headingLevel: 2 }
)

const emit = defineEmits<{ markRead: [rowId: string] }>()

const messages = useMessages()
const idBase = useId()

function unreadStatus(count: number): NavItemStatus {
  return { tone: 'new', label: messages.channelUnreadCount({ count }), count }
}
</script>

<template>
  <div class="rl-inbox">
    <RlCallout v-if="degraded" tone="warning">{{ messages.inboxDegraded() }}</RlCallout>

    <section class="rl-inbox__section" :aria-labelledby="`${idBase}-waiting`">
      <RlHeading :id="`${idBase}-waiting`" :level="headingLevel" size="sm" weight="semibold">
        {{ messages.inboxWaitingHeading() }}
      </RlHeading>
      <ul v-if="waiting.length" class="rl-inbox__list">
        <li v-for="row in waiting" :key="row.id" class="rl-inbox__row" data-section="waiting">
          <component :is="row.link.as ?? 'a'" v-bind="row.link.attrs" class="rl-inbox__link">
            <RlIcon name="warning" :size="16" class="rl-inbox__icon rl-inbox__icon--waiting" />
            <span class="rl-inbox__main">
              <span class="rl-inbox__title">{{ row.anchorTitle }}</span>
              <span class="rl-inbox__meta"
                >{{ row.entityType }} · {{ row.channel }} · {{ row.state }}</span
              >
            </span>
            <time class="rl-inbox__time" :datetime="row.sinceIso">{{ row.since }}</time>
          </component>
        </li>
      </ul>
      <p v-else class="rl-inbox__empty">{{ messages.inboxWaitingEmpty() }}</p>
      <p v-if="waitingMore" class="rl-inbox__more">
        {{ messages.inboxShowing({ shown: waitingMore.shown, total: waitingMore.total }) }}
      </p>
    </section>

    <section class="rl-inbox__section" :aria-labelledby="`${idBase}-mentions`">
      <RlHeading :id="`${idBase}-mentions`" :level="headingLevel" size="sm" weight="semibold">
        {{ messages.inboxMentionsHeading() }}
      </RlHeading>
      <ul v-if="mentions.length" class="rl-inbox__list">
        <li v-for="row in mentions" :key="row.id" class="rl-inbox__row" data-section="mentions">
          <component :is="row.link.as ?? 'a'" v-bind="row.link.attrs" class="rl-inbox__link">
            <RlIcon
              :name="row.kind === 'reply' ? 'message' : 'discussion'"
              :size="16"
              class="rl-inbox__icon"
            />
            <span class="rl-inbox__main">
              <span class="rl-inbox__title">
                {{
                  row.kind === 'reply'
                    ? messages.inboxReplied({ author: row.author })
                    : messages.inboxMentioned({ author: row.author })
                }}
              </span>
              <span class="rl-inbox__meta">{{ row.channel }} · {{ row.anchorTitle }}</span>
            </span>
            <time class="rl-inbox__time" :datetime="row.atIso">{{ row.at }}</time>
          </component>
          <RlButton
            variant="ghost"
            size="sm"
            :loading="busy.has(row.id)"
            :pending-label="messages.channelText({ key: 'working' })"
            @click="emit('markRead', row.id)"
          >
            {{ messages.inboxMarkRead() }}
          </RlButton>
        </li>
      </ul>
      <p v-else class="rl-inbox__empty">{{ messages.inboxMentionsEmpty() }}</p>
    </section>

    <section class="rl-inbox__section" :aria-labelledby="`${idBase}-unread`">
      <RlHeading :id="`${idBase}-unread`" :level="headingLevel" size="sm" weight="semibold">
        {{ messages.inboxUnreadHeading() }}
      </RlHeading>
      <ul v-if="unread.length" class="rl-inbox__list">
        <li v-for="row in unread" :key="row.id" class="rl-inbox__row" data-section="unread">
          <component :is="row.link.as ?? 'a'" v-bind="row.link.attrs" class="rl-inbox__link">
            <RlIcon name="discussion" :size="16" class="rl-inbox__icon" />
            <span class="rl-inbox__main">
              <span class="rl-inbox__title">{{ row.channel }}</span>
              <span class="rl-inbox__meta">{{ row.anchorTitle }}</span>
            </span>
            <RlNavStatus v-if="row.unread > 0" :status="unreadStatus(row.unread)" />
          </component>
          <RlButton
            variant="ghost"
            size="sm"
            :loading="busy.has(row.id)"
            :pending-label="messages.channelText({ key: 'working' })"
            @click="emit('markRead', row.id)"
          >
            {{ messages.inboxMarkRead() }}
          </RlButton>
        </li>
      </ul>
      <p v-else class="rl-inbox__empty">{{ messages.inboxUnreadEmpty() }}</p>
    </section>
  </div>
</template>

<style scoped>
.rl-inbox {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-5);
  color: var(--rl-color-text);
  font-size: var(--rl-font-size-sm);
}

.rl-inbox__section {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
}

.rl-inbox__list {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  overflow: hidden;
}

.rl-inbox__row {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  padding-right: var(--rl-space-2);
}

.rl-inbox__row + .rl-inbox__row {
  border-top: 1px solid var(--rl-color-border);
}

.rl-inbox__link {
  display: flex;
  flex: 1;
  align-items: center;
  gap: var(--rl-space-3);
  min-width: 0;
  padding: var(--rl-space-2) var(--rl-space-3);
  color: inherit;
  text-decoration: none;
}

.rl-inbox__link:hover {
  background: var(--rl-color-bg-hover);
}

.rl-inbox__link:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -2px;
}

.rl-inbox__icon {
  flex: none;
  color: var(--rl-color-text-muted);
}
.rl-inbox__icon--waiting {
  color: var(--rl-color-warning-fg);
}

.rl-inbox__main {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
}

.rl-inbox__title {
  overflow: hidden;
  font-weight: var(--rl-font-weight-semibold);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-inbox__meta {
  overflow: hidden;
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-xs);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-inbox__time {
  flex: none;
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-xs);
}

.rl-inbox__empty,
.rl-inbox__more {
  margin: 0;
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-xs);
}
</style>
