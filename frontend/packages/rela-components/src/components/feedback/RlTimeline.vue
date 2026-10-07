<script setup lang="ts">
/**
 * What happened to something, newest first: an activity feed, an audit trail, a
 * history panel.
 *
 * The counterpart to the comment components. A comment is something a person
 * wrote and can reply to; a timeline entry is something that happened and cannot
 * be answered. They belong beside each other in a detail panel and are
 * deliberately separate: giving a comment thread a "changed status" row makes
 * every reply control meaningless on half the rows.
 *
 * ## The app decides what an event means
 *
 * There is no union of event kinds here. A typed graph's vocabulary is the
 * app's — rela has relation changes and property edits, another deployment has
 * deploys and approvals — so an entry arrives with its wording, its icon and its
 * tone already chosen, and this lays them out. That is the same boundary the
 * calendar draws by taking pre-formatted times, and it is what lets this be
 * useful without the schema contract the gap analysis is still waiting on.
 *
 * ## Markup
 *
 * An ordered list, because the order is the content: these events happened in
 * this sequence, and a screen reader should say "3 of 12" as it moves. Groups
 * are nested lists under a heading rather than a flat list with separator rows,
 * so the day a thing happened on is structure rather than a visual break.
 */
import { computed } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import RlAvatar from '../data/RlAvatar.vue'
import RlText from '../common/RlText.vue'
import RlHeading from '../common/RlHeading.vue'
import type { TimelineEntry, TimelineGroup } from './timeline'

const props = withDefaults(
  defineProps<{
    /**
     * The entries, in the order they should read. Newest first is the usual
     * choice for a feed and is not enforced: an audit trail read oldest first is
     * equally valid, and reversing the caller's array would be this component
     * overriding a decision it cannot make.
     */
    entries?: TimelineEntry[]
    /**
     * The same content grouped under headings, usually by day. Takes precedence
     * over `entries`; pass one or the other.
     */
    groups?: TimelineGroup[]
    /**
     * The heading level the group labels render at, so the timeline fits the
     * outline of whatever page it is on. The visual size does not change with
     * it, which is the split `RlHeading` already draws.
     */
    headingLevel?: 2 | 3 | 4 | 5 | 6
    /**
     * Whether an entry shows its actor's avatar. Off by default: a feed where
     * every row is the same person is a column of identical circles, and the
     * name is already written.
     */
    avatars?: boolean
    /**
     * What to say when there is nothing. Left unset the timeline renders
     * nothing at all, which suits a panel that has its own empty state.
     */
    emptyLabel?: string
  }>(),
  {
    entries: () => [],
    groups: undefined,
    headingLevel: 3,
    avatars: false,
    emptyLabel: undefined,
  },
)

/*
 * One shape internally, so the template is not written twice. A flat list
 * becomes a single unlabelled group, which is the same reduction `RlTable` makes
 * for a sectionless list.
 */
const resolvedGroups = computed<TimelineGroup[]>(() =>
  props.groups ?? [{ id: 'all', entries: props.entries }],
)

const isEmpty = computed(() =>
  resolvedGroups.value.every((group) => group.entries.length === 0),
)
</script>

<template>
  <div class="rl-timeline">
    <RlText v-if="isEmpty && emptyLabel" size="sm" tone="subtle">{{ emptyLabel }}</RlText>

    <template v-else>
      <section
        v-for="group in resolvedGroups"
        :key="group.id"
        class="rl-timeline__group"
      >
        <RlHeading
          v-if="group.label"
          :level="headingLevel"
          size="sm"
          class="rl-timeline__heading"
          >{{ group.label }}</RlHeading
        >

        <ol class="rl-timeline__list">
          <li
            v-for="entry in group.entries"
            :key="entry.id"
            class="rl-timeline__entry"
            :class="{ 'rl-timeline__entry--current': entry.current }"
            :aria-current="entry.current ? 'true' : undefined"
          >
            <!--
              The rail: a marker per entry, joined by a line drawn with a
              pseudo-element so it stops at the last entry rather than running
              past it into the padding.
            -->
            <span class="rl-timeline__rail" aria-hidden="true">
              <span
                class="rl-timeline__marker"
                :class="`rl-timeline__marker--${entry.tone ?? 'grey'}`"
              >
                <RlIcon v-if="entry.icon" :name="entry.icon" :size="12" />
              </span>
            </span>

            <div class="rl-timeline__content">
              <p class="rl-timeline__line">
                <!--
                  Decorative: the actor's name follows in the text, so the
                  avatar would otherwise be read twice.
                -->
                <RlAvatar
                  v-if="avatars && entry.actor"
                  :name="entry.actor"
                  :src="entry.actorAvatarUrl"
                  size="xs"
                  decorative
                  class="rl-timeline__avatar"
                />
                <span v-if="entry.actor" class="rl-timeline__actor">{{ entry.actor }}</span>

                <!--
                  Replaces the summary, for an entry that needs markup: a link
                  to the thing that changed, or a tag showing the new status.
                -->
                <slot name="entry" :entry="entry">
                  <span v-if="entry.summary" class="rl-timeline__summary">{{ entry.summary }}</span>
                </slot>

                <!--
                  A real `<time>` when the caller passed the instant, so the
                  date behind "2 hours ago" is recoverable. Otherwise the
                  formatted string alone, since a `datetime` this component
                  guessed would be worse than none.
                -->
                <time
                  v-if="entry.timestamp && entry.datetime"
                  class="rl-timeline__timestamp"
                  :datetime="entry.datetime"
                  >{{ entry.timestamp }}</time
                >
                <span v-else-if="entry.timestamp" class="rl-timeline__timestamp">{{
                  entry.timestamp
                }}</span>
              </p>

              <slot name="detail" :entry="entry">
                <p v-if="entry.detail" class="rl-timeline__detail">{{ entry.detail }}</p>
              </slot>
            </div>
          </li>
        </ol>
      </section>
    </template>
  </div>
</template>

<style scoped>
.rl-timeline__group + .rl-timeline__group { margin-top: var(--rl-space-5); }

.rl-timeline__heading {
  /*
   * Sticky, so the day a run of events belongs to stays visible while it
   * scrolls. A long audit trail is read by scrolling, and a heading that
   * leaves the screen takes the only thing dating the rows with it.
   */
  position: sticky;
  top: 0;
  z-index: var(--rl-z-sticky);
  padding: var(--rl-space-1) 0;
  background: var(--rl-color-bg);
}

.rl-timeline__list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.rl-timeline__entry {
  display: flex;
  gap: var(--rl-space-3);
}

.rl-timeline__rail {
  position: relative;
  display: flex;
  justify-content: center;
  flex: none;
  width: 18px;
}

/*
 * The connecting line, drawn from the marker down to the next entry. On the
 * rail rather than between the markers so it cannot outlive the list: the last
 * entry's line is suppressed below, which a border on the container could not
 * do.
 */
.rl-timeline__rail::before {
  content: '';
  position: absolute;
  top: 18px;
  bottom: -2px;
  width: 1px;
  background: var(--rl-color-border);
}

.rl-timeline__entry:last-child .rl-timeline__rail::before { display: none; }

.rl-timeline__marker {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  flex: none;
  /*
   * Aligned with the first line of text rather than the top of the box, so a
   * marker sits beside its summary and not above it.
   */
  margin-top: 2px;
  border-radius: var(--rl-radius-pill);
  /*
   * A ring in the page background, so the connecting line passes behind the
   * marker rather than appearing to touch it.
   */
  box-shadow: 0 0 0 2px var(--rl-color-bg);
  color: var(--rl-color-text-inverse);
}

/*
 * The tones reuse the status palette rather than a private set, so a timeline
 * restyles with the rest of the library.
 */
.rl-timeline__marker--grey { background: var(--rl-color-status-grey); }
.rl-timeline__marker--green { background: var(--rl-color-status-green); }
.rl-timeline__marker--amber { background: var(--rl-color-status-amber); }
.rl-timeline__marker--red { background: var(--rl-color-status-red); }
.rl-timeline__marker--blue { background: var(--rl-color-status-blue); }
.rl-timeline__marker--purple { background: var(--rl-color-accent); }

.rl-timeline__content {
  min-width: 0;
  /* The gap between entries, paid below the content so the rail stays unbroken. */
  padding-bottom: var(--rl-space-4);
}

.rl-timeline__entry:last-child .rl-timeline__content { padding-bottom: 0; }

.rl-timeline__line {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--rl-space-1) var(--rl-space-2);
  margin: 0;
  font-size: var(--rl-font-size-sm);
  line-height: var(--rl-line-height-normal);
  color: var(--rl-color-text-muted);
}

.rl-timeline__avatar { flex: none; }

.rl-timeline__actor {
  font-weight: var(--rl-font-weight-medium);
  color: var(--rl-color-text);
}

.rl-timeline__timestamp { color: var(--rl-color-text-subtle); }

.rl-timeline__detail {
  margin: var(--rl-space-2) 0 0;
  font-size: var(--rl-font-size-sm);
  line-height: var(--rl-line-height-relaxed);
  color: var(--rl-color-text);
}

/* The entry a deep link points at, or the newest one. A fill rather than a
   heavier marker: the marker already carries the event's own tone. */
.rl-timeline__entry--current .rl-timeline__content {
  /* Bled into the gutter so the fill reads as a band, not a floating box. */
  margin: 0 calc(var(--rl-space-2) * -1) 0 0;
  padding: 0 var(--rl-space-2) var(--rl-space-4);
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-bg-selected);
}
</style>
