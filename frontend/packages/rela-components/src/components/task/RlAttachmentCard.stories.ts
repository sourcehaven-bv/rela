import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlAttachmentCard from './RlAttachmentCard.vue'
import RlAttachmentList from './RlAttachmentList.vue'

const meta: Meta<typeof RlAttachmentCard> = {
  title: 'Task/Attachment card',
  component: RlAttachmentCard,
  parameters: { layout: 'padded' },
  args: { attachment: { id: 'a', name: 'Design brief.pdf', kind: 'pdf' } },
}
export default meta
type Story = StoryObj<typeof RlAttachmentCard>

export const Playground: Story = {}

/** One card per file kind; the colour and label come from `kind`. */
export const AllKinds: Story = {
  render: () => ({
    components: { RlAttachmentCard },
    setup: () => ({
      items: [
        { id: '1', name: 'Design brief.pdf', kind: 'pdf' },
        { id: '2', name: 'Research notes.docx', kind: 'doc' },
        { id: '3', name: 'Budget.xlsx', kind: 'sheet' },
        { id: '4', name: 'Mockup.png', kind: 'image' },
        { id: '5', name: 'archive.zip', kind: 'other' },
      ],
    }),
    template: `
      <div style="display:flex; gap:12px; flex-wrap:wrap">
        <RlAttachmentCard v-for="a in items" :key="a.id" :attachment="a" />
      </div>
    `,
  }),
}

/** With an `href` the card downloads the file; `removable` adds a remove button. */
export const DownloadAndRemove: Story = {
  render: () => ({
    components: { RlAttachmentCard, RlAttachmentList },
    setup: () => ({
      items: [
        { id: '1', name: 'Awesome.pdf', kind: 'pdf', href: '#' },
        { id: '2', name: 'Awesome.docx', kind: 'doc', href: '#' },
        { id: '3', name: 'Notes.txt', kind: 'doc', action: 'Pending save' },
      ],
    }),
    template: `
      <RlAttachmentList>
        <RlAttachmentCard v-for="a in items" :key="a.id" :attachment="a" removable />
      </RlAttachmentList>
    `,
  }),
}
