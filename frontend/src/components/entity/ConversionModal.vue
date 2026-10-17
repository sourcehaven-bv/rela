<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useModalStack } from '@/composables/modalStack'
import { useSchemaStore } from '@/stores'
import type { ConversionOffer, Entity, PropertyDef } from '@/types'
import { convertEntity, previewConversion, type ConversionPreview } from '@/api/conversions'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlButtonGroup from 'rela-components/components/common/RlButtonGroup.vue'
import RlModal from 'rela-components/components/overlay/RlModal.vue'

const props = defineProps<{ source: Entity; offer: ConversionOffer }>()
const emit = defineEmits<{ close: []; converted: [id: string, face: string] }>()
useModalStack(computed(() => true))
const schema = useSchemaStore()
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
  return face || 'default face'
}
const faceMapping = reactive<Record<string, string>>({})
// A changed input invalidates the reviewed preview.
watch([values, faceMapping, targetID, content], () => {
  reviewed.value = false
}, { deep: true })
const mappedFields = computed(() => new Set(props.offer.mapped_fields ?? []))
const fields = computed(() => Object.entries(target.value?.properties ?? {}).filter(([name]) => !mappedFields.value.has(name)))
const sourceFaces = computed(() => {
  const available = new Set([props.source._world?.face ?? '', ...(props.source._faces ?? []).map((face) => face.face)])
  return Object.keys(props.offer.face_choices ?? {}).filter((face) => available.has(face))
})

function setValue(name: string, def: PropertyDef, input: string) {
  if (def.list) values[name] = input.split(',').map((item) => item.trim()).filter(Boolean)
  else if (def.type === 'integer') values[name] = input === '' ? undefined : Number(input)
  else values[name] = input
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
      error.value = `These relationships cannot be kept: ${preview.value.invalid_relations.join(', ')}`
      return
    }
    if (preview.value.invalid_faces?.length) {
      error.value = `These face choices are invalid: ${preview.value.invalid_faces.join(', ')}`
      return
    }
    if (preview.value.required_faces?.length) {
      error.value = `Choose a destination face for: ${preview.value.required_faces.map((face) => face || 'default face').join(', ')}`
      return
    }
    if (preview.value.required.length) {
      error.value = `Required fields need a value: ${preview.value.required.join(', ')}`
      return
    }
    if (preview.value.relations_to_drop.length && !acceptDrops.value) {
      error.value = 'Confirm that the listed relationships may be removed.'
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
    error.value = err instanceof Error ? err.message : 'Conversion failed'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <RlModal :open="true" :title="offer.label" size="lg" @close="emit('close')">
    <p>Convert {{ source.id }} into {{ target?.label || offer.to }}.</p>
    <label v-for="face in sourceFaces" :key="face" class="convert-field">
      {{ face || 'Default face' }} →
      <select v-model="faceMapping[face]" :required="(offer.face_choices?.[face]?.length ?? 0) > 1">
        <option value="">Choose destination face…</option>
        <option v-for="choice in offer.face_choices?.[face] ?? []" :key="choice" :value="choice">{{ choice || 'Default face' }}</option>
      </select>
    </label>
    <label v-if="target?.id_type === 'manual'" class="convert-field">
      New ID <span aria-hidden="true">*</span>
      <input v-model="targetID" required autocomplete="off" />
    </label>
    <label v-for="[name, def] in fields" :key="name" class="convert-field">
      {{ def.label || name }} <span v-if="def.required" aria-hidden="true">*</span>
      <input
        v-if="def.type === 'boolean'"
        type="checkbox"
        :checked="values[name] === true"
        @change="values[name] = ($event.target as HTMLInputElement).checked"
      />
      <select v-else-if="def.type === 'enum'" v-model="values[name]" :required="def.required">
        <option value="">Select…</option>
        <option v-for="option in def.values || []" :key="option" :value="option">
          {{ def.labels?.[option] || option }}
        </option>
      </select>
      <textarea v-else-if="def.list" :value="Array.isArray(values[name]) ? values[name].join(', ') : ''" rows="2" @input="setValue(name, def, ($event.target as HTMLTextAreaElement).value)" />
      <textarea v-else-if="name === 'description'" :value="String(values[name] ?? '')" rows="3" @input="setValue(name, def, ($event.target as HTMLTextAreaElement).value)" />
      <input v-else :value="String(values[name] ?? '')" :type="def.type === 'date' ? 'date' : def.type === 'integer' ? 'number' : 'text'" :required="def.required" @input="setValue(name, def, ($event.target as HTMLInputElement).value)" />
    </label>
    <label v-if="!offer.content_mapped" class="convert-field">Content <textarea v-model="content" rows="7" /></label>
    <section v-if="preview?.relations_to_drop.length" class="convert-warning" aria-live="polite">
      <strong>Relationships that will be removed</strong>
      <ul><li v-for="rel in preview.relations_to_drop" :key="rel">{{ rel }}</li></ul>
      <label><input v-model="acceptDrops" type="checkbox" /> I understand these relationships will be removed.</label>
    </section>
    <section v-if="preview?.invalid_relations.length" class="convert-warning" role="alert">
      <strong>Relationships that cannot be converted</strong>
      <ul><li v-for="rel in preview.invalid_relations" :key="rel">{{ rel }}</li></ul>
    </section>
    <section v-if="preview?.invalid_faces?.length" class="convert-warning" role="alert">
      <strong>Face choices that cannot be converted</strong>
      <ul><li v-for="face in preview.invalid_faces" :key="face">{{ face }}</li></ul>
    </section>
    <section v-if="reviewed && preview?.moves?.length" class="convert-warning" aria-live="polite">
      <strong>Comments move to the new {{ targetLabel }}</strong>
      <ul>
        <li v-for="move in preview.moves" :key="move.source_face">
          Comments on the {{ faceLabel(move.source_face) }} move to {{ targetLabel }} ({{ faceLabel(move.target_face) }}).
        </li>
      </ul>
      <p>Everyone who can read the new {{ targetLabel }} can read these comments. Its history starts at this conversion.</p>
    </section>
    <p v-if="error" role="alert">{{ error }}</p>
    <template #actions>
      <RlButtonGroup>
        <RlButton variant="secondary" :disabled="busy" @click="emit('close')">Cancel</RlButton>
        <RlButton variant="primary" :disabled="busy" @click="submit">{{ reviewed ? 'Convert' : 'Review' }}</RlButton>
      </RlButtonGroup>
    </template>
  </RlModal>
</template>

<style scoped>
.convert-field { display: grid; gap: .35rem; margin-block: .75rem; }
.convert-field input, .convert-field select, .convert-field textarea { width: 100%; }
</style>
