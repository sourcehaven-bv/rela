<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useModalStack } from '@/composables/modalStack'
import { useSchemaStore } from '@/stores'
import type { ConversionOffer, Entity } from '@/types'
import { convertEntity, previewConversion, type ConversionPreview } from '@/api/conversions'
import FieldRenderer from '@/components/forms/FieldRenderer.vue'
import MarkdownEditor from '@/components/forms/milkdown/MilkdownEditor.vue'
import { makeRefResolver } from '@/utils/entityRefResolver'
import { refBareId } from '@/utils/entityRef'
import {
  conversionFields,
  createFormFor,
  isEmptyValue,
  type PropertyField,
} from './conversionFields'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlButtonGroup from 'rela-components/components/common/RlButtonGroup.vue'
import RlModal from 'rela-components/components/overlay/RlModal.vue'

const props = defineProps<{ source: Entity; offer: ConversionOffer }>()
const emit = defineEmits<{ close: []; converted: [id: string, face: string] }>()
useModalStack(computed(() => true))
const schema = useSchemaStore()
const { t } = useI18n()
const target = computed(() => schema.getEntityType(props.offer.to))
const values = reactive<Record<string, unknown>>({})
const targetID = ref('')
const content = ref(props.source.content ?? '')
const busy = ref(false)
const error = ref('')
const preview = ref<ConversionPreview | null>(null)
const acceptDrops = ref(false)
const reviewed = ref(false)
const targetLabel = computed(() => target.value?.label || props.offer.to)
function faceLabel(face: string) {
  return face || t('entity.convert.defaultFaceLower')
}
const faceMapping = reactive<Record<string, string>>({})
// A changed input invalidates the reviewed preview.
watch(
  [values, faceMapping, targetID, content],
  () => {
    reviewed.value = false
  },
  { deep: true }
)
const mappedFields = computed(() => new Set(props.offer.mapped_fields ?? []))
// The target's create form decides each field's widget, label and order.
const fields = computed(() =>
  conversionFields(
    target.value?.properties ?? {},
    createFormFor(schema.forms, props.offer.to),
    mappedFields.value
  )
)
// The preview names the required fields still without a value; each gets
// its error next to its input, until the field is edited.
const fieldErrors = ref<Record<string, string>>({})
watch(preview, (next) => {
  const errors: Record<string, string> = {}
  for (const name of next?.required ?? []) errors[name] = t('form.validation.required')
  fieldErrors.value = errors
})
// The optional fields open when one of them has an error, so it is seen.
const optionalHasError = computed(() =>
  fields.value.optional.some((field) => field.property in fieldErrors.value)
)
const refResolver = computed(() => makeRefResolver(props.source.mentions))
const mentionSelf = computed(() => ({
  id: refBareId(props.source.id),
  type: props.source.type,
  face: props.source._world?.face ?? '',
}))
const sourceFaces = computed(() => {
  const available = new Set([
    props.source._world?.face ?? '',
    ...(props.source._faces ?? []).map((face) => face.face),
  ])
  return Object.keys(props.offer.face_choices ?? {}).filter((face) => available.has(face))
})

// An emptied field is left out of the request, so it reads as not supplied.
function setValue(field: PropertyField, value: unknown) {
  const name = field.property
  if (isEmptyValue(value)) delete values[name]
  else values[name] = value
  if (name in fieldErrors.value) {
    const { [name]: _cleared, ...rest } = fieldErrors.value
    fieldErrors.value = rest
  }
}

async function submit() {
  error.value = ''
  busy.value = true
  try {
    const request = {
      source_id: props.source.id,
      source_face: props.source._world?.face ?? '',
      ...(target.value?.id_type === 'manual' ? { target_id: targetID.value } : {}),
      properties: values,
      faces: faceMapping,
      ...(!props.offer.content_mapped ? { content: content.value } : {}),
    }
    preview.value = await previewConversion(props.offer.name, request)
    if (preview.value.invalid_relations.length) {
      error.value = t('entity.convert.invalidRelations', {
        list: preview.value.invalid_relations.join(', '),
      })
      return
    }
    if (preview.value.invalid_faces?.length) {
      error.value = t('entity.convert.invalidFaces', {
        list: preview.value.invalid_faces.join(', '),
      })
      return
    }
    if (preview.value.required_faces?.length) {
      error.value = t('entity.convert.requiredFaces', {
        list: preview.value.required_faces.map((face) => faceLabel(face)).join(', '),
      })
      return
    }
    if (preview.value.required.length) {
      error.value = t('entity.convert.requiredFields', { list: preview.value.required.join(', ') })
      return
    }
    if (preview.value.relations_to_drop.length && !acceptDrops.value) {
      error.value = t('entity.convert.confirmDrops')
      return
    }
    // The first passing preview only shows its consequences, including who
    // can read the comments afterwards; the next click converts.
    if (!reviewed.value) {
      reviewed.value = true
      return
    }
    const result = await convertEntity(props.offer.name, {
      ...request,
      ...(acceptDrops.value ? { drop_relations: preview.value.relations_to_drop } : {}),
    })
    emit('converted', result.entity_id, result.face)
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('entity.convert.failed')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <RlModal :open="true" :title="offer.label" size="lg" @close="emit('close')">
    <p>{{ t('entity.convert.intro', { id: source.id, type: target?.label || offer.to }) }}</p>
    <label v-for="face in sourceFaces" :key="face" class="convert-field">
      {{ face || t('entity.convert.defaultFace') }} →
      <select v-model="faceMapping[face]" :required="(offer.face_choices?.[face]?.length ?? 0) > 1">
        <option value="">{{ t('entity.convert.chooseFace') }}</option>
        <option v-for="choice in offer.face_choices?.[face] ?? []" :key="choice" :value="choice">
          {{ choice || t('entity.convert.defaultFace') }}
        </option>
      </select>
    </label>
    <label v-if="target?.id_type === 'manual'" class="convert-field">
      {{ t('entity.convert.newId') }} <span aria-hidden="true">*</span>
      <input v-model="targetID" required autocomplete="off" />
    </label>
    <div class="convert-fields">
      <FieldRenderer
        v-for="field in fields.required"
        :key="field.property"
        :field="field"
        :property-def="target?.properties?.[field.property]"
        :value="values[field.property]"
        :error="fieldErrors[field.property]"
        :entity-type="offer.to"
        @update="setValue(field, $event)"
      />
    </div>
    <details v-if="fields.optional.length" class="convert-more" :open="optionalHasError">
      <summary>{{ t('entity.convert.moreFields') }}</summary>
      <div class="convert-fields">
        <FieldRenderer
          v-for="field in fields.optional"
          :key="field.property"
          :field="field"
          :property-def="target?.properties?.[field.property]"
          :value="values[field.property]"
          :error="fieldErrors[field.property]"
          :entity-type="offer.to"
          @update="setValue(field, $event)"
        />
      </div>
    </details>
    <p v-if="fields.files.length" class="convert-files">
      {{ t('entity.convert.filesLater', { list: fields.files.join(', ') }) }}
    </p>
    <div v-if="!offer.content_mapped" class="convert-field">
      <span>{{ t('entity.convert.content') }}</span>
      <MarkdownEditor v-model="content" :ref-resolver="refResolver" :mention-self="mentionSelf" />
    </div>
    <section v-if="preview?.relations_to_drop.length" class="convert-warning" aria-live="polite">
      <strong>{{ t('entity.convert.dropsHeading') }}</strong>
      <ul>
        <li v-for="rel in preview.relations_to_drop" :key="rel">{{ rel }}</li>
      </ul>
      <label
        ><input v-model="acceptDrops" type="checkbox" />
        {{ t('entity.convert.acceptDrops') }}</label
      >
    </section>
    <section v-if="preview?.invalid_relations.length" class="convert-warning" role="alert">
      <strong>{{ t('entity.convert.invalidRelationsHeading') }}</strong>
      <ul>
        <li v-for="rel in preview.invalid_relations" :key="rel">{{ rel }}</li>
      </ul>
    </section>
    <section v-if="preview?.invalid_faces?.length" class="convert-warning" role="alert">
      <strong>{{ t('entity.convert.invalidFacesHeading') }}</strong>
      <ul>
        <li v-for="face in preview.invalid_faces" :key="face">{{ face }}</li>
      </ul>
    </section>
    <p v-if="reviewed" class="convert-ready" aria-live="polite">
      {{ t('entity.convert.reviewDone', { id: source.id, type: targetLabel }) }}
    </p>
    <section v-if="reviewed && preview?.moves?.length" class="convert-warning" aria-live="polite">
      <strong>{{ t('entity.convert.commentsMoveHeading', { type: targetLabel }) }}</strong>
      <ul>
        <li v-for="move in preview.moves" :key="move.source_face">
          {{
            t('entity.convert.commentsMove', {
              from: faceLabel(move.source_face),
              type: targetLabel,
              to: faceLabel(move.target_face),
            })
          }}
        </li>
      </ul>
      <p>
        {{ t('entity.convert.commentsReaders', { type: targetLabel }) }}
      </p>
    </section>
    <p v-if="error" role="alert">{{ error }}</p>
    <template #actions>
      <RlButtonGroup>
        <RlButton variant="secondary" :disabled="busy" @click="emit('close')">{{
          t('common.cancel')
        }}</RlButton>
        <RlButton variant="primary" :disabled="busy" @click="submit">{{
          reviewed ? t('entity.convert.convert') : t('entity.convert.review')
        }}</RlButton>
      </RlButtonGroup>
    </template>
  </RlModal>
</template>

<style scoped>
.convert-field {
  display: grid;
  gap: 0.35rem;
  margin-block: 0.75rem;
}
.convert-field input,
.convert-field select {
  width: 100%;
}
.convert-fields {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: var(--space-md);
  margin-block: var(--space-md);
}
.convert-fields > * {
  grid-column: span var(--field-span, 12);
}
.convert-more > summary {
  cursor: pointer;
}
</style>
