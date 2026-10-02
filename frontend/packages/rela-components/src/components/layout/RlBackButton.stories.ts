import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { markRaw } from 'vue'
import RlBackButton from './RlBackButton.vue'

/**
 * Stands in for `vue-router`'s RouterLink, so the story shows the real call
 * shape while the library keeps no router dependency of its own.
 *
 * It resolves `href` from `to` and binds it explicitly on its own root, which
 * is what the real component does and what makes this story worth having: a
 * stub that let the href arrive by fallthrough would render correctly even if
 * the back button passed an `href` of its own, because fallthrough loses to an
 * explicit binding. Binding it here means a regression shows up as a link with
 * no URL rather than passing quietly.
 */
const RouterLink = markRaw({
  props: { to: { type: [String, Object], required: true } },
  template: `<a :href="typeof to === 'string' ? to : to.path"><slot /></a>`,
})

const meta: Meta<typeof RlBackButton> = {
  title: 'Layout/BackButton',
  component: RlBackButton,
  parameters: { layout: 'padded' },
  args: { label: 'Back to Sprint 4', href: '#' },
}
export default meta
type Story = StoryObj<typeof RlBackButton>

export const Playground: Story = {}

/** Naming the destination is the point; a bare "Back" is the fallback. */
export const Labels: Story = {
  render: () => ({
    components: { RlBackButton },
    template: `
      <div style="display:flex; flex-direction:column; gap:8px; align-items:flex-start">
        <RlBackButton label="Back to Sprint 4" href="#" />
        <RlBackButton label="Back to Open issues" href="#" />
        <RlBackButton />
      </div>
    `,
  }),
}

/**
 * An app with its own link component passes it as `as`, and its `to` falls
 * through to it.
 *
 * This is the alternative to resolving the destination to an `href` first.
 * `router.resolve(to).href` makes the calling component depend on a router
 * that can resolve, which a partially mocked router in a test cannot, so the
 * component becomes untestable for a reason that has nothing to do with it.
 */
export const AsRouterLink: Story = {
  render: () => ({
    components: { RlBackButton },
    setup: () => ({ RouterLink }),
    template: `<RlBackButton :as="RouterLink" :to="{ path: '/tickets' }" label="All Tickets" />`,
  }),
}
