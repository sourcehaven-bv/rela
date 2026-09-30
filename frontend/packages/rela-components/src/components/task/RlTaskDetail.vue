<script setup lang="ts">
/**
 * The contents of a task's detail view: title, fields, description,
 * attachments and subtasks.
 *
 * It has no padding of its own. The surface it sits on owns that, which is
 * what lets it be followed in the same scroll area by a full-width divider or
 * comment list without the two disagreeing about where the edge is. In a
 * panel that surface is `RlDetailPanel`.
 */
import type { Attachment, DetailField, StatusOption, Tag, Task } from '../../types'
import RlDetailField from './RlDetailField.vue'
import RlHeading from '../common/RlHeading.vue'
import RlText from '../common/RlText.vue'
import RlAttachmentCard from './RlAttachmentCard.vue'
import RlAttachmentList from './RlAttachmentList.vue'
import RlSubtaskRow from './RlSubtaskRow.vue'

withDefaults(
  defineProps<{
    title: string
    fields: DetailField[]
    description?: string[]
    attachments?: Attachment[]
    subtasks?: Task[]
    showEmptyFieldsToggle?: boolean
    emptyFieldsLabel?: string
    /** Makes every detail field click-to-edit. */
    editableFields?: boolean
    /** Choices offered when a status field is being edited. */
    statusOptions?: StatusOption[]
    /** Choices offered when a single-tag field, such as a priority, is edited. */
    tagOptions?: Tag[]
  }>(),
  {
    description: () => [],
    attachments: () => [],
    subtasks: () => [],
    showEmptyFieldsToggle: true,
    emptyFieldsLabel: 'Show empty fields',
    editableFields: false,
    statusOptions: () => [],
    tagOptions: () => [],
  },
)

const emit = defineEmits<{
  toggleEmptyFields: []
  /** A field was edited and the edit kept. */
  updateField: [field: DetailField]
  selectSubtask: [task: Task]
  openAttachment: [attachment: Attachment]
}>()
</script>

<template>
  <div class="rl-task-detail">
    <!-- Slotted so the title can be made editable without a second heading. -->
    <slot name="title">
      <RlHeading :level="1" size="2xl" weight="normal" class="rl-task-detail__title">{{ title }}</RlHeading>
    </slot>

    <dl class="rl-task-detail__fields">
      <RlDetailField
        v-for="field in fields"
        :key="field.id"
        :field="field"
        :editable="editableFields"
        :options="statusOptions"
        :tag-options="tagOptions"
        @update:field="emit('updateField', $event)"
      >
        <!-- Passed through per field, so an app with its own property
             widgets or a per-field comment marker does not have to give up
             this component and rebuild the list. -->
        <template v-if="$slots['field-value']" #value="slotProps">
          <slot name="field-value" v-bind="slotProps" />
        </template>

        <template v-if="$slots['field-label-trailing']" #label-trailing="slotProps">
          <slot name="field-label-trailing" v-bind="slotProps" />
        </template>
      </RlDetailField>
    </dl>

    <div v-if="showEmptyFieldsToggle" class="rl-task-detail__divider">
      <button type="button" class="rl-task-detail__divider-button" @click="emit('toggleEmptyFields')">
        {{ emptyFieldsLabel }}
      </button>
    </div>

    <section v-if="description.length" class="rl-task-detail__section">
      <RlHeading
        :level="2"
        size="md"
        weight="normal"
        tone="muted"
        line-height="normal"
        class="rl-task-detail__section-title"
      >Description</RlHeading>
      <RlText
        v-for="(paragraph, index) in description"
        :key="index"
        as="p"
        size="sm"
        class="rl-task-detail__paragraph"
      >
        {{ paragraph }}
      </RlText>
    </section>

    <section v-if="attachments.length" class="rl-task-detail__section">
      <RlHeading
        :level="2"
        size="md"
        weight="normal"
        tone="muted"
        line-height="normal"
        class="rl-task-detail__section-title"
      >
        Attachements <RlText size="md" tone="subtle">&middot; {{ attachments.length }}</RlText>
      </RlHeading>
      <RlAttachmentList>
        <RlAttachmentCard
          v-for="attachment in attachments"
          :key="attachment.id"
          :attachment="attachment"
          @click="emit('openAttachment', $event)"
        />
      </RlAttachmentList>
    </section>

    <section v-if="subtasks.length" class="rl-task-detail__section">
      <RlHeading
        :level="2"
        size="md"
        weight="normal"
        tone="muted"
        line-height="normal"
        class="rl-task-detail__section-title"
      >
        Subtasks <RlText size="md" tone="subtle">&middot; {{ subtasks.length }}</RlText>
      </RlHeading>
      <div class="rl-task-detail__subtasks">
        <RlSubtaskRow
          v-for="(subtask, index) in subtasks"
          :key="subtask.id"
          :task="subtask"
          :highlight="index === 0"
          @click="emit('selectSubtask', $event)"
        />
      </div>
    </section>

    <slot />
  </div>
</template>

<style scoped>
.rl-task-detail__title { margin-bottom: var(--rl-space-6); }

.rl-task-detail__fields {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-3);
  margin: 0;
}

.rl-task-detail__divider {
  display: flex;
  align-items: center;
  gap: var(--rl-space-4);
  margin: var(--rl-space-5) 0 var(--rl-space-6);
}
.rl-task-detail__divider::before,
.rl-task-detail__divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--rl-color-border);
}

.rl-task-detail__divider-button {
  padding: var(--rl-space-2) var(--rl-space-3);
  border: none;
  border-radius: var(--rl-radius-md);
  background: transparent;
  font-family: inherit;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-subtle);
  cursor: pointer;
}
.rl-task-detail__divider-button:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}
.rl-task-detail__divider-button:hover { color: var(--rl-color-text-muted); }

.rl-task-detail__section + .rl-task-detail__section { margin-top: var(--rl-space-8); }

.rl-task-detail__section-title { margin-bottom: var(--rl-space-3); }

.rl-task-detail__paragraph {
  margin-bottom: var(--rl-space-4);
  line-height: var(--rl-line-height-relaxed);
}
.rl-task-detail__paragraph:last-child { margin-bottom: 0; }

.rl-task-detail__subtasks {
  border-top: 1px solid var(--rl-color-border);
}

@media (max-width: 767px) {
  .rl-task-detail__title { font-size: var(--rl-font-size-xl); }
}
</style>
