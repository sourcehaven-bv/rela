import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, waitFor } from 'storybook/test'
import { ref } from 'vue'
import RlMarkdownEditor from './RlMarkdownEditor.vue'
import RlBlockIcon from './RlBlockIcon.vue'

const meta: Meta<typeof RlMarkdownEditor> = {
  title: 'Editor/Markdown editor',
  component: RlMarkdownEditor,
}
export default meta

type Story = StoryObj<typeof RlMarkdownEditor>

/**
 * Resolves once the ProseMirror surface exists.
 *
 * The editor builds itself in `onMounted` behind an `await`, so a play
 * function that queried immediately would find nothing.
 */
async function waitForEditor(canvasElement: HTMLElement): Promise<HTMLElement> {
  let surface: HTMLElement | null = null
  await waitFor(() => {
    surface = canvasElement.querySelector<HTMLElement>('.ProseMirror')
    expect(surface).toBeInTheDocument()
  })
  return surface as unknown as HTMLElement
}

const SAMPLE = `# Release notes

The editor round-trips markdown: what you type here serializes back to the
text the parent holds.

## What works

- Bold, italic, strikethrough and inline code
- Lists, including nested ones
- [ ] An unchecked task
- [x] A checked one

> Quotes keep their marker through a round trip.

| Surface | Status |
| ------- | ------ |
| Toolbar | Done   |
| Tables  | Done   |

\`\`\`ts
const editor = 'milkdown'
\`\`\`
`

export const Default: Story = {
  render: () => ({
    components: { RlMarkdownEditor },
    setup() {
      const body = ref(SAMPLE)
      return { body }
    },
    template: `
      <div style="max-width:760px; margin:0 auto; padding:24px">
        <RlMarkdownEditor v-model="body" />
      </div>
    `,
  }),
}

export const Empty: Story = {
  render: () => ({
    components: { RlMarkdownEditor },
    setup() {
      const body = ref('')
      return { body }
    },
    template: `
      <div style="max-width:760px; margin:0 auto; padding:24px">
        <RlMarkdownEditor v-model="body" placeholder="Start writing..." />
      </div>
    `,
  }),
}

/** The markdown the parent holds, shown beside the editor that produces it. */
export const WithSerializedOutput: Story = {
  render: () => ({
    components: { RlMarkdownEditor },
    setup() {
      const body = ref('Type here and watch the markdown on the right.\n')
      return { body }
    },
    template: `
      <div style="display:grid; grid-template-columns:1fr 1fr; gap:16px; padding:24px">
        <RlMarkdownEditor v-model="body" />
        <pre style="margin:0; padding:12px; overflow:auto; background:var(--rl-color-bg-sunken);
                    border-radius:var(--rl-radius-md); font-size:var(--rl-font-size-sm)"
        >{{ body }}</pre>
      </div>
    `,
  }),
}

/** Without the toolbar, for a surface that supplies its own chrome. */
export const NoToolbar: Story = {
  render: () => ({
    components: { RlMarkdownEditor },
    setup() {
      const body = ref('Just the editing surface.\n')
      return { body }
    },
    template: `
      <div style="max-width:760px; margin:0 auto; padding:24px">
        <RlMarkdownEditor v-model="body" hide-toolbar />
      </div>
    `,
  }),
}

/**
 * An app button in the toolbar's `#toolbar-extra` slot.
 *
 * The real use is a mention or reference picker, which needs an app's own
 * Milkdown node passed through `plugins`. This shows the slot wiring without
 * that: the button reaches the live view and inserts through it.
 */
export const WithToolbarExtra: Story = {
  render: () => ({
    components: { RlMarkdownEditor, RlBlockIcon },
    setup() {
      const body = ref('Press the rightmost toolbar button.\n')
      function insert(view: { state: unknown; dispatch: (tr: unknown) => void } | null) {
        if (!view) return
        const state = view.state as {
          tr: { insertText: (text: string) => unknown }
        }
        view.dispatch(state.tr.insertText('@reference'))
      }
      return { body, insert }
    },
    template: `
      <div style="max-width:760px; margin:0 auto; padding:24px">
        <RlMarkdownEditor v-model="body">
          <template #toolbar-extra="{ view }">
            <button
              type="button"
              class="rl-editor-toolbar__button"
              title="Insert a reference"
              aria-label="Insert a reference"
              @mousedown.prevent
              @click="insert(view)"
            >
              <RlBlockIcon name="reference" />
            </button>
          </template>
        </RlMarkdownEditor>
      </div>
    `,
  }),
}

/**
 * Types into the editor and asserts what comes back out.
 *
 * The round trip is the property worth pinning: typing markdown shorthand must
 * produce the structure it names, and serializing must spell that structure
 * the way `serializerContract` says. Run as a story so the assertion happens
 * against a real ProseMirror in a real browser — a jsdom editor would not
 * exercise the input rules at all.
 */
export const RoundTrip: Story = {
  render: () => ({
    components: { RlMarkdownEditor },
    setup() {
      const body = ref('')
      return { body }
    },
    template: `
      <div style="padding:24px">
        <RlMarkdownEditor v-model="body" />
        <pre data-testid="output">{{ body }}</pre>
      </div>
    `,
  }),
  play: async ({ canvasElement, step }) => {
    // `Editor.make()` is awaited, so the surface appears a tick after the
    // component renders.
    const surface = await waitForEditor(canvasElement)
    const output = () => canvasElement.querySelector('[data-testid="output"]')?.textContent ?? ''

    surface.focus()

    await step('markdown shorthand becomes structure', async () => {
      // `userEvent.keyboard` dispatches the trusted key events ProseMirror's
      // input rules listen for; setting textContent would bypass them.
      await userEvent.keyboard('# Title{Enter}')
      await userEvent.keyboard('- first{Enter}second{Enter}{Enter}')
      await userEvent.keyboard('> quoted')

      await expect(surface.querySelector('h1')).toBeInTheDocument()
      await expect(surface.querySelectorAll('ul li')).toHaveLength(2)
      await expect(surface.querySelector('blockquote')).toBeInTheDocument()
    })

    await step('serializes with the pinned conventions', async () => {
      // Milkdown's markdown listener is debounced, so the emitted value
      // trails the last keystroke.
      await waitFor(() => expect(output()).toContain('# Title'))

      const markdown = output()
      // `-` bullets and ATX headings, per DEFAULT_STRINGIFY_OPTIONS. A `*`
      // bullet or a setext heading here means the serializer options were
      // dropped somewhere in the ctx merge.
      await expect(markdown).toContain('- first')
      await expect(markdown).toContain('- second')
      await expect(markdown).toContain('> quoted')
      await expect(markdown).not.toContain('* first')
      await expect(markdown).not.toContain('===')
    })
  },
}

/** The toolbar reflects, and changes, the formatting at the cursor. */
export const ToolbarTogglesFormatting: Story = {
  render: () => ({
    components: { RlMarkdownEditor },
    setup() {
      const body = ref('plain paragraph\n')
      return { body }
    },
    template: `
      <div style="padding:24px">
        <RlMarkdownEditor v-model="body" />
        <pre data-testid="output">{{ body }}</pre>
      </div>
    `,
  }),
  play: async ({ canvasElement, step }) => {
    const surface = await waitForEditor(canvasElement)
    const button = (label: string) =>
      canvasElement.querySelector<HTMLButtonElement>(`.rl-editor-toolbar__button[aria-label="${label}"]`)
    const output = () => canvasElement.querySelector('[data-testid="output"]')?.textContent ?? ''

    surface.focus()
    await userEvent.click(surface.querySelector('p') as HTMLElement)

    await step('a block command lights up once it applies', async () => {
      const h2 = button('Heading 2')
      await expect(h2).toHaveAttribute('aria-pressed', 'false')

      await userEvent.click(h2 as HTMLButtonElement)
      await waitFor(() => expect(surface.querySelector('h2')).toBeInTheDocument())
      await expect(button('Heading 2')).toHaveAttribute('aria-pressed', 'true')
      await waitFor(() => expect(output()).toContain('## plain paragraph'))
    })

    await step('pressing it again toggles back, rather than no-opping', async () => {
      // The case `toggleTo` exists for: Milkdown's WrapInHeading is one-way,
      // so without an inverse the second press would appear to do nothing.
      await userEvent.click(button('Heading 2') as HTMLButtonElement)
      await waitFor(() => expect(surface.querySelector('h2')).not.toBeInTheDocument())

      // The reversal must REACH the parent. Reverting an edit lands back on
      // the markdown the editor was loaded with, so an emit guard that
      // compares against the loaded value drops this one and leaves the
      // parent holding the heading it no longer has.
      await waitFor(() => expect(output()).not.toContain('## plain paragraph'))
      await expect(output()).toContain('plain paragraph')
    })
  },
}
