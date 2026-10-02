<script setup lang="ts">
/**
 * The full comment thread for one record, with an anchor picker on the
 * composer.
 *
 * Collapsed by default. Where comments also appear beside the thing they point
 * at — see `RlCommentIndicator` — this panel is the catch-all: the place where
 * a section-anchored or detached comment lives, because neither has a field
 * row of its own to sit on. The header summary says how many of those there
 * are, so the count is not a mystery when the panel is folded away.
 *
 * Presentational: the caller owns the comments and applies every change. The
 * composer is always offered, because whether this user may post is the
 * server's answer to give, and a hidden box would claim a refusal we were
 * never told about.
 */
import { computed, ref, watch } from 'vue'
import RlCount from '../common/RlCount.vue'
import RlIcon from '../common/RlIcon.vue'
import RlButton from '../common/RlButton.vue'
import RlEmptyState from '../feedback/RlEmptyState.vue'
import RlCommentThread from './RlCommentThread.vue'
import type { AnchoredComment, CommentAnchorOption, NewComment } from './types'

const props = withDefaults(
  defineProps<{
    comments: AnchoredComment[]
    /** Anchors a new comment may be attached to. Empty hides the picker. */
    anchorOptions?: CommentAnchorOption[]
    title?: string
    /** Open on mount, for a view where comments are the point. */
    defaultExpanded?: boolean
    /** Blocks the composer and shows its pending state. */
    submitting?: boolean
  }>(),
  { anchorOptions: () => [], title: 'All comments', defaultExpanded: false, submitting: false },
)

const emit = defineEmits<{
  add: [comment: NewComment]
  edit: [id: string, body: string]
  toggleResolved: [id: string]
  remove: [id: string]
}>()

const expanded = ref(props.defaultExpanded)
const body = ref('')
const anchorKey = ref('')

const unresolvedCount = computed(() => props.comments.filter((c) => !c.resolved).length)

/*
 * Comments with nowhere else to appear. These are the reason the panel exists
 * once per-field indicators are in play: a section anchor names a heading and
 * a detached one points at something gone, so neither can render beside a
 * field.
 */
const homelessCount = computed(
  () => props.comments.filter((c) => c.anchor.kind !== 'property' || c.detached).length,
)

const selectedAnchor = computed(() =>
  props.anchorOptions.find((option) => option.key === anchorKey.value),
)

const canSubmit = computed(
  () => Boolean(body.value.trim()) && !props.submitting && Boolean(selectedAnchor.value),
)

/*
 * Default to the first anchor, so posting never requires touching the picker
 * first. Re-runs when the options change, since the previous selection may no
 * longer exist.
 */
watch(
  () => props.anchorOptions,
  (options) => {
    if (!options.some((option) => option.key === anchorKey.value)) {
      anchorKey.value = options[0]?.key ?? ''
    }
  },
  { immediate: true },
)

function submit() {
  const anchor = selectedAnchor.value
  if (!anchor || !canSubmit.value) return
  emit('add', { anchor: anchor.anchor, body: body.value.trim() })
  body.value = ''
}
</script>

<template>
  <section class="rl-comments-panel">
    <h2 class="rl-comments-panel__heading">
      <button
        type="button"
        class="rl-comments-panel__toggle"
        :aria-expanded="expanded"
        @click="expanded = !expanded"
      >
        <RlIcon
          class="rl-comments-panel__chevron"
          :name="expanded ? 'chevron-up' : 'chevron-right'"
          :size="16"
        />
        <span class="rl-comments-panel__title">{{ title }}</span>
        <span v-if="unresolvedCount > 0" class="rl-comments-panel__open">
          {{ unresolvedCount }} open
        </span>
        <span class="rl-comments-panel__summary">
          <RlCount :value="comments.length" />
          <template v-if="homelessCount > 0"> &middot; {{ homelessCount }} not on a field</template>
        </span>
      </button>
    </h2>

    <div v-if="expanded" class="rl-comments-panel__body">
      <RlCommentThread
        v-if="comments.length > 0"
        :comments="comments"
        show-anchor
        @edit="(id, text) => $emit('edit', id, text)"
        @toggle-resolved="(id) => $emit('toggleResolved', id)"
        @remove="(id) => $emit('remove', id)"
      />
      <RlEmptyState v-else title="No comments yet" description="Start the thread below." />

      <form class="rl-comments-panel__composer" @submit.prevent="submit">
        <select
          v-if="anchorOptions.length > 0"
          v-model="anchorKey"
          class="rl-comments-panel__anchor"
          aria-label="What this comment is about"
        >
          <option v-for="option in anchorOptions" :key="option.key" :value="option.key">
            {{ option.label }}
          </option>
        </select>

        <textarea
          v-model="body"
          class="rl-comments-panel__input"
          rows="3"
          placeholder="Add a comment..."
          aria-label="Comment body"
          @keydown.meta.enter.prevent="submit"
          @keydown.ctrl.enter.prevent="submit"
        />

        <RlButton
          variant="primary"
          size="sm"
          type="submit"
          class="rl-comments-panel__submit"
          :disabled="!canSubmit"
          :loading="submitting"
          loading-label="Posting comment"
        >
          Add comment
        </RlButton>
      </form>
    </div>
  </section>
</template>

<style scoped>
.rl-comments-panel {
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-bg);
  overflow: hidden;
}

.rl-comments-panel__heading { margin: 0; }

.rl-comments-panel__toggle {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  width: 100%;
  padding: var(--rl-space-3) var(--rl-page-gutter-right) var(--rl-space-3) var(--rl-page-gutter-left);
  border: none;
  background: transparent;
  font: inherit;
  text-align: left;
  color: var(--rl-color-text);
  cursor: pointer;
}

.rl-comments-panel__toggle:hover { background: var(--rl-color-bg-hover); }

.rl-comments-panel__chevron {
  flex: none;
  color: var(--rl-color-text-subtle);
}

.rl-comments-panel__title {
  font-size: var(--rl-font-size-md);
  font-weight: 600;
}

.rl-comments-panel__open {
  padding: 0 var(--rl-space-2);
  border-radius: var(--rl-radius-pill);
  background: var(--rl-color-accent);
  color: var(--rl-color-text-inverse);
  font-size: var(--rl-font-size-xs);
  font-weight: 500;
}

/* Pushed to the right edge, so the counts read as a summary of what is folded
 * away rather than as part of the title. */
.rl-comments-panel__summary {
  display: flex;
  align-items: baseline;
  gap: var(--rl-space-1);
  margin-left: auto;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-subtle);
}

.rl-comments-panel__body { border-top: 1px solid var(--rl-color-border); }

.rl-comments-panel__composer {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--rl-space-2);
  padding: var(--rl-space-4) var(--rl-page-gutter-right) var(--rl-space-4) var(--rl-page-gutter-left);
  border-top: 1px solid var(--rl-color-border);
  background: var(--rl-color-bg-sunken);
}

.rl-comments-panel__anchor,
.rl-comments-panel__input {
  width: 100%;
  padding: var(--rl-space-2) var(--rl-space-3);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-bg);
  font-family: inherit;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text);
}

.rl-comments-panel__input { resize: vertical; }
.rl-comments-panel__input::placeholder { color: var(--rl-color-text-subtle); }

.rl-comments-panel__anchor:focus-visible,
.rl-comments-panel__input:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -1px;
  border-color: var(--rl-color-accent);
}
</style>
