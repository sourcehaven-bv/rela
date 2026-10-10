import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlTrackChangesMockup from '../fixtures/RlTrackChangesMockup.vue'
import RlSuggestableDescription from '../fixtures/RlSuggestableDescription.vue'
import RlAppShell from '../components/layout/RlAppShell.vue'
import RlSidebar from '../components/layout/RlSidebar.vue'
import RlDetailPanel from '../components/task/RlDetailPanel.vue'
import RlTaskDetail from '../components/task/RlTaskDetail.vue'
import RlIconButton from '../components/common/RlIconButton.vue'
import { navGroups, detailFields } from '../fixtures'

/*
 * Written the way rela's serializer writes markdown (`-` bullets, `_italic_`,
 * one line per paragraph), so opening the body in the editor changes nothing.
 */
const BODY = `# Onboarding guide

New team members start with this page. It explains how to get access and who to ask for help.

## Access

- Request a laptop from IT
- Ask your manager for a GitHub account
- Join the #general channel

## First week

Spend the first week reading the architecture notes and pairing with a colleague. The setup script installs everything you need.

> Ask questions early. Nobody expects you to know the codebase yet.

## Old VPN instructions

Connect to the VPN with the legacy client before you clone anything.
`

const EDITED = `# Onboarding guide

New team members start with this page. It explains how to get **access** and who to ask for help.

## Access and accounts

- Request a laptop from IT
- Ask your manager for a GitHub account and an SSO login
- Join the #general and #help channels
- Book an intro call with the security team

## First week

Spend the first two weeks reading the architecture notes and pairing with a colleague. Run \`just setup\` to install everything you need.

> Ask questions early. Nobody expects you to know the codebase yet.
`

const meta: Meta<typeof RlTrackChangesMockup> = {
  title: 'Mockups/Track changes',
  component: RlTrackChangesMockup,
  parameters: { layout: 'padded' },
  args: { body: BODY },
}
export default meta
type Story = StoryObj<typeof RlTrackChangesMockup>

/**
 * A reviewer opens a body with one posted edit session. Every hunk is drawn
 * inline and has a card in the margin. Hover a card to find its change, click
 * a change to find its card. "With changes" and "Original" preview the body
 * with all of them accepted or all rejected.
 */
export const Review: Story = {
  args: {
    changeSets: [
      { id: 'hanna', author: 'Hanna de Vries', when: '2 hours ago', base: BODY, edited: EDITED },
    ],
  },
}

/**
 * The author side. "Suggest edits" opens the normal editor; nothing is saved.
 * Posting diffs the edit against the body and turns each hunk into a
 * suggestion, which lands in the review view.
 */
export const Suggesting: Story = {
  args: { startSuggesting: true },
}

/** A body nobody has suggested changes to yet. */
export const NoSuggestions: Story = {}

/*
 * The same section on the task detail page. A pen beside the heading opens a
 * menu of modes; clicking the text edits directly, as inline editing does
 * elsewhere. Click a marked change to open its card.
 */
function detailPage(canEdit = true, withSuggestions = true) {
  return {
    render: () => ({
      components: {
        RlAppShell,
        RlSidebar,
        RlDetailPanel,
        RlTaskDetail,
        RlIconButton,
        RlSuggestableDescription,
      },
      setup: () => ({
        navGroups,
        detailFields,
        navOpen: ref(false),
        canEdit,
        body: BODY,
        sets: withSuggestions
          ? [
              {
                id: 'hanna',
                author: 'Hanna de Vries',
                when: '2 hours ago',
                base: BODY,
                edited: EDITED,
              },
            ]
          : [],
      }),
      template: `
        <RlAppShell v-model:nav-open="navOpen" :sidebar-default-width="260">
          <template #sidebar>
            <RlSidebar workspace-name="Atlas Projects" :groups="navGroups" active-id="atlas" @close="navOpen = false" />
          </template>
          <RlDetailPanel variant="page" :show-navigation="false" :show-expand="false" show-link>
            <div style="max-width:900px">
              <RlTaskDetail title="Write the onboarding guide" :fields="detailFields" :show-empty-fields-toggle="false">
                <RlSuggestableDescription :body="body" :change-sets="sets" :can-edit="canEdit" />
              </RlTaskDetail>
            </div>
          </RlDetailPanel>
        </RlAppShell>
      `,
    }),
    parameters: { layout: 'fullscreen' },
  } satisfies StoryObj
}

/** Suggestions from one edit session, reviewed in place. */
export const PageWithSuggestions = detailPage()

/** Before anyone has suggested anything. */
export const PageWithoutSuggestions = detailPage(true, false)

/** A reader who may comment but not edit. There is no choice to make, so there is no menu. */
export const PageForCommentOnlyReader = detailPage(false)
