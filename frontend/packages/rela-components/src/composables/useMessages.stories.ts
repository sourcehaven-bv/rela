import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, within } from 'storybook/test'
import RlBulkActionBar from '../components/data/RlBulkActionBar.vue'
import RlButton from '../components/common/RlButton.vue'
import { provideMessages } from './useMessages'

/**
 * The seam for the strings this library composes itself.
 *
 * A label, a placeholder or a button's caption arrives as a prop, so it is
 * already in the app's language. What is left are the strings a component
 * builds around a value it only knows at render (`3 tasks selected`) and the
 * screen-reader announcements that have no visible element to hang a prop on.
 *
 * Those are the ones worth a seam. An English label in a Dutch app is reported
 * on the first screenshot; an English announcement inside an `aria-live` region
 * is heard only by the users least likely to be asked, and looks perfect to
 * everyone reviewing it.
 */
const meta: Meta = {
  title: 'Foundations/Messages',
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj

/** The defaults, with nothing provided. Every component works unwired. */
export const English: Story = {
  render: () => ({
    components: { RlBulkActionBar, RlButton },
    template: `
      <div style="position:relative; height:120px">
        <RlBulkActionBar :count="3" label="task">
          <RlButton size="sm" variant="secondary">Move</RlButton>
        </RlBulkActionBar>
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('3 tasks selected')).toBeInTheDocument()
  },
}

/**
 * The same bar under a provided set. `provideMessages` is merged over the
 * defaults, so an app translates what it cares about and anything it misses —
 * including a string a later version of this library adds — keeps working in
 * English rather than rendering blank.
 *
 * Note the plural: the function receives the count and returns the finished
 * string, so a language with more than two plural forms can express them. A
 * template with a `{count}` placeholder could not.
 */
export const Translated: Story = {
  render: () => ({
    components: { RlBulkActionBar, RlButton },
    setup() {
      provideMessages({
        selectedCount: ({ count, noun }) =>
          `${count} ${count === 1 ? noun : `${noun}s`} geselecteerd`,
      })
    },
    template: `
      <div style="position:relative; height:120px">
        <RlBulkActionBar :count="3" label="taak">
          <RlButton size="sm" variant="secondary">Verplaatsen</RlButton>
        </RlBulkActionBar>
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('3 taaks geselecteerd')).toBeInTheDocument()
    await expect(canvas.queryByText(/selected/)).toBeNull()
  },
}

/**
 * Two languages on one page, which is the reason this is provide/inject rather
 * than a module-level record: a translator's side-by-side view, or an admin
 * screen pinned to English inside a localised app. Each subtree reads the set
 * its own ancestor provided.
 */
export const TwoAtOnce: Story = {
  render: () => ({
    components: {
      RlBulkActionBar,
      /*
       * Its own component, because `provideMessages` applies to a component's
       * descendants: calling it in this story's own setup would cover both
       * panes. A subtree wanting a different set needs its own provider.
       */
      TranslatedPane: {
        components: { RlBulkActionBar },
        setup() {
          provideMessages({
            selectedCount: ({ count }) => `${count} rijen geselecteerd`,
          })
        },
        template: `<RlBulkActionBar :count="2" label="rij" />`,
      },
    },
    template: `
      <div style="display:flex; gap:24px">
        <div style="position:relative; flex:1; height:120px">
          <RlBulkActionBar :count="2" label="row" />
        </div>
        <div style="position:relative; flex:1; height:120px">
          <TranslatedPane />
        </div>
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('2 rows selected')).toBeInTheDocument()
    await expect(canvas.getByText('2 rijen geselecteerd')).toBeInTheDocument()
  },
}
