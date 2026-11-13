<script setup lang="ts">
/**
 * The items of a message's object list as cards: `display: cards` in
 * data-entry.yaml (TKT-GR1MXL5). Generic: `cards` names the properties and
 * fields, and nothing here knows what they mean.
 *
 * `cards.body`, a string or markdown property of the message, shows above
 * the cards: the app's rendered HTML when the message carries it, else the
 * text. Each item of `cards.items` is one card. `title` is its heading,
 * `badge` a boolean field shown as a tag labeled with the field's label
 * while it is true, and `plus` and `minus` two lists of points under their
 * field labels, marked + and −. Every other set field follows as
 * "label: value", in field order, so no value is lost.
 *
 * Values are text: a date or datetime field shows the app's preformatted
 * `message.itemText`, an enum field its label, a boolean yes or no. The
 * HTML of the body is the app's, already sanitized, as in RlChannelMessage.
 */
import { computed, useId } from 'vue'
import RlTag from '../common/RlTag.vue'
import type {
  ChannelEnumOption,
  ChannelMessageCards,
  ChannelMessageView,
  ChannelObjectItem,
  ChannelPropertyDef,
} from './types'
import { useMessages } from '../../composables/useMessages'

const props = defineProps<{
  message: ChannelMessageView
  cards: ChannelMessageCards
  /** The message type's properties, for the field labels and types. */
  properties: ChannelPropertyDef[]
  enumOptions?: Record<string, ChannelEnumOption[]>
}>()

const messages = useMessages()
const baseId = useId()

const itemsDef = computed(() => props.properties.find((p) => p.name === props.cards.items))
const fieldDefs = computed<ChannelPropertyDef[]>(() => itemsDef.value?.fields ?? [])

const body = computed(() => {
  const name = props.cards.body
  if (!name) return undefined
  const html = props.message.html?.[name]
  if (html !== undefined) return { html }
  const v = props.message.props?.[name]
  return typeof v === 'string' && v ? { text: v } : undefined
})

const items = computed<ChannelObjectItem[]>(() => {
  const v = props.message.props?.[props.cards.items]
  if (!Array.isArray(v)) return []
  return v.filter((x): x is ChannelObjectItem => !!x && typeof x === 'object' && !Array.isArray(x))
})

/** The fields a card shows in their own place; the rest are listed. */
const placed = computed(
  () => new Set([props.cards.title, props.cards.plus, props.cards.minus, props.cards.badge].filter(Boolean)),
)

function def(name: string | undefined): ChannelPropertyDef | undefined {
  return name ? fieldDefs.value.find((f) => f.name === name) : undefined
}

function labelOf(name: string): string {
  return def(name)?.label ?? name
}

/** One scalar of item i's field as text. */
function scalar(i: number, name: string, v: string | number | boolean): string {
  const pre = props.message.itemText?.[props.cards.items]?.[i]?.[name]
  if (pre !== undefined) return pre
  if (typeof v === 'boolean') return messages.channelText({ key: v ? 'yes' : 'no' })
  const enumType = def(name)?.enum
  const opt = enumType ? props.enumOptions?.[enumType]?.find((o) => o.value === v) : undefined
  return opt?.label ?? String(v)
}

/** A field as a list of points: a list's items, or one point for a scalar. */
function points(i: number, name: string | undefined): string[] {
  if (!name) return []
  const v = items.value[i]?.[name]
  if (v === undefined || v === '') return []
  return Array.isArray(v) ? v.filter((x) => x !== '') : [scalar(i, name, v)]
}

function title(i: number): string | undefined {
  const name = props.cards.title
  const v = name ? items.value[i]?.[name] : undefined
  if (v === undefined || v === '') return undefined
  return Array.isArray(v) ? v.join(', ') : scalar(i, name!, v)
}

function badge(i: number): boolean {
  const name = props.cards.badge
  return !!name && items.value[i]?.[name] === true
}

/** The set fields no card place shows, as label and text, in field order. */
function rest(i: number): { name: string; label: string; text: string }[] {
  const item = items.value[i] ?? {}
  const names = fieldDefs.value.length ? fieldDefs.value.map((f) => f.name) : Object.keys(item).sort()
  return names
    .filter((n) => !placed.value.has(n) && item[n] !== undefined && item[n] !== '')
    .map((n) => {
      const v = item[n]!
      return { name: n, label: labelOf(n), text: Array.isArray(v) ? v.join(', ') : scalar(i, n, v) }
    })
}
</script>

<template>
  <div class="rl-channel-cards">
    <!-- Sanitized by the app before it reaches this component; see the header comment. -->
    <div v-if="body?.html !== undefined" class="rl-channel-cards__body rl-channel-cards__md" v-html="body.html" />
    <p v-else-if="body?.text" class="rl-channel-cards__body rl-channel-cards__plain">{{ body.text }}</p>

    <ul v-if="items.length" class="rl-channel-cards__list">
      <li
        v-for="(_, i) in items"
        :key="i"
        class="rl-channel-cards__card"
        :aria-labelledby="title(i) ? `${baseId}-${i}` : undefined"
      >
        <div v-if="title(i) || badge(i)" class="rl-channel-cards__head">
          <span v-if="title(i)" :id="`${baseId}-${i}`" class="rl-channel-cards__title">{{ title(i) }}</span>
          <RlTag v-if="badge(i)" :label="labelOf(cards.badge!)" color="green" />
        </div>
        <div v-if="points(i, cards.plus).length" class="rl-channel-cards__points">
          <span class="rl-channel-cards__points-label">{{ labelOf(cards.plus!) }}</span>
          <ul class="rl-channel-cards__plus">
            <li v-for="(p, k) in points(i, cards.plus)" :key="k">{{ p }}</li>
          </ul>
        </div>
        <div v-if="points(i, cards.minus).length" class="rl-channel-cards__points">
          <span class="rl-channel-cards__points-label">{{ labelOf(cards.minus!) }}</span>
          <ul class="rl-channel-cards__minus">
            <li v-for="(p, k) in points(i, cards.minus)" :key="k">{{ p }}</li>
          </ul>
        </div>
        <dl v-if="rest(i).length" class="rl-channel-cards__fields">
          <template v-for="f in rest(i)" :key="f.name">
            <dt>{{ f.label }}</dt>
            <dd>{{ f.text }}</dd>
          </template>
        </dl>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.rl-channel-cards {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-3);
  padding: var(--rl-space-3);
}

.rl-channel-cards__body { margin: 0; }

.rl-channel-cards__plain {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.rl-channel-cards__md {
  line-height: var(--rl-line-height-relaxed);
  overflow-wrap: anywhere;
}

.rl-channel-cards__md :deep(p) { margin: 0 0 var(--rl-space-2); }
.rl-channel-cards__md :deep(p:last-child) { margin-bottom: 0; }

.rl-channel-cards__list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: var(--rl-space-3);
  margin: 0;
  padding: 0;
  list-style: none;
}

.rl-channel-cards__card {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
  min-width: 0;
  padding: var(--rl-space-3);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-bg);
  overflow-wrap: anywhere;
}

.rl-channel-cards__head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rl-space-2);
}

.rl-channel-cards__title { font-weight: var(--rl-font-weight-semibold); }

.rl-channel-cards__points-label {
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-xs);
}

.rl-channel-cards__plus,
.rl-channel-cards__minus {
  margin: 0;
  padding-left: var(--rl-space-5);
  white-space: pre-wrap;
}

.rl-channel-cards__plus { list-style: '+ '; }
.rl-channel-cards__minus { list-style: '− '; }

.rl-channel-cards__fields {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: var(--rl-space-1) var(--rl-space-3);
  margin: 0;
}

.rl-channel-cards__fields dt { color: var(--rl-color-text-muted); }
.rl-channel-cards__fields dd { min-width: 0; margin: 0; }
</style>
