import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlAvatar from './RlAvatar.vue'
import RlAvatarGroup from './RlAvatarGroup.vue'

const meta: Meta<typeof RlAvatar> = {
  title: 'Data/Avatar',
  component: RlAvatar,
  parameters: { layout: 'padded' },
  argTypes: { size: { control: { type: 'inline-radio' }, options: ['xs', 'sm', 'md', 'lg'] } },
  args: { name: 'Hanry Fonda', size: 'md' },
}
export default meta
type Story = StoryObj<typeof RlAvatar>

/**
 * The colour is derived from the name, so the same person is the same colour
 * everywhere without anyone assigning one.
 */
export const Playground: Story = {}

export const Sizes: Story = {
  render: () => ({
    components: { RlAvatar },
    template: `
      <div style="display:flex; align-items:center; gap:12px">
        <RlAvatar name="Hanry Fonda" size="xs" />
        <RlAvatar name="Hanry Fonda" size="sm" />
        <RlAvatar name="Hanry Fonda" size="md" />
        <RlAvatar name="Hanry Fonda" size="lg" />
      </div>
    `,
  }),
}

/** Different names take different colours from the tag palette. */
export const People: Story = {
  render: () => ({
    components: { RlAvatar },
    template: `
      <div style="display:flex; gap:8px">
        <RlAvatar name="Hanry Fonda" />
        <RlAvatar name="Ada Lovelace" />
        <RlAvatar name="Grace Hopper" />
        <RlAvatar name="Alan Turing" />
        <RlAvatar name="Katherine Johnson" />
        <RlAvatar />
      </div>
    `,
  }),
}

/**
 * The group is one labelled image rather than several, so a screen reader
 * says the whole list once instead of reading five separate avatars.
 */
export const Group: Story = {
  render: () => ({
    components: { RlAvatarGroup },
    setup: () => ({
      people: [
        { name: 'Hanry Fonda' },
        { name: 'Ada Lovelace' },
        { name: 'Grace Hopper' },
        { name: 'Alan Turing' },
        { name: 'Katherine Johnson' },
      ],
    }),
    template: '<RlAvatarGroup :people="people" :max="3" />',
  }),
}
