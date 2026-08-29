<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import type { WidgetProps } from './types'
import type { AttachmentInfo } from '@/types'
import { uploadAttachment, deleteAttachment, AttachmentError } from '@/api/attachments'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  // Fired after a successful upload or delete so the parent can refresh
  // the entity (the property value and _attachments changed server-side).
  'attachment-changed': []
  // Create mode only: the staged (not-yet-uploaded) file list for this
  // property changed. The host form owns the list — see `stagedFiles`.
  'update:staged-files': [files: File[]]
}>()

const files = computed<AttachmentInfo[]>(() => props.attachments ?? [])
const staged = computed<File[]>(() => props.stagedFiles ?? [])
const maxCount = computed(() => props.max ?? 1)
const isSingle = computed(() => maxCount.value <= 1)
// Capacity counts BOTH persisted and staged files: in create mode every file
// is staged, and a form could in principle show both.
const atCapacity = computed(() => files.value.length + staged.value.length >= maxCount.value)

// Edit mode can mutate only when the widget knows the owning entity and
// isn't disabled by ACL.
const canEdit = computed(
  () => props.mode === 'edit' && !props.disabled && !!props.entityType && !!props.entityId
)
// Create mode (TKT-7K3BJF): no entity id exists yet, so a pick is STAGED
// rather than uploaded. An attachment cannot be written before the entity
// row exists, so the host form uploads these after the create returns an id.
const canStage = computed(() => props.mode === 'edit' && !props.disabled && !props.entityId)
// The add control shows when editing/staging and there's room (single-cap:
// shows as "Replace" once a file exists; multi-cap: hidden at capacity).
const canAdd = computed(
  () => (canEdit.value || canStage.value) && (isSingle.value || !atCapacity.value)
)

const busy = ref(false)
const progress = ref(0)
const uploadError = ref('')

// Object URLs for staged image previews, keyed by File. Revoked on removal
// and on unmount — an un-revoked blob URL pins its bytes for the life of the
// document.
const stagedPreviews = ref(new Map<File, string>())

function stagedPreviewUrl(file: File): string | undefined {
  if (!file.type.startsWith('image/')) return undefined
  let url = stagedPreviews.value.get(file)
  if (!url) {
    url = URL.createObjectURL(file)
    stagedPreviews.value.set(file, url)
  }
  return url
}

function revokeStagedPreview(file: File) {
  const url = stagedPreviews.value.get(file)
  if (url) {
    URL.revokeObjectURL(url)
    stagedPreviews.value.delete(file)
  }
}

// Revoke previews for files that are no longer staged (e.g. the host cleared
// the list after a successful create), so the map can't outlive its files.
watch(staged, (current) => {
  for (const file of [...stagedPreviews.value.keys()]) {
    if (!current.includes(file)) revokeStagedPreview(file)
  }
})

onBeforeUnmount(() => {
  for (const url of stagedPreviews.value.values()) URL.revokeObjectURL(url)
  stagedPreviews.value.clear()
})

function stageFile(file: File) {
  // Respect the cap at pick time, mirroring edit mode's capacity rule.
  if (atCapacity.value && !isSingle.value) return
  // Single-cap replaces rather than appends, matching the server's max:1
  // semantics (a new upload supersedes the existing file).
  const next = isSingle.value ? [file] : [...staged.value, file]
  if (isSingle.value) for (const f of staged.value) revokeStagedPreview(f)
  uploadError.value = ''
  emit('update:staged-files', next)
}

function unstageFile(file: File) {
  revokeStagedPreview(file)
  emit(
    'update:staged-files',
    staged.value.filter((f) => f !== file)
  )
}

function isImage(att: AttachmentInfo): boolean {
  return att.contentType?.startsWith('image/') ?? false
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

async function doUpload(file: File) {
  if (!props.entityType || !props.entityId || busy.value) return
  busy.value = true
  progress.value = 0
  uploadError.value = ''
  try {
    await uploadAttachment(props.entityType, props.entityId, props.propertyName, file, (f) => {
      progress.value = f
    })
    emit('attachment-changed')
  } catch (err) {
    uploadError.value = uploadErrorMessage(err)
  } finally {
    busy.value = false
  }
}

function uploadErrorMessage(err: unknown): string {
  if (err instanceof AttachmentError) {
    if (err.status === 413) return 'File is too large.'
    if (err.status === 409) return 'This field already holds the maximum number of files.'
    return err.message
  }
  return 'Upload failed.'
}

async function doDelete(att: AttachmentInfo) {
  if (busy.value) return
  busy.value = true
  uploadError.value = ''
  try {
    await deleteAttachment(att.href)
    emit('attachment-changed')
  } catch (err) {
    uploadError.value = err instanceof AttachmentError ? err.message : 'Delete failed.'
  } finally {
    busy.value = false
  }
}

// A picked file is STAGED in create mode and uploaded immediately in edit
// mode — the single branch that separates the two behaviours.
function acceptFile(file: File) {
  if (canStage.value) stageFile(file)
  else void doUpload(file)
}

function onFileInput(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) acceptFile(file)
  input.value = '' // allow re-selecting the same file
}

const dragOver = ref(false)
function onDrop(event: DragEvent) {
  dragOver.value = false
  if (!canAdd.value) return
  const file = event.dataTransfer?.files?.[0]
  if (file) acceptFile(file)
}
</script>

<template>
  <div :id="id" class="file-widget">
    <!-- The current files (display in any mode). -->
    <ul v-if="files.length" class="file-list">
      <li v-for="att in files" :key="att.id" class="file-item">
        <a
          v-if="isImage(att)"
          :href="att.href"
          target="_blank"
          rel="noopener"
          class="file-preview-link"
        >
          <img :src="att.href" :alt="att.filename" class="file-preview" />
        </a>
        <div class="file-meta">
          <a :href="att.href" :download="att.filename" class="file-name">{{ att.filename }}</a>
          <span class="file-size">{{ formatSize(att.size) }}</span>
          <button
            v-if="canEdit"
            type="button"
            class="file-remove"
            :disabled="busy"
            @click="doDelete(att)"
          >
            Remove
          </button>
        </div>
      </li>
    </ul>

    <!-- Staged (not yet uploaded) files — create mode only. Same markup as
         the persisted list, but the name is plain text: there is nothing to
         download yet, and the preview comes from a local object URL. -->
    <ul v-if="staged.length" class="file-list file-list-staged">
      <li v-for="file in staged" :key="`${file.name}-${file.size}-${file.lastModified}`" class="file-item">
        <img
          v-if="stagedPreviewUrl(file)"
          :src="stagedPreviewUrl(file)"
          :alt="file.name"
          class="file-preview"
        />
        <div class="file-meta">
          <span class="file-name">{{ file.name }}</span>
          <span class="file-size">{{ formatSize(file.size) }}</span>
          <span class="file-staged-badge">Pending save</span>
          <button type="button" class="file-remove" @click="unstageFile(file)">Remove</button>
        </div>
      </li>
    </ul>

    <span v-else-if="!files.length && mode !== 'edit'" class="file-empty">No file attached</span>

    <!-- Add / replace control (edit mode, with room). -->
    <div
      v-if="canAdd"
      class="file-dropzone"
      :class="{ 'is-dragover': dragOver, 'is-busy': busy }"
      @dragover.prevent="dragOver = true"
      @dragleave.prevent="dragOver = false"
      @drop.prevent="onDrop"
    >
      <label class="file-pick">
        <input type="file" :disabled="busy" @change="onFileInput" />
        <span>{{ isSingle && (files.length || staged.length) ? 'Replace file' : 'Add a file' }}</span>
      </label>
      <span class="file-hint">or drag &amp; drop</span>
      <span v-if="!isSingle" class="file-count">
        {{ files.length + staged.length }} / {{ maxCount }}
      </span>
    </div>

    <!-- At capacity in multi mode: explain why no add control. -->
    <p v-else-if="(canEdit || canStage) && !isSingle && atCapacity" class="file-edit-note">
      Maximum of {{ maxCount }} files reached — remove one to add another.
    </p>

    <!-- Edit mode but the widget can't mutate (no entity context / ACL). -->
    <p v-else-if="mode === 'edit' && !canEdit && !canStage" class="file-edit-note">
      {{ disabled ? 'Editing this attachment is not permitted.' : 'Attachment editing unavailable.' }}
    </p>

    <!-- Upload progress. -->
    <div v-if="busy && progress > 0" class="file-progress">
      <div class="file-progress-bar" :style="{ width: Math.round(progress * 100) + '%' }" />
    </div>

    <p v-if="uploadError" class="file-error">{{ uploadError }}</p>
  </div>
</template>

<style scoped>
.file-widget {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.file-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.file-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.file-preview-link {
  display: inline-block;
  max-width: 320px;
}

.file-preview {
  max-width: 320px;
  max-height: 240px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  object-fit: contain;
}

.file-meta {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.file-name {
  color: var(--accent-color, #6366f1);
  text-decoration: none;
  font-size: 14px;
}

.file-name:hover {
  text-decoration: underline;
}

.file-size {
  color: var(--text-muted, #6b7280);
  font-size: 12px;
}

.file-remove {
  margin-left: auto;
  border: none;
  background: none;
  color: var(--error-color, #ef4444);
  font-size: 13px;
  cursor: pointer;
}

.file-remove:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

/* A staged file is not yet persisted; the badge says so without relying on
   colour alone. */
.file-staged-badge {
  font-size: var(--font-size-sm, 12px);
  color: var(--text-muted, #6b7280);
  font-style: italic;
}

.file-empty {
  color: var(--text-muted, #6b7280);
  font-size: 14px;
  font-style: italic;
}

.file-edit-note {
  margin: 0;
  color: var(--text-muted, #6b7280);
  font-size: 12px;
}

.file-dropzone {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px dashed var(--border-color);
  border-radius: 6px;
  background: var(--input-bg);
}

/* A surface tint, not a focus ring — stays translucent (see ConflictsView). */
.file-dropzone.is-dragover {
  border-color: var(--accent-color, #6366f1);
  background: color-mix(in srgb, var(--accent-color) 6%, transparent);
}

.file-dropzone.is-busy {
  opacity: 0.6;
  pointer-events: none;
}

.file-pick {
  display: inline-flex;
  align-items: center;
  cursor: pointer;
}

.file-pick input[type='file'] {
  display: none;
}

.file-pick span {
  color: var(--accent-color, #6366f1);
  font-size: 14px;
}

.file-hint {
  color: var(--text-muted, #6b7280);
  font-size: 12px;
}

.file-count {
  margin-left: auto;
  color: var(--text-muted, #6b7280);
  font-size: 12px;
}

.file-progress {
  height: 4px;
  border-radius: 2px;
  background: var(--hover-bg, #e5e7eb);
  overflow: hidden;
}

.file-progress-bar {
  height: 100%;
  background: var(--accent-color, #6366f1);
  transition: width 0.1s linear;
}

.file-error {
  margin: 0;
  color: var(--error-color, #ef4444);
  font-size: 12px;
}
</style>
