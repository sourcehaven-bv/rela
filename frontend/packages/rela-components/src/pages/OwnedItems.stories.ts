/**
 * Mockup: editing owned and related items inline, on the parent's page.
 *
 * Nothing is stored. Deleting an owned item offers
 * Undo; a linked entity is shared, so it is unlinked rather than deleted and
 * says who else sees its edits.
 */
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlOwnedItemsMockup from '../fixtures/RlOwnedItemsMockup.vue'
import RlAppShell from '../components/layout/RlAppShell.vue'
import RlSidebar from '../components/layout/RlSidebar.vue'
import RlDetailPanel from '../components/task/RlDetailPanel.vue'
import RlTaskDetail from '../components/task/RlTaskDetail.vue'
import { navGroups, detailFields } from '../fixtures'

const meta: Meta<typeof RlOwnedItemsMockup> = {
  title: 'Mockups/Owned and related items',
  component: RlOwnedItemsMockup,
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj<typeof meta>

/** Owned items of one type, edited as table rows: quote lines. */
export const LineItems: Story = { args: { pattern: 'table' } }

/** Owned items of several types, edited as cards: a page built from blocks. */
export const Blocks: Story = { args: { pattern: 'blocks' } }

/** Linked entities the parent does not own: shared, so edits save at once. */
export const LinkedEntities: Story = { args: { pattern: 'related' } }

/** All three on one entity page. */
export const OnEntityPage: StoryObj = {
  render: () => ({
    components: { RlAppShell, RlSidebar, RlDetailPanel, RlTaskDetail, RlOwnedItemsMockup },
    setup: () => ({ navGroups, detailFields, navOpen: ref(false) }),
    template: `
      <RlAppShell v-model:nav-open="navOpen" :sidebar-default-width="260">
        <template #sidebar>
          <RlSidebar workspace-name="Atlas Projects" :groups="navGroups" active-id="atlas" @close="navOpen = false" />
        </template>
        <RlDetailPanel variant="page" :show-navigation="false" :show-expand="false" show-link>
          <div style="max-width:960px">
            <RlTaskDetail title="QUO-0042 · Rela rollout for Acme" :fields="detailFields" :show-empty-fields-toggle="false">
              <RlOwnedItemsMockup />
            </RlTaskDetail>
          </div>
        </RlDetailPanel>
      </RlAppShell>
    `,
  }),
  parameters: { layout: 'fullscreen' },
}
