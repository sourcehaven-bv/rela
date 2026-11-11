import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, fn, userEvent, within } from 'storybook/test'
import { ref } from 'vue'
import RlChannelMessage from './RlChannelMessage.vue'
import RlChannelQuestions from './RlChannelQuestions.vue'
import type { ChannelQuestionState } from './types'
import { pickBatch, pickChoices, pickMessageTypes, pickPresentation } from '../../fixtures/channels'

/**
 * A batch of questions as one group (TKT-GR1MXL5). The story's slot renders
 * each question as the channel does; a click on a choice records the
 * answer, as the app does once the server answers.
 */
const meta: Meta<typeof RlChannelQuestions> = {
  title: 'Channel/Channel questions',
  component: RlChannelQuestions,
  parameters: { layout: 'padded' },
  args: {
    messages: pickBatch,
    choices: pickChoices,
    questions: new Map<number, ChannelQuestionState>(pickBatch.map((m) => [m.seq, { answered: false }])),
  },
  render: (args) => ({
    components: { RlChannelQuestions, RlChannelMessage },
    setup() {
      const questions = ref(new Map(args.questions))
      const onChoose = fn((key: string) => {
        const [, seq, i] = key.split(':')
        const s = Number(seq)
        const label = pickChoices.get(s)?.[Number(i)]?.label
        const next = new Map(questions.value)
        next.set(s, { answered: true, answer: label, answerSeq: (questions.value.get(s)?.answerSeq ?? 100) + 1 })
        questions.value = next
      })
      return { args, questions, onChoose, type: pickMessageTypes[0], presentation: pickPresentation }
    },
    template: `
      <RlChannelQuestions :messages="args.messages" :choices="args.choices" :questions="questions">
        <template #message="{ message, reopened }">
          <RlChannelMessage
            :message="message"
            :message-type="type"
            :presentation="presentation"
            :choices="args.choices.get(message.seq)"
            :question="reopened ? undefined : questions.get(message.seq)"
            @choose="onChoose"
          />
        </template>
      </RlChannelQuestions>`,
  }),
}
export default meta
type Story = StoryObj<typeof RlChannelQuestions>

/**
 * Three open questions, each headed "Question k of 3". Answering the first
 * collapses it to one line with the answer, and focus moves there.
 */
export const Open: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const list = canvas.getByRole('list', { name: 'Questions' })
    await expect(within(list).getByText('Question 1 of 3')).toBeVisible()
    await expect(within(list).getByText('Question 3 of 3')).toBeVisible()
    const first = canvasElement.querySelector<HTMLElement>('[data-question="30"]')!
    await userEvent.click(within(first).getByRole('button', { name: /SQLite/ }))
    await expect(within(first).getByText('Answer: SQLite')).toBeVisible()
    await expect(within(first).queryByRole('group', { name: 'Choices' })).toBeNull()
  },
}

/**
 * Change reopens an answered question: its choices show again and the
 * first takes focus; a new answer collapses it again.
 */
export const ChangeAnswer: Story = {
  args: {
    questions: new Map<number, ChannelQuestionState>([
      [30, { answered: true, answer: 'PostgreSQL', answerSeq: 33 }],
      [31, { answered: false }],
      [32, { answered: false }],
    ]),
  },
  play: async ({ canvasElement }) => {
    const first = canvasElement.querySelector<HTMLElement>('[data-question="30"]')!
    await expect(within(first).getByText('Answer: PostgreSQL')).toBeVisible()
    await userEvent.click(within(first).getByRole('button', { name: /Change/ }))
    const group = within(first).getByRole('group', { name: 'Choices' })
    await expect(within(group).getAllByRole('button')[0]).toHaveFocus()
    await userEvent.keyboard('{Tab}{Enter}')
    await expect(within(first).getByText('Answer: SQLite')).toBeVisible()
  },
}

/** Every question answered: three lines. */
export const AllAnswered: Story = {
  args: {
    questions: new Map<number, ChannelQuestionState>([
      [30, { answered: true, answer: 'PostgreSQL', answerSeq: 33 }],
      [31, { answered: true, answer: 'Drop', answerSeq: 34 }],
      [32, { answered: true }],
    ]),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('Answer: Drop')).toBeVisible()
    await expect(canvas.getByText('Answered')).toBeVisible()
    await expect(canvas.getAllByRole('button', { name: /Change/ })).toHaveLength(3)
  },
}
