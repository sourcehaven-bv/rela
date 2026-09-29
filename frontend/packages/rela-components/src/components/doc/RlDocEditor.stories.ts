import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlDocEditor from './RlDocEditor.vue'
import RlDocToolbar from './RlDocToolbar.vue'

const meta: Meta<typeof RlDocEditor> = {
  title: 'Doc/Doc editor',
  component: RlDocEditor,
}
export default meta

type Story = StoryObj<typeof RlDocEditor>

export const Empty: Story = {
  render: () => ({
    components: { RlDocEditor, RlDocToolbar },
    template: `
      <div style="height:100vh; display:flex; flex-direction:column">
        <RlDocToolbar />
        <RlDocEditor style="flex:1" />
      </div>
    `,
  }),
}

export const WithContent: Story = {
  args: {
    title: 'MT meeting 30-09-2026',
    body: 'Agenda\n\n1. Status Atlas for Sourcehaven\n2. ISO27001 progress\n3. Hiring',
  },
}
