<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, useTemplateRef } from 'vue'
import { useUIStore } from '@/stores'
import { useConfirm } from '@/composables/useConfirm'
import { addComment, updateComment, deleteComment, type Comment } from '@/api/comments'
import { getErrorMessage } from '@/api/errors'
import SuggestionDiff from './SuggestionDiff.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'

/**
 * The thread for a text-anchored comment, opened by clicking its highlight
 * (TKT-FIO205 stage 2).
 *
 * Separate from CommentIndicator because a highlight has no component of its
 * own: the marks are inserted into the body's `v-html` output, so the parent
 * owns both the click state and the placement, and passes them here.
 */
const props = defineProps<{
  entityType: string
  entityId: string
  /** The clicked highlight's comments. */
  comments: Comment[]
  /** Placement in coordinates relative to the body element. */
  position: { top: number; left: number }
  /**
   * Whether this user may apply suggestions right now: the entity's
   * `_actions.update`, and no other body write in flight. Combined with each
   * comment's own `acceptable` hint.
   */
  canAccept?: boolean
}>()

// `accept` goes to the parent rather than being handled here: the parent owns
// the body's pending writes, which must settle before the accept is sent.
const emit = defineEmits<{ changed: []; close: []; accept: [comment: Comment] }>()

const uiStore = useUIStore()
const { confirm } = useConfirm()

const editingId = ref<string | null>(null)
const editBody = ref('')
const rootRef = useTemplateRef<HTMLElement>('root')

// True while a delete is being confirmed or sent. The confirm dialog lives
// outside the popover, so its button click would otherwise close the popover;
// the parent unmounts it, and the later `changed` is dropped, leaving the
// deleted thread highlighted until a reload.
let deleting = false

const replyBody = ref('')
const replying = ref(false)

/**
 * Posts a reply against the SAME anchor as the comment being replied to.
 *
 * Stage 1 has no threading — replies are separate comments that happen to share
 * an anchor, which is why this re-sends the original's quote and context rather
 * than a parent id. That keeps the reply pinned to the same text even after the
 * body is edited, since it re-resolves independently.
 */
async function submitReply() {
  const text = replyBody.value.trim()
  const first = props.comments[0]
  if (!text || !first || replying.value) return

  replying.value = true
  try {
    await addComment(props.entityType, props.entityId, {
      anchor: {
        kind: 'text',
        ref: '',
        quote: first.anchor.quote,
        // The stored quote is SOURCE text, so no rendered-text context applies;
        // it is already unique enough to have resolved once.
      },
      body: text,
    })
    replyBody.value = ''
    emit('changed')
  } catch (err) {
    uiStore.error(getErrorMessage(err))
  } finally {
    replying.value = false
  }
}

function startEdit(c: Comment) {
  editingId.value = c.id
  editBody.value = c.body
}

async function saveEdit(c: Comment) {
  const text = editBody.value.trim()
  if (!text) return
  try {
    await updateComment(props.entityType, props.entityId, c.id, { body: text })
    editingId.value = null
    emit('changed')
  } catch (err) {
    uiStore.error(getErrorMessage(err))
  }
}

async function toggleResolved(c: Comment) {
  try {
    await updateComment(props.entityType, props.entityId, c.id, { resolved: !c.resolved })
    emit('changed')
  } catch (err) {
    uiStore.error(getErrorMessage(err))
  }
}

async function remove(c: Comment) {
  // Branch on the boolean: useConfirm resolves false if the shell unmounts
  // while the dialog is open, and a DELETE must not fire on that path.
  deleting = true
  try {
    const ok = await confirm({
      title: 'Delete comment',
      message: 'Delete this comment? This cannot be undone.',
      confirmLabel: 'Delete',
      danger: true,
    })
    if (!ok) return
    await deleteComment(props.entityType, props.entityId, c.id)
    emit('changed')
    emit('close')
  } catch (err) {
    uiStore.error(getErrorMessage(err))
  } finally {
    deleting = false
  }
}

function onDocClick(e: MouseEvent) {
  const target = e.target as HTMLElement | null
  // A click on a highlight is the parent's to interpret — it may be opening a
  // different thread, and closing here first would fight that.
  if (deleting || target?.closest('mark[data-comment-id]')) return
  if (!rootRef.value?.contains(target)) emit('close')
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && !deleting) emit('close')
}

onMounted(() => {
  document.addEventListener('click', onDocClick)
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('keydown', onKeydown)
})

function formatDate(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}
</script>

<template>
  <div
    ref="root"
    class="tcp"
    :style="{ top: `${position.top}px`, left: `${position.left}px` }"
    @click.stop
  >
    <header class="tcp-head">
      <span class="tcp-title">Comment on selection</span>
      <button type="button" class="tcp-x" aria-label="Close" @click="emit('close')">✕</button>
    </header>

    <ul class="tcp-list">
      <li
        v-for="c in comments"
        :key="c.id"
        class="tcp-cmt"
        :class="{ 'tcp-cmt--resolved': c.resolved }"
      >
        <SuggestionDiff
          v-if="c.anchor.quote && c.anchor.replacement != null"
          :quote="c.anchor.quote"
          :replacement="c.anchor.replacement"
        />
        <blockquote v-else-if="c.anchor.quote" class="tcp-quote">{{ c.anchor.quote }}</blockquote>
        <div class="tcp-meta">
          <b>{{ c.author }}</b>
          <span>{{ formatDate(c.created_at) }}</span>
          <span v-if="c.anchor.uncertain" class="tcp-flag" title="The text may have moved">
            may have moved
          </span>
          <span
            v-if="c.detached"
            class="tcp-flag tcp-flag--detached"
            title="The quoted text is gone"
          >
            detached
          </span>
        </div>

        <template v-if="editingId === c.id">
          <textarea v-model="editBody" class="tcp-input" rows="3" />
          <div class="tcp-acts">
            <RlButton variant="primary" size="sm" @click="saveEdit(c)">Save</RlButton>
            <RlButton variant="secondary" size="sm" @click="editingId = null">Cancel</RlButton>
          </div>
        </template>

        <template v-else>
          <p class="tcp-body">{{ c.body }}</p>
          <div class="tcp-acts">
            <RlButton v-if="c.acceptable && canAccept" variant="primary" size="sm" @click="emit('accept', c)">
              Accept
            </RlButton>
            <RlButton v-if="c.editable" variant="secondary" size="sm" @click="toggleResolved(c)">
              {{ c.resolved ? 'Reopen' : 'Resolve' }}
            </RlButton>
            <RlButton v-if="c.editable" variant="secondary" size="sm" @click="startEdit(c)">Edit</RlButton>
            <RlButton v-if="c.deletable" variant="secondary" size="sm" tone="danger" @click="remove(c)">
              Delete
            </RlButton>
          </div>
        </template>
      </li>
    </ul>

    <!-- Replies are separate comments sharing this anchor: stage 1 has no
         threading, so they re-resolve independently against the same text. -->
    <form class="tcp-reply" @submit.prevent="submitReply">
      <textarea
        v-model="replyBody"
        class="tcp-input"
        rows="2"
        placeholder="Reply…"
        aria-label="Reply"
        @keydown.meta.enter="submitReply"
        @keydown.ctrl.enter="submitReply"
      />
      <div class="tcp-reply-row">
        <span class="tcp-hint">⌘↵ to post</span>
        <RlButton
          type="submit"
          variant="primary"
          size="sm"
          :disabled="replying || !replyBody.trim()"
        >
          {{ replying ? 'Posting…' : 'Reply' }}
        </RlButton>
      </div>
    </form>
  </div>
</template>

<style scoped>
.tcp {
  position: absolute;
  z-index: 26;
  width: 340px;
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--radius-lg, 8px);
  box-shadow: var(--shadow-lg, 0 10px 30px rgb(0 0 0 / 16%));
  text-align: left;
}

.tcp-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-bottom: 1px solid var(--rl-color-border);
}
.tcp-title {
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}
.tcp-x {
  border: 0;
  background: none;
  cursor: pointer;
  color: var(--rl-color-text-muted);
  font-size: var(--font-size-sm);
}

.tcp-list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 340px;
  overflow-y: auto;
}
.tcp-cmt {
  padding: 10px 12px;
  border-bottom: 1px solid var(--rl-color-border);
}
.tcp-cmt:last-child {
  border-bottom: 0;
}
.tcp-cmt--resolved {
  opacity: 0.6;
}

/* The anchored text, so the thread states what it is about even when the
 * highlight is scrolled out of view or has detached. */
.tcp-quote {
  margin: 0 0 6px;
  padding: 3px 8px;
  border-left: 3px solid var(--rl-color-accent);
  background: var(--rl-color-bg);
  color: var(--rl-color-text-muted);
  font-size: var(--font-size-sm);
  font-style: italic;
  max-height: 3.4em;
  overflow: hidden;
}

.tcp-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
  margin-bottom: 3px;
}
.tcp-meta b {
  color: var(--rl-color-text);
}
.tcp-flag {
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--rl-color-status-amber);
  color: var(--rl-color-text);
}
.tcp-flag--detached {
  background: var(--rl-color-danger);
  color: #fff;
}

.tcp-body {
  margin: 0 0 6px;
  font-size: var(--font-size-base);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.tcp-acts {
  display: flex;
  gap: 5px;
}

.tcp-reply {
  padding: 10px 12px;
  border-top: 1px solid var(--rl-color-border);
  background: var(--rl-color-bg);
}
.tcp-reply-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 6px;
}
.tcp-hint {
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}

.tcp-input {
  width: 100%;
  padding: 6px 8px;
  resize: vertical;
  border: 1px solid var(--rl-color-border);
  border-radius: var(--radius-sm, 5px);
  background: var(--rl-color-bg-raised);
  color: var(--rl-color-text);
  font: inherit;
  font-size: var(--font-size-base);
}
.tcp-input:focus {
  outline: none;
  border-color: var(--rl-color-accent);
  box-shadow:
    0 0 0 2px var(--rl-color-bg),
    0 0 0 4px var(--rl-color-focus);
}

@media (max-width: 640px) {
  .tcp {
    left: 0;
    right: 0;
    width: auto;
  }
}
</style>
