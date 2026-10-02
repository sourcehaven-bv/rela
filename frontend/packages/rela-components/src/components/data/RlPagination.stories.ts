import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlPagination from './RlPagination.vue'

const meta: Meta<typeof RlPagination> = {
  title: 'Data/Pagination',
  component: RlPagination,
  parameters: { layout: 'padded' },
  args: { page: 1, pageCount: 12, summary: '1-25 of 289' },
  render: (args) => ({
    components: { RlPagination },
    setup: () => ({ args, page: ref(args.page) }),
    template: '<RlPagination v-bind="args" v-model:page="page" />',
  }),
}
export default meta
type Story = StoryObj<typeof RlPagination>

/** The first and last pages are always offered; the middle collapses to a gap. */
export const Playground: Story = {}

/** Mid-range, where a gap appears on both sides. */
export const MiddlePage: Story = { args: { page: 6, summary: '126-150 of 289' } }

/** Few enough pages to show them all, so no gap appears. */
export const FewPages: Story = { args: { page: 2, pageCount: 4, summary: '26-50 of 92' } }

/** Disabled while the next page is loading. */
export const Loading: Story = { args: { page: 3, disabled: true } }
