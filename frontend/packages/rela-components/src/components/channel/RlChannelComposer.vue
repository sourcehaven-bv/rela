<script setup lang="ts">
/**
 * Writes a channel message, or edits one.
 *
 * It offers only the message types the user may post now (`postable`, the
 * channel's `_actions.post`). While a reply is being written it offers only
 * the types that may reply. With nothing to offer it renders nothing; the
 * caller decides what to say instead.
 *
 * The fields follow the presentation (data-entry.yaml `channels:`) through
 * `resolveChannelFields`: listed order, labels, help, placeholders, widgets
 * and spans, hidden fields left out, the rest required-first then by name.
 *
 * It checks only what it can know for certain: a required field is filled,
 * and a text is within `max` code points (as the server counts). Everything
 * else (dates, integers, the server's own rules) is the app's, which passes
 * field messages back in `errors` and a general one in `error`.
 *
 * Values are emitted raw and per message type, so switching type and back
 * keeps what was typed. The composer does not clear itself on submit, since
 * it cannot know whether the post landed: the app calls `reset()` after a
 * success and leaves the values in place after a failure.
 */
import { computed, nextTick, reactive, ref, useTemplateRef, watch } from 'vue'
import RlButton from '../common/RlButton.vue'
import RlButtonGroup from '../common/RlButtonGroup.vue'
import RlIconButton from '../common/RlIconButton.vue'
import RlBanner from '../feedback/RlBanner.vue'
import RlKbd from '../data/RlKbd.vue'
import RlSegmentedControl from '../data/RlSegmentedControl.vue'
import RlChannelField from './RlChannelField.vue'
import { codePointLength, resolveChannelFields, type ResolvedChannelField } from './channelFields'
import type {
  ChannelEditEvent,
  ChannelEditing,
  ChannelEnumOption,
  ChannelFieldValue,
  ChannelMessageTypeDef,
  ChannelPostEvent,
  ChannelPresentation,
  ChannelReadableRef,
  ChannelReplyQuote,
} from './types'
import { useMessages } from '../../composables/useMessages'

const props = withDefaults(
  defineProps<{
    /** Every message type of the channel type. */
    messageTypes: ChannelMessageTypeDef[]
    /** The message types the user may post now. */
    postable: string[]
    presentation?: ChannelPresentation
    /** Enum labels and colours, keyed by enum type. */
    enumOptions?: Record<string, ChannelEnumOption[]>
    /** Asks the app to pick an entity of one of the given types. */
    pickRef?: (to: string[]) => Promise<ChannelReadableRef | null>
    /** The message being replied to. */
    replyTo?: ChannelReplyQuote
    /** The message being edited; switches the composer to edit mode. */
    editing?: ChannelEditing
    submitting?: boolean
    /** Field messages from the app, keyed by property name. */
    errors?: Record<string, string>
    /** A message that belongs to no one field. */
    error?: string
    /** Named under a datetime field, so the zone is never implied. */
    timeZoneLabel?: string
    submitShortcut?: string
  }>(),
  { submitting: false, submitShortcut: 'Ctrl+Enter' },
)

const emit = defineEmits<{
  post: [event: ChannelPostEvent]
  save: [event: ChannelEditEvent]
  cancelReply: []
  cancelEdit: []
}>()

const messages = useMessages()
const form = useTemplateRef<HTMLFormElement>('form')

const offered = computed(() => {
  if (props.editing) {
    const t = props.messageTypes.find((m) => m.name === props.editing!.type)
    return t ? [t] : []
  }
  const allowed = new Set(props.postable)
  return props.messageTypes.filter((t) => allowed.has(t.name) && (!props.replyTo || t.reply))
})

const selected = ref('')
const current = computed(
  () => offered.value.find((t) => t.name === selected.value) ?? offered.value[0],
)

watch(
  offered,
  (list) => {
    if (!list.some((t) => t.name === selected.value)) selected.value = list[0]?.name ?? ''
  },
  { immediate: true },
)

const fields = computed<ResolvedChannelField[]>(() =>
  current.value
    ? resolveChannelFields(current.value, props.presentation?.messages?.[current.value.name])
    : [],
)

/** Values per message type, so a type switch keeps what was typed. */
const drafts = reactive<Record<string, Record<string, ChannelFieldValue>>>({})
const refs = reactive<Record<string, ChannelReadableRef | undefined>>({})
const localErrors = ref<Record<string, string>>({})

function seeded(type: ChannelMessageTypeDef): Record<string, ChannelFieldValue> {
  // A required boolean has no "not set": it starts unchecked, so "no" needs no click.
  const out: Record<string, ChannelFieldValue> = {}
  for (const p of type.properties) if (p.type === 'boolean' && p.required) out[p.name] = false
  return out
}

watch(
  current,
  (t) => {
    if (t && !drafts[t.name]) drafts[t.name] = seeded(t)
  },
  { immediate: true },
)

const values = computed<Record<string, ChannelFieldValue>>(() =>
  current.value ? (drafts[current.value.name] ?? {}) : {},
)

/**
 * The new-message draft an edit displaced, put back when the edit ends, so
 * editing an old message never eats a message half written.
 */
let stashed: { type: string; values: Record<string, ChannelFieldValue> | undefined } | null = null

watch(
  () => props.editing,
  (e, prev) => {
    localErrors.value = {}
    for (const k of Object.keys(refs)) delete refs[k]
    if (prev && stashed) {
      const t = props.messageTypes.find((m) => m.name === stashed!.type)
      if (stashed.values) drafts[stashed.type] = stashed.values
      else if (t) drafts[stashed.type] = seeded(t)
      stashed = null
    }
    if (!e) return
    stashed = { type: e.type, values: drafts[e.type] }
    selected.value = e.type
    drafts[e.type] = { ...e.values }
    Object.assign(refs, e.refs ?? {})
  },
  { immediate: true },
)

function setValue(name: string, v: ChannelFieldValue) {
  const t = current.value
  if (!t) return
  ;(drafts[t.name] ??= seeded(t))[name] = v
  if (localErrors.value[name]) {
    const { [name]: _, ...rest } = localErrors.value
    localErrors.value = rest
  }
}

function isEmpty(v: ChannelFieldValue): boolean {
  return v === undefined || (typeof v === 'string' && v.trim() === '')
}

function check(): Record<string, string> {
  const out: Record<string, string> = {}
  for (const f of fields.value) {
    const v = values.value[f.property.name]
    if (f.property.required && isEmpty(v)) out[f.property.name] = 'Required.'
    else if (f.property.max && typeof v === 'string' && codePointLength(v) > f.property.max) {
      out[f.property.name] = `At most ${f.property.max} characters.`
    }
  }
  return out
}

function errorFor(name: string): string | undefined {
  return localErrors.value[name] ?? props.errors?.[name]
}

async function submit() {
  const t = current.value
  if (!t || props.submitting) return
  localErrors.value = check()
  if (Object.keys(localErrors.value).length) {
    await nextTick()
    form.value?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
    return
  }
  const rendered = fields.value.map((f) => f.property.name)
  const out: Record<string, ChannelFieldValue> = {}
  for (const name of rendered) out[name] = values.value[name]
  if (props.editing) {
    emit('save', { seq: props.editing.seq, type: t.name, values: out, rendered })
  } else {
    emit('post', { type: t.name, values: out, rendered, replyTo: props.replyTo?.seq })
  }
}

function cancelEdit() {
  localErrors.value = {}
  emit('cancelEdit')
}

/** Clears the current type's values, for the app to call after a post landed. */
function reset() {
  const t = current.value
  if (t) drafts[t.name] = seeded(t)
  for (const k of Object.keys(refs)) delete refs[k]
  localErrors.value = {}
}

defineExpose({
  reset,
  focus: () => form.value?.querySelector<HTMLElement>('input, textarea, select, button')?.focus(),
})
</script>

<template>
  <form
    v-if="current"
    ref="form"
    class="rl-channel-composer"
    :aria-label="editing ? 'Edit message' : 'New message'"
    novalidate
    @submit.prevent="submit"
    @keydown.meta.enter.prevent="submit"
    @keydown.ctrl.enter.prevent="submit"
  >
    <RlSegmentedControl
      v-if="offered.length > 1"
      v-model="selected"
      class="rl-channel-composer__types"
      label="Message type"
      size="sm"
      :options="offered.map((t) => ({ value: t.name, label: t.label }))"
    />

    <div v-if="replyTo && !editing" class="rl-channel-composer__reply">
      <span class="rl-channel-composer__reply-text">
        <strong>{{ messages.channelReplyingTo({ author: replyTo.author }) }}</strong>
        <span v-if="replyTo.excerpt"> {{ replyTo.excerpt }}</span>
      </span>
      <RlIconButton icon="x" label="Cancel reply" :size="14" @click="emit('cancelReply')" />
    </div>

    <RlBanner v-if="error" tone="danger" class="rl-channel-composer__banner">{{ error }}</RlBanner>

    <div class="rl-channel-composer__fields">
      <div
        v-for="f in fields"
        :key="`${current.name}:${f.property.name}`"
        class="rl-channel-composer__field"
        :style="{ '--rl-channel-span': f.span ?? 12 }"
      >
        <RlChannelField
          :field="f"
          :model-value="values[f.property.name]"
          :error="errorFor(f.property.name)"
          :enum-options="enumOptions"
          :chosen-ref="refs[f.property.name]"
          :pick-ref="pickRef"
          :time-zone-label="timeZoneLabel"
          @update:model-value="(v) => setValue(f.property.name, v)"
          @pick="(r) => (refs[f.property.name] = r ?? undefined)"
        />
      </div>
    </div>

    <div class="rl-channel-composer__foot">
      <span class="rl-channel-composer__hint">
        <RlKbd :keys="submitShortcut" size="sm" /> to {{ editing ? 'save' : 'send' }}
      </span>
      <RlButtonGroup>
        <RlButton v-if="editing" variant="ghost" size="sm" :disabled="submitting" @click="cancelEdit">
          Cancel
        </RlButton>
        <RlButton
          variant="primary"
          size="sm"
          type="submit"
          icon="send"
          :loading="submitting"
          :pending-label="editing ? 'Saving…' : 'Sending…'"
        >
          {{ editing ? 'Save' : 'Send' }}
        </RlButton>
      </RlButtonGroup>
    </div>
  </form>
</template>

<style scoped>
.rl-channel-composer {
  container-type: inline-size;
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-3);
  padding: var(--rl-space-3);
  border: 1px solid var(--rl-color-border-strong);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-bg);
}

.rl-channel-composer__types { align-self: flex-start; }

.rl-channel-composer__reply {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  padding: var(--rl-space-1) var(--rl-space-1) var(--rl-space-1) var(--rl-space-3);
  border-left: 3px solid var(--rl-color-border-strong);
  background: var(--rl-color-bg-sunken);
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-muted);
}

.rl-channel-composer__reply-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-channel-composer__fields {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: var(--rl-space-3);
}

.rl-channel-composer__field { grid-column: span var(--rl-channel-span); }

/* Side-by-side fields stop making sense once each would be a sliver. */
@container (max-width: 480px) {
  .rl-channel-composer__field { grid-column: 1 / -1; }
}

.rl-channel-composer__foot {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
}

.rl-channel-composer__hint {
  display: flex;
  align-items: center;
  gap: var(--rl-space-1);
  margin-right: auto;
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-subtle);
}
</style>
