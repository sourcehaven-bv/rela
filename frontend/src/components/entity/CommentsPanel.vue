<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useSchemaStore, useUIStore } from '@/stores'
import {
  listComments,
  addComment,
  updateComment,
  deleteComment,
  type Comment,
  type CommentAnchor,
} from '@/api/comments'
import { getErrorMessage } from '@/api/errors'
import { useConfirm } from '@/composables/useConfirm'

/**
 * The entity's comment thread (TKT-FIO205, stage 1).
 *
 * Renders only when the type is `commentable` in the schema. That flag is
 * policy — "commenting is possible here" — never permission: the server
 * re-authorizes every call, so the panel may still be present for a user who
 * cannot post, and a 403 is the honest answer rather than something the UI
 * pretends to prevent.
 */
const props = defineProps<{
  entityType: string
  entityId: string
  /**
   * Section ids from the entity's configured view, offered as anchor targets
   * alongside the type's properties.
   */
  sectionIds?: string[]
}>()

const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const { confirm } = useConfirm()

const comments = ref<Comment[]>([])
const loading = ref(false)
/**
 * Set when the thread could not be loaded. Distinct from "no comments": a
 * failed load must not render as an empty thread, or a permissions problem
 * looks like a clean slate.
 */
const loadError = ref<string | null>(null)

const newBody = ref('')
const newAnchorKey = ref('')
const submitting = ref(false)

const editingId = ref<string | null>(null)
const editBody = ref('')

const typeDef = computed(() => schemaStore.getEntityType(props.entityType))

/** Commenting is possible for this type. The server decides who may act. */
const commentable = computed(() => typeDef.value?.commentable === true)

/**
 * The anchor targets a user can attach a comment to: every declared property,
 * then any section the view configures.
 *
 * Keyed as `kind:ref` so the two namespaces cannot collide — a property and a
 * section may legitimately share a name.
 */
interface AnchorOption {
  key: string
  label: string
  anchor: CommentAnchor
}

const anchorOptions = computed<AnchorOption[]>(() => {
  const options: AnchorOption[] = []
  const properties = typeDef.value?.properties ?? {}
  for (const name of Object.keys(properties)) {
    options.push({ key: `property:${name}`, label: name, anchor: { kind: 'property', ref: name } })
  }
  for (const id of props.sectionIds ?? []) {
    options.push({
      key: `section:${id}`,
      label: `Section: ${id}`,
      anchor: { kind: 'section', ref: id },
    })
  }
  return options
})

/** Unresolved first, so an active thread is not buried under settled remarks. */
const sortedComments = computed(() =>
  [...comments.value].sort((a, b) => Number(a.resolved) - Number(b.resolved))
)

const unresolvedCount = computed(() => comments.value.filter((c) => !c.resolved).length)

function anchorLabel(anchor: CommentAnchor): string {
  return anchor.kind === 'section' ? `Section: ${anchor.ref}` : anchor.ref
}

function formatDate(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}

async function load() {
  if (!commentable.value) return
  loading.value = true
  loadError.value = null
  try {
    comments.value = await listComments(props.entityType, props.entityId)
  } catch (err) {
    // A 404 here means "cannot read the target, or commenting is off" — the
    // server makes those indistinguishable on purpose. Either way there is no
    // thread to show, so report it rather than rendering an empty one.
    comments.value = []
    loadError.value = getErrorMessage(err)
  } finally {
    loading.value = false
  }
}

async function submit() {
  const option = anchorOptions.value.find((o) => o.key === newAnchorKey.value)
  if (!option || !newBody.value.trim() || submitting.value) return

  submitting.value = true
  try {
    const created = await addComment(props.entityType, props.entityId, {
      anchor: option.anchor,
      body: newBody.value.trim(),
    })
    comments.value = [...comments.value, created]
    newBody.value = ''
  } catch (err) {
    uiStore.error(getErrorMessage(err))
  } finally {
    submitting.value = false
  }
}

function startEdit(comment: Comment) {
  editingId.value = comment.id
  editBody.value = comment.body
}

function cancelEdit() {
  editingId.value = null
  editBody.value = ''
}

/**
 * Applies a change and reflects it locally.
 *
 * PATCH returns 204, so there is no server copy to swap in; the local record
 * is patched from what we sent. Any field the server would have changed
 * independently is picked up by the next load.
 */
async function applyUpdate(comment: Comment, patch: { body?: string; resolved?: boolean }) {
  try {
    await updateComment(props.entityType, props.entityId, comment.id, patch)
    comments.value = comments.value.map((c) => (c.id === comment.id ? { ...c, ...patch } : c))
  } catch (err) {
    uiStore.error(getErrorMessage(err))
  }
}

async function saveEdit(comment: Comment) {
  const body = editBody.value.trim()
  if (!body) return
  await applyUpdate(comment, { body })
  cancelEdit()
}

async function toggleResolved(comment: Comment) {
  await applyUpdate(comment, { resolved: !comment.resolved })
}

async function remove(comment: Comment) {
  // Branch on the boolean: useConfirm resolves false if the shell unmounts
  // while the dialog is open, and a DELETE must not fire on that path.
  const ok = await confirm({
    title: 'Delete comment',
    message: 'Delete this comment? This cannot be undone.',
    confirmLabel: 'Delete',
    danger: true,
  })
  if (!ok) return

  try {
    await deleteComment(props.entityType, props.entityId, comment.id)
    comments.value = comments.value.filter((c) => c.id !== comment.id)
  } catch (err) {
    uiStore.error(getErrorMessage(err))
  }
}

// Reload when the panel points at a different entity. `immediate` covers the
// initial mount, so there is no separate onMounted doing the same call.
watch(
  () => [props.entityType, props.entityId, commentable.value] as const,
  () => {
    comments.value = []
    cancelEdit()
    void load()
  },
  { immediate: true }
)

// Default the anchor picker to the first available target once the schema has
// loaded, so posting a comment never requires touching the select first.
watch(
  anchorOptions,
  (options) => {
    if (!options.some((o) => o.key === newAnchorKey.value)) {
      newAnchorKey.value = options[0]?.key ?? ''
    }
  },
  { immediate: true }
)

defineExpose({ load })
</script>

<template>
  <section v-if="commentable" class="comments-panel">
    <header class="panel-header">
      <h2>
        Comments
        <span v-if="unresolvedCount > 0" class="unresolved-count">{{ unresolvedCount }} open</span>
      </h2>
    </header>

    <div v-if="loading" class="comments-state">Loading comments…</div>

    <div v-else-if="loadError" class="comments-state comments-error">{{ loadError }}</div>

    <template v-else>
      <ul v-if="sortedComments.length > 0" class="comment-list">
        <li
          v-for="comment in sortedComments"
          :key="comment.id"
          class="comment"
          :class="{ 'comment-resolved': comment.resolved }"
        >
          <div class="comment-meta">
            <span class="comment-anchor">{{ anchorLabel(comment.anchor) }}</span>
            <span
              v-if="comment.detached"
              class="comment-detached"
              title="The property this comment points at no longer exists on this entity"
            >
              detached
            </span>
            <span class="comment-author">{{ comment.author }}</span>
            <span class="comment-date">{{ formatDate(comment.created_at) }}</span>
          </div>

          <div v-if="editingId === comment.id" class="comment-edit">
            <textarea v-model="editBody" class="comment-input" rows="3" />
            <div class="comment-actions">
              <button class="btn btn-sm btn-primary" @click="saveEdit(comment)">Save</button>
              <button class="btn btn-sm btn-secondary" @click="cancelEdit">Cancel</button>
            </div>
          </div>

          <template v-else>
            <p class="comment-body">{{ comment.body }}</p>
            <div class="comment-actions">
              <button
                v-if="comment.editable"
                class="btn btn-sm btn-secondary"
                @click="toggleResolved(comment)"
              >
                {{ comment.resolved ? 'Reopen' : 'Resolve' }}
              </button>
              <button
                v-if="comment.editable"
                class="btn btn-sm btn-secondary"
                @click="startEdit(comment)"
              >
                Edit
              </button>
              <button
                v-if="comment.deletable"
                class="btn btn-sm btn-danger"
                @click="remove(comment)"
              >
                Delete
              </button>
            </div>
          </template>
        </li>
      </ul>

      <div v-else class="comments-state">No comments yet.</div>

      <!-- Always offered: whether this user may post is the server's call, and
           a hidden box would misreport a permission we have not been told. -->
      <form class="comment-form" @submit.prevent="submit">
        <select v-model="newAnchorKey" class="anchor-select" aria-label="Comment anchor">
          <option v-for="option in anchorOptions" :key="option.key" :value="option.key">
            {{ option.label }}
          </option>
        </select>
        <textarea
          v-model="newBody"
          class="comment-input"
          rows="3"
          placeholder="Add a comment…"
          aria-label="Comment body"
        />
        <button
          type="submit"
          class="btn btn-sm btn-primary"
          :disabled="submitting || !newBody.trim() || !newAnchorKey"
        >
          {{ submitting ? 'Adding…' : 'Add comment' }}
        </button>
      </form>
    </template>
  </section>
</template>

<style scoped>
.comments-panel {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  margin-bottom: 24px;
  overflow: hidden;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border-color);
}

.panel-header h2 {
  margin: 0;
  font-size: 18px;
  color: var(--text-color);
  display: flex;
  align-items: center;
  gap: 12px;
}

.unresolved-count {
  font-size: 12px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 10px;
  background: var(--accent-color);
  color: var(--accent-contrast-text, #fff);
}

.comments-state {
  padding: 16px 24px;
  color: var(--muted-text);
  font-size: 14px;
}

.comments-error {
  color: var(--error-color, #c00);
}

.comment-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.comment {
  padding: 16px 24px;
  border-bottom: 1px solid var(--border-color);
}

.comment-resolved {
  opacity: 0.6;
}

.comment-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 12px;
  color: var(--muted-text);
  margin-bottom: 8px;
}

.comment-anchor {
  font-family: var(--mono-font, monospace);
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--input-bg);
  color: var(--text-color);
}

.comment-detached {
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--warning-color, #b58900);
  color: #fff;
}

.comment-author {
  font-weight: 600;
  color: var(--text-color);
}

.comment-body {
  margin: 0 0 8px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  color: var(--text-color);
}

.comment-actions {
  display: flex;
  gap: 8px;
}

.comment-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px 24px;
  border-top: 1px solid var(--border-color);
}

.comment-input,
.anchor-select {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--input-bg);
  color: var(--text-color);
  font-size: 14px;
  font-family: inherit;
}

.comment-input:focus,
.anchor-select:focus {
  outline: none;
  border-color: var(--accent-color);
  box-shadow:
    0 0 0 2px var(--focus-ring-gap),
    0 0 0 4px var(--focus-ring);
}

/* `.btn`, `.btn-sm` and the variants are global (unscoped, App.vue) — only the
 * panel-specific placement is declared here. Redeclaring them scoped would
 * compile to a [data-v-*] selector that outranks and silently forks the themed
 * originals. */
.comment-form .btn {
  align-self: flex-start;
}
</style>
