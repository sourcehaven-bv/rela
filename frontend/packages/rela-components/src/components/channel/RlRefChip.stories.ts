import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, fn, userEvent, within } from 'storybook/test'
import RlRefChip from './RlRefChip.vue'

const meta: Meta<typeof RlRefChip> = {
  title: 'Channel/Ref chip',
  component: RlRefChip,
  parameters: { layout: 'padded' },
  args: {
    value: { id: 'RISK-88', type: 'risk', title: 'Unencrypted export storage' },
    href: '#/entity/RISK-88',
    onOpen: fn(),
  },
}
export default meta
type Story = StoryObj<typeof RlRefChip>

/** A ref the user may read: a real link to the entity, with its title. */
export const Readable: Story = {
  play: async ({ canvasElement }) => {
    const link = within(canvasElement).getByRole('link')
    await expect(link).toHaveAttribute('href', '#/entity/RISK-88')
    await expect(link).toHaveTextContent('RISK-88')
    await expect(link).toHaveTextContent('Unencrypted export storage')
  },
}

/**
 * A ref the user may not read shows a fixed placeholder. No id, type, title or
 * link reaches the page, because each of them is something the server withheld.
 */
export const Restricted: Story = {
  args: { value: { restricted: true }, href: undefined },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('Restricted reference')).toBeVisible()
    await expect(canvas.queryByRole('link')).toBeNull()
    await expect(canvas.queryByRole('button')).toBeNull()
    await expect(canvasElement.textContent?.trim()).toBe('Restricted reference')
  },
}

/** Without an href the chip is a button, and the app navigates on `open`. */
export const WithoutHref: Story = {
  args: { href: undefined },
  play: async ({ args, canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole('button', { name: /RISK-88/ }))
    await expect(args.onOpen).toHaveBeenCalledWith(
      { id: 'RISK-88', type: 'risk', title: 'Unencrypted export storage' },
      expect.anything(),
    )
  },
}

/** A title written as markup is text: no element is created from it. */
export const MarkupInTitle: Story = {
  args: { value: { id: 'TKT-1', type: 'ticket', title: '<img src=x onerror="alert(1)">' } },
  play: async ({ canvasElement }) => {
    await expect(canvasElement.querySelector('img')).toBeNull()
    await expect(within(canvasElement).getByRole('link')).toHaveTextContent('<img src=x onerror="alert(1)">')
  },
}
