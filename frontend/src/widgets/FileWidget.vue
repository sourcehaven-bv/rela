<script setup lang="ts">
import { computed, ref } from 'vue'
import type { WidgetProps } from './types'
import type { AttachmentInfo } from '@/types'
import { uploadAttachment, deleteAttachment, AttachmentError } from '@/api/attachments'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  // Fired after a successful upload or delete so the parent can refresh
  // the entity (the property value and _attachments changed server-side).
  'attachment-changed': []
}>()

const files = computed<AttachmentInfo[]>(() => props.attachments ?? [])
const maxCount = computed(() => props.max ?? 1)
const isSingle = computed(() => maxCount.value <= 1)
const atCapacity = computed(() => files.value.length >= maxCount.value)

// Edit mode can mutate only when the widget knows the owning entity and
// isn't disabled by ACL.
const canEdit = computed(
  () => props.mode === 'edit' && !props.disabled && !!props.entityType && !!props.entityId
)
// The add control shows when editing and there's room (single-cap: shows
// as "Replace" once a file exists; multi-cap: hidden at capacity).
const canAdd = computed(() => canEdit.value && (isSingle.value || !atCapacity.value))

const busy = ref(false)
const progress = ref(0)
const uploadError = ref('')

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

function onFileInput(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) void doUpload(file)
  input.value = '' // allow re-selecting the same file
}

const dragOver = ref(false)
function onDrop(event: DragEvent) {
  dragOver.value = false
  if (!canAdd.value) return
  const file = event.dataTransfer?.files?.[0]
  if (file) void doUpload(file)
}
</script>

<template>
  <div class="file-widget">
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

    <span v-else-if="mode !== 'edit'" class="file-empty">No file attached</span>

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
        <span>{{ isSingle && files.length ? 'Replace file' : 'Add a file' }}</span>
      </label>
      <span class="file-hint">or drag &amp; drop</span>
      <span v-if="!isSingle" class="file-count">{{ files.length }} / {{ maxCount }}</span>
    </div>

    <!-- At capacity in multi mode: explain why no add control. -->
    <p v-else-if="canEdit && !isSingle && atCapacity" class="file-edit-note">
      Maximum of {{ maxCount }} files reached — remove one to add another.
    </p>

    <!-- Edit mode but the widget can't mutate (no entity context / ACL). -->
    <p v-else-if="mode === 'edit' && !canEdit" class="file-edit-note">
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

.file-dropzone.is-dragover {
  border-color: var(--accent-color, #6366f1);
  background: rgba(99, 102, 241, 0.06);
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
