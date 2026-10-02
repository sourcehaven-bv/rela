import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, within } from 'storybook/test'
import RlCodeBlock from './RlCodeBlock.vue'

const lua = `local function on_create(entity)
  if entity.type ~= "Control" then
    return
  end
  entity.props.reviewed = false
end`

const meta: Meta<typeof RlCodeBlock> = {
  title: 'Data/Code block',
  component: RlCodeBlock,
  parameters: { layout: 'padded' },
  args: { code: lua, language: 'lua' },
  decorators: [() => ({ template: '<div style="max-width:620px"><story /></div>' })],
}
export default meta
type Story = StoryObj<typeof RlCodeBlock>

export const Playground: Story = {}

/** A caption for the file the snippet came from. It replaces the language label. */
export const WithTitle: Story = {
  args: { title: 'hooks/on_create.lua' },
}

/** Numbered, for when something else refers to a line. */
export const LineNumbers: Story = {
  args: { lineNumbers: true },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    /* Six lines in the fixture, so six numbers. */
    await expect(canvas.getByText('6')).toBeInTheDocument()

    /*
     * The numbers are decorative: they are `aria-hidden`, so the block's text
     * as announced is the code alone. A screen reader reading "1 local
     * function" would be reading furniture.
     */
    const number = canvasElement.querySelector('.rl-code-block__number')
    await expect(number).toHaveAttribute('aria-hidden', 'true')
  },
}

/** Past `maxLines` the block scrolls, and becomes focusable so a keyboard can reach it. */
export const Scrolling: Story = {
  args: {
    code: Array.from({ length: 40 }, (_, i) => `print("line ${i + 1}")`).join('\n'),
    maxLines: 8,
  },
  play: async ({ canvasElement }) => {
    const pre = canvasElement.querySelector('.rl-code-block__pre') as HTMLElement

    /* Focusable, or a scrollable region is unreachable without a pointer. */
    await expect(pre).toHaveAttribute('tabindex', '0')
    await expect(pre.scrollHeight).toBeGreaterThan(pre.clientHeight)
  },
}

/**
 * Copying, and what it puts on the clipboard.
 *
 * The button reads from the `code` prop rather than the rendered text, which is
 * what keeps the line numbers out of the copy and lets a highlighted block copy
 * as plain source.
 */
export const Copying: Story = {
  args: { lineNumbers: true },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    /*
     * The clipboard is stubbed rather than used. `navigator.clipboard` is
     * undefined outside a secure context, and the test runner's page is one
     * such context — so driving the real API here would test the environment
     * instead of the component.
     *
     * Stubbing also lets the interesting assertion happen at all: what the
     * component put on the clipboard, which is the thing a reader cannot
     * check by looking.
     */
    const written: string[] = []
    const original = navigator.clipboard
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: async (text: string) => void written.push(text) },
      configurable: true,
    })

    try {
      await userEvent.click(canvas.getByRole('button', { name: 'Copy code' }))

      /* Confirmed in place, in a live region, rather than through a toast. */
      const status = await canvas.findByRole('status')
      await expect(status).toHaveTextContent('Copied')

      /*
       * The source, with no line numbers in it. This is the assertion worth
       * having: the numbers are rendered inside the `<code>`, so a copy that
       * read `textContent` back would silently include them.
       */
      await expect(written).toHaveLength(1)
      await expect(written[0]).toBe(lua)
      await expect(written[0]).not.toContain('1local')
    } finally {
      Object.defineProperty(navigator, 'clipboard', {
        value: original,
        configurable: true,
      })
    }
  },
}

/**
 * What happens when the clipboard is not there.
 *
 * `navigator.clipboard` is undefined outside a secure context, not merely
 * refusing, so reaching for it throws rather than rejecting. The component
 * reports that through `copy-error` and does NOT claim success: a block that
 * said "Copied" while copying nothing is the worst outcome, because the user
 * moves on believing they have the code.
 */
export const CopyUnavailable: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    const original = navigator.clipboard
    Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true })

    try {
      await userEvent.click(canvas.getByRole('button', { name: 'Copy code' }))

      /* No confirmation, because nothing was copied. */
      await expect(canvas.queryByRole('status')).toBeNull()
      await expect(canvas.queryByText('Copied')).toBeNull()
    } finally {
      Object.defineProperty(navigator, 'clipboard', {
        value: original,
        configurable: true,
      })
    }
  },
}

/**
 * Already-highlighted markup through the default slot.
 *
 * The component keeps the frame, the copy button and the scrolling, and stops
 * styling the text. `code` is still passed, because that is what the copy
 * button uses — the highlighted markup would copy its own spans' text.
 */
export const Highlighted: Story = {
  render: () => ({
    components: { RlCodeBlock },
    setup: () => ({ lua }),
    template: `
      <RlCodeBlock :code="lua" language="lua" title="hooks/on_create.lua">
        <span style="color:var(--rl-tag-purple-fg)">local function</span> on_create(entity)
  <span style="color:var(--rl-tag-purple-fg)">if</span> entity.type ~= <span style="color:var(--rl-tag-green-fg)">"Control"</span> <span style="color:var(--rl-tag-purple-fg)">then</span>
    <span style="color:var(--rl-tag-purple-fg)">return</span>
  <span style="color:var(--rl-tag-purple-fg)">end</span>
  entity.props.reviewed = <span style="color:var(--rl-tag-blue-fg)">false</span>
<span style="color:var(--rl-tag-purple-fg)">end</span>
      </RlCodeBlock>
    `,
  }),
}

/** No copy control, for a block that is an illustration rather than something to take. */
export const NotCopyable: Story = {
  args: { copyable: false, title: 'Rejected, for reference' },
}
