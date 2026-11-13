<script setup lang="ts">
/**
 * The questions of one batch as one group (TKT-GR1MXL5): an agent that asks
 * several questions at once posts them together, and the reader answers
 * them one by one. `groupQuestions` (channelQuestions.ts) decides which
 * messages form a group; this component only draws it.
 *
 * The group is a list named "Questions". Each item is headed "Question k of
 * N". An unanswered question renders through the `message` slot, which the
 * channel fills with the full message and its choices. An answered one
 * (`questions`) collapses to one line: its excerpt, the answer, and, while
 * the app still passes choices for it, a Change button. Change shows the
 * full message again with `reopened` set in the slot, so the channel shows
 * the choices rather than the answer, and moves focus to the first choice.
 * A new answer (`answerSeq`) collapses it again.
 *
 * The collapsed line carries the question's `data-seq`, so the channel's
 * `focusRow` finds it. When a question collapses while focus is inside it
 * (the reader just answered), focus moves to its collapsed line, so it is
 * not lost to the page.
 */
import { computed, nextTick, ref, useId, useTemplateRef, watch } from 'vue'
import RlButton from '../common/RlButton.vue'
import RlIcon from '../common/RlIcon.vue'
import type { ChannelDecisionView, ChannelMessageView, ChannelQuestionState } from './types'
import { useMessages } from '../../composables/useMessages'

const props = defineProps<{
  /** The questions in log order. */
  messages: ChannelMessageView[]
  /** Where each question stands, by seq. */
  questions?: ReadonlyMap<number, ChannelQuestionState>
  /** The reply buttons of each question, by seq; Change needs them. */
  choices?: ReadonlyMap<number, ChannelDecisionView[]>
  /** The label of a message type, the title of a question without an excerpt. */
  typeLabel?: (type: string) => string
}>()

defineSlots<{
  message(props: { message: ChannelMessageView; reopened: boolean }): unknown
}>()

const strings = useMessages()
const baseId = useId()
const list = useTemplateRef<HTMLElement>('list')

/** Reopened questions, by seq, with the answer they were reopened at. */
const reopened = ref<ReadonlyMap<number, number | undefined>>(new Map())

function isReopened(seq: number): boolean {
  return reopened.value.has(seq) && reopened.value.get(seq) === props.questions?.get(seq)?.answerSeq
}

function collapsed(seq: number): boolean {
  return !!props.questions?.get(seq)?.answered && !isReopened(seq)
}

const collapsedSeqs = computed(() => props.messages.filter((m) => collapsed(m.seq)).map((m) => m.seq))

function titleOf(m: ChannelMessageView): string {
  return m.excerpt ?? (m.type ? (props.typeLabel?.(m.type) ?? m.type) : '')
}

function answerText(seq: number): string {
  const answer = props.questions?.get(seq)?.answer
  return answer !== undefined ? strings.channelAnswer({ answer }) : strings.channelText({ key: 'answered' })
}

function item(seq: number): HTMLElement | null | undefined {
  return list.value?.querySelector<HTMLElement>(`[data-question="${seq}"]`)
}

async function reopen(seq: number) {
  const next = new Map(reopened.value)
  next.set(seq, props.questions?.get(seq)?.answerSeq)
  reopened.value = next
  await nextTick()
  item(seq)?.querySelector<HTMLElement>('.rl-channel-message__choices button:not([disabled])')?.focus()
}

// Runs before the DOM update, while the expanded question still holds focus.
watch(
  collapsedSeqs,
  (now, before) => {
    const active = document.activeElement
    const fresh = now.filter((seq) => !before?.includes(seq))
    const target = fresh.find((seq) => active && item(seq)?.contains(active))
    if (target === undefined) return
    void nextTick(() => item(target)?.querySelector<HTMLElement>('.rl-channel-questions__answered')?.focus())
  },
  { flush: 'pre' },
)
</script>

<template>
  <div class="rl-channel-questions">
    <ol ref="list" class="rl-channel-questions__list" :aria-label="strings.channelText({ key: 'questions' })">
      <li
        v-for="(m, k) in messages"
        :key="m.seq"
        class="rl-channel-questions__item"
        :data-question="m.seq"
        :aria-labelledby="`${baseId}-${m.seq}`"
      >
        <p :id="`${baseId}-${m.seq}`" class="rl-channel-questions__number">
          {{ strings.channelQuestionNumber({ index: k + 1, total: messages.length }) }}
        </p>
        <div v-if="collapsed(m.seq)" class="rl-channel-questions__answered" :data-seq="m.seq" tabindex="-1">
          <span class="rl-channel-questions__title">{{ titleOf(m) }}</span>
          <span class="rl-channel-questions__answer">
            <RlIcon name="check" :size="14" />
            <span>{{ answerText(m.seq) }}</span>
          </span>
          <RlButton v-if="choices?.get(m.seq)?.length" variant="ghost" size="sm" @click="reopen(m.seq)">
            {{ strings.channelText({ key: 'changeAnswer' })
            }}<span class="rl-visually-hidden">{{ strings.channelChangeAnswerHint() }}</span>
          </RlButton>
        </div>
        <slot v-else name="message" :message="m" :reopened="isReopened(m.seq)" />
      </li>
    </ol>
  </div>
</template>

<style scoped>
.rl-channel-questions {
  margin: var(--rl-space-2) var(--rl-space-5);
  border-left: 3px solid var(--rl-color-border-strong);
}

.rl-channel-questions__list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.rl-channel-questions__item + .rl-channel-questions__item {
  border-top: 1px solid var(--rl-color-border);
}

.rl-channel-questions__number {
  margin: 0;
  padding: var(--rl-space-2) var(--rl-space-5) 0;
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-xs);
  font-weight: var(--rl-font-weight-medium);
}

.rl-channel-questions__answered {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rl-space-2) var(--rl-space-3);
  padding: var(--rl-space-1) var(--rl-space-5) var(--rl-space-2);
  font-size: var(--rl-font-size-sm);
}

.rl-channel-questions__answered:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -2px;
}

.rl-channel-questions__title {
  min-width: 0;
  overflow-wrap: anywhere;
}

.rl-channel-questions__answer {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-1);
  color: var(--rl-color-text-muted);
  overflow-wrap: anywhere;
}
</style>
