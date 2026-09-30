import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlTaskDetail from './RlTaskDetail.vue'
import RlDetailPanel from './RlDetailPanel.vue'
import RlComment from './RlComment.vue'
import RlCommentComposer from './RlCommentComposer.vue'
import {
  detailFields,
  detailDescription,
  detailAttachments,
  detailSubtasks,
  detailComments,
} from '../../fixtures'

const meta: Meta<typeof RlTaskDetail> = {
  title: 'Task/Task detail',
  component: RlTaskDetail,
}
export default meta

type Story = StoryObj<typeof RlTaskDetail>

export const Default: Story = {
  args: {
    title: 'Create awesome UX for Atlas Projects',
    fields: detailFields,
    description: detailDescription,
    subtasks: detailSubtasks,
  },
  render: (args) => ({
    components: { RlTaskDetail },
    setup: () => ({ args }),
    template: `<div style="max-width:900px"><RlTaskDetail v-bind="args" /></div>`,
  }),
}

export const InPanelWithComments: StoryObj = {
  render: () => ({
    components: { RlDetailPanel, RlTaskDetail, RlComment, RlCommentComposer },
    setup: () => ({
      fields: detailFields,
      description: detailDescription,
      attachments: detailAttachments,
      comments: detailComments,
    }),
    template: `
      <div style="height:100vh; display:flex; justify-content:flex-end">
        <RlDetailPanel>
          <RlTaskDetail
            title="Create awesome UX for Atlas Projects"
            :fields="fields"
            :description="description"
            :attachments="attachments"
          />
          <div class="rl-detail-panel-bleed" style="border-top:1px solid var(--rl-color-border); padding-block:24px">
            <RlComment v-for="c in comments" :key="c.id" :comment="c" />
          </div>
          <template #footer><RlCommentComposer /></template>
        </RlDetailPanel>
      </div>
    `,
  }),
}
