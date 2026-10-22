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
 * Ctrl+Enter (or Cmd+Enter) and the button send. A plain Enter in a one-line
 * field does not, so a half-filled card is never sent by accident.
 *
 * Values and chosen refs are kept per message type, so switching type and
 * back keeps what was typed. The composer does not clear itself on submit,
 * since it cannot know whether the post landed; see `reset`.
 *
 * Edit mode starts when `editing` names a new message (by `seq` and `type`).
 * A new `editing` object for the same message, such as one rebuilt after a
 * live update, does not reseed the fields: the user's typing wins, and the
 * app decides whether to warn that the message changed. The new-message
 * draft and the selected type are put aside for the edit and come back when
 * `editing` is cleared, whether the edit was saved or cancelled.
 *
 * It emits `typing` when the user changes a field to a non-empty value while
 * writing a new message of a `log` type. Picking a ref counts: the field
 * sends the picked id as its new value. It never emits during an edit or
 * for an `ephemeral` type, which announces no message to come. The app
 * decides whether and how often to tell the server.
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
import { useMessages, type ChannelTextKey } from '../../composables/useMessages'

const props = withDefaults(
  defineProps<{
    /** Every message type of the channel type. */
    messageTypes: ChannelMessageTypeDef[]
    /** The message types the user may post now. */
    postable: string[]
    presentation?: ChannelPresentation
    /** Enum labels and colours, keyed by enum type. */
    enumOptions?: Record<string, ChannelEnumOption[]>
    /**
     * Asks the app to pick an entity of one of the given types. Resolve null
     * on cancel; never reject (a rejection is treated as a cancel).
     */
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
  {
    presentation: undefined,
    enumOptions: undefined,
    pickRef: undefined,
    replyTo: undefined,
    editing: undefined,
    submitting: false,
    errors: undefined,
    error: undefined,
    timeZoneLabel: undefined,
    submitShortcut: 'Ctrl+Enter',
  },
)

const emit = defineEmits<{
  post: [event: ChannelPostEvent]
  save: [event: ChannelEditEvent]
  cancelReply: []
  cancelEdit: []
  typing: []
}>()

const messages = useMessages()
const t = (key: ChannelTextKey) => messages.channelText({ key })
const form = useTemplateRef<HTMLFormElement>('form')

const offered = computed(() => {
  if (props.editing) {
    const type = props.messageTypes.find((m) => m.name === props.editing!.type)
    return type ? [type] : []
  }
  const allowed = new Set(props.postable)
  return props.messageTypes.filter((m) => allowed.has(m.name) && (!props.replyTo || m.reply))
})

/** An edit of a message whose type the descriptor no longer has. */
const typeGone = computed(() => !!props.editing && offered.value.length === 0)

const selected = ref('')
const current = computed(
  () => offered.value.find((m) => m.name === selected.value) ?? offered.value[0],
)

watch(
  offered,
  (list) => {
    // An edit sets and restores the selection itself; see the editing watcher.
    if (props.editing) return
    if (!list.some((m) => m.name === selected.value)) selected.value = list[0]?.name ?? ''
  },
  { immediate: true },
)

const fields = computed<ResolvedChannelField[]>(() =>
  current.value
    ? resolveChannelFields(current.value, props.presentation?.messages?.[current.value.name])
    : [],
)

type Values = Record<string, ChannelFieldValue>
type Refs = Record<string, ChannelReadableRef | undefined>

/** Values per message type, so a type switch keeps what was typed. */
const drafts = reactive<Record<string, Values>>({})
/** Chosen refs per message type, next to the ids in `drafts`. */
const refs = reactive<Record<string, Refs>>({})
/** In an edit: the ref properties still holding a ref the user may not read. */
const restricted = reactive(new Set<string>())
const localErrors = ref<Record<string, string>>({})

function seeded(type: ChannelMessageTypeDef): Values {
  // A required boolean has no "not set": it starts unchecked, so "no" needs no click.
  const out: Values = {}
  for (const p of type.properties) if (p.type === 'boolean' && p.required) out[p.name] = false
  return out
}

function reseed(type: string) {
  const def = props.messageTypes.find((m) => m.name === type)
  drafts[type] = def ? seeded(def) : {}
  refs[type] = {}
}

watch(
  current,
  (type) => {
    if (type && !drafts[type.name]) drafts[type.name] = seeded(type)
  },
  { immediate: true },
)

const values = computed<Values>(() => (current.value ? (drafts[current.value.name] ?? {}) : {}))
const chosen = computed<Refs>(() => (current.value ? (refs[current.value.name] ?? {}) : {}))

/**
 * What an edit displaced: the selected type, and the draft and refs of the
 * edited type, put back when the edit ends.
 */
let stashed: { selected: string; type: string; values?: Values; refs?: Refs } | null = null

watch(
  () => (props.editing ? `${props.editing.seq}:${props.editing.type}` : ''),
  (_key, prevKey) => {
    localErrors.value = {}
    restricted.clear()
    if (prevKey && stashed) {
      const s = stashed
      stashed = null
      if (s.values) drafts[s.type] = s.values
      else reseed(s.type)
      refs[s.type] = s.refs ?? {}
      if (offered.value.some((m) => m.name === s.selected)) selected.value = s.selected
    }
    const e = props.editing
    if (!e) return
    stashed = { selected: selected.value, type: e.type, values: drafts[e.type], refs: refs[e.type] }
    selected.value = e.type
    drafts[e.type] = { ...e.values }
    refs[e.type] = { ...e.refs }
    for (const name of e.restricted ?? []) restricted.add(name)
  },
  { immediate: true },
)

function setValue(name: string, v: ChannelFieldValue) {
  const type = current.value
  if (!type) return
  ;(drafts[type.name] ??= seeded(type))[name] = v
  if (!props.editing && type.persist === 'log' && !isEmpty(v)) emit('typing')
  if (localErrors.value[name]) {
    const { [name]: _, ...rest } = localErrors.value
    localErrors.value = rest
  }
}

function setRef(name: string, r: ChannelReadableRef | null) {
  const type = current.value
  if (!type) return
  ;(refs[type.name] ??= {})[name] = r ?? undefined
  if (r) restricted.delete(name)
}

function isEmpty(v: ChannelFieldValue): boolean {
  return v === undefined || (typeof v === 'string' && v.trim() === '')
}

function check(): Record<string, string> {
  const out: Record<string, string> = {}
  for (const f of fields.value) {
    const name = f.property.name
    if (restricted.has(name)) continue
    const v = values.value[name]
    if (f.property.required && isEmpty(v)) out[name] = t('required')
    else if (f.property.max && typeof v === 'string' && codePointLength(v) > f.property.max) {
      out[name] = messages.channelTooLong({ max: f.property.max })
    }
  }
  return out
}

function errorFor(name: string): string | undefined {
  return localErrors.value[name] ?? props.errors?.[name]
}

async function submit() {
  const type = current.value
  if (!type || props.submitting) return
  localErrors.value = check()
  if (Object.keys(localErrors.value).length) {
    await nextTick()
    form.value?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
    return
  }
  // An untouched restricted ref is left out, so the app carries it as
  // unrendered and the server keeps it.
  const rendered = fields.value.map((f) => f.property.name).filter((n) => !restricted.has(n))
  const out: Values = {}
  for (const name of rendered) out[name] = values.value[name]
  if (props.editing) {
    emit('save', { seq: props.editing.seq, type: type.name, values: out, rendered })
  } else {
    emit('post', { type: type.name, values: out, rendered, replyTo: props.replyTo?.seq })
  }
}

function cancelEdit() {
  localErrors.value = {}
  emit('cancelEdit')
}

/**
 * Clears a message type's values and refs, for the app to call once a post
 * landed. Pass the `post` event it acknowledges: that clears the posted type
 * whatever is on screen by then, even if the user switched type or started
 * an edit while the request was in flight. Without an argument it clears the
 * current type, outside an edit only.
 *
 * Do not call it for a save. Clearing `editing` ends the edit and brings back
 * the draft the edit put aside; a `save` event passed here is ignored, so a
 * late call cannot wipe that draft.
 */
function reset(posted?: ChannelPostEvent | ChannelEditEvent) {
  localErrors.value = {}
  if (posted && 'seq' in posted) return
  const type = posted?.type ?? (props.editing ? undefined : current.value?.name)
  if (!type) return
  if (props.editing && stashed?.type === type) {
    // The posted draft is the one the edit put aside.
    stashed.values = undefined
    stashed.refs = undefined
    return
  }
  reseed(type)
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
    :aria-label="t(editing ? 'editMessage' : 'newMessage')"
    novalidate
    @submit.prevent
    @keydown.meta.enter.prevent="submit"
    @keydown.ctrl.enter.prevent="submit"
  >
    <RlSegmentedControl
      v-if="offered.length > 1"
      v-model="selected"
      class="rl-channel-composer__types"
      :label="t('messageType')"
      size="sm"
      :options="offered.map((m) => ({ value: m.name, label: m.label }))"
    />

    <div v-if="replyTo && !editing" class="rl-channel-composer__reply">
      <span class="rl-channel-composer__reply-text">
        <strong>{{ messages.channelReplyingTo({ author: replyTo.author }) }}</strong>
        <span v-if="replyTo.excerpt"> {{ replyTo.excerpt }}</span>
      </span>
      <RlIconButton icon="x" :label="t('cancelReply')" :size="14" @click="emit('cancelReply')" />
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
          :chosen-ref="chosen[f.property.name]"
          :pick-ref="pickRef"
          :restricted="restricted.has(f.property.name)"
          :time-zone-label="timeZoneLabel"
          @update:model-value="(v) => setValue(f.property.name, v)"
          @pick="(r) => setRef(f.property.name, r)"
        />
      </div>
    </div>

    <div class="rl-channel-composer__foot">
      <span class="rl-channel-composer__hint">
        <RlKbd :keys="submitShortcut" size="sm" /> {{ t(editing ? 'toSave' : 'toSend') }}
      </span>
      <RlButtonGroup>
        <RlButton v-if="editing" variant="ghost" size="sm" :disabled="submitting" @click="cancelEdit">
          {{ t('cancel') }}
        </RlButton>
        <RlButton
          variant="primary"
          size="sm"
          icon="send"
          :loading="submitting"
          :pending-label="t(editing ? 'saving' : 'sending')"
          @click="submit"
        >
          {{ t(editing ? 'save' : 'send') }}
        </RlButton>
      </RlButtonGroup>
    </div>
  </form>
  <div v-else-if="typeGone" class="rl-channel-composer rl-channel-composer--gone" role="status">
    <span>{{ t('typeGone') }}</span>
    <RlButton variant="ghost" size="sm" @click="cancelEdit">{{ t('cancel') }}</RlButton>
  </div>
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

.rl-channel-composer--gone {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-sm);
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
