import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlInlineEdit from './RlInlineEdit.vue'
import RlTag from './RlTag.vue'

const meta: Meta<typeof RlInlineEdit> = {
  title: 'Common/Inline edit',
  component: RlInlineEdit,
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'A value that reads as text until you click it, then becomes a control ' +
          'in the same place. For a value with no always-on control that reads ' +
          'as plain text: free text, a title, a date. A value picked from a list ' +
          'should use RlOptionSelect with variant="inline" instead, which is the ' +
          'same control throughout and needs no swap at all.',
      },
    },
  },
}
export default meta
type Story = StoryObj<typeof RlInlineEdit>

/** Free text. Enter or clicking away keeps the edit, Escape drops it. */
export const Text: Story = {
  render: () => ({
    components: { RlInlineEdit },
    setup() {
      const value = ref('Rowdy Bakker')
      // Held separately so Escape has something to go back to.
      const draft = ref(value.value)
      return { value, draft }
    },
    template: `
      <div style="max-width:320px">
        <RlInlineEdit
          label="Assignee"
          :empty="!value"
          @edit="draft = value"
          @commit="value = draft"
        >
          <template #read>{{ value }}</template>
          <template #edit>
            <input v-model="draft" type="text" class="rl-control rl-control--sm" aria-label="Assignee" />
          </template>
        </RlInlineEdit>
      </div>
    `,
  }),
}

/** With nothing to show, the trigger reads as a placeholder rather than a gap. */
export const Empty: Story = {
  render: () => ({
    components: { RlInlineEdit },
    setup() {
      const value = ref('')
      const draft = ref('')
      return { value, draft }
    },
    template: `
      <div style="max-width:320px">
        <RlInlineEdit
          label="Due date"
          placeholder="Add a due date"
          :empty="!value"
          @edit="draft = value"
          @commit="value = draft"
        >
          <template #read>{{ value }}</template>
          <template #edit>
            <input v-model="draft" type="text" class="rl-control rl-control--sm" aria-label="Due date" />
          </template>
        </RlInlineEdit>
      </div>
    `,
  }),
}

/** Disabled drops the affordance entirely and leaves the value as text. */
export const Disabled: Story = {
  render: () => ({
    components: { RlInlineEdit, RlTag },
    template: `
      <div style="max-width:320px">
        <RlInlineEdit label="Project" disabled>
          <template #read><RlTag label="Atlas" color="blue" /></template>
        </RlInlineEdit>
      </div>
    `,
  }),
}

/**
 * `trigger="explicit"` is for a read view that is rendered content rather
 * than a value: it has links, checkboxes and comment markers inside it, all
 * of which a wrapping button would swallow, and which nesting inside one
 * would make invalid HTML.
 *
 * The content renders plainly. A click on it still starts the edit, but not
 * when it lands on something interactive, and not when it ends a text
 * selection — dragging across the prose to select it is reading, not editing.
 * The edit button beside it is the reliable way in and stays in the tab
 * order, so a keyboard user is never left without one.
 *
 * A click the content has already handled is left alone too: the highlight
 * below opens a comment thread through a delegated handler that calls
 * `preventDefault`, and no selector or marker names it. Whatever the content
 * did with the click, it was not a request to edit.
 *
 * Try it: follow the link, tick the checkbox, open the highlight, select a
 * sentence, then click the plain text or press the edit button.
 */
export const ExplicitTrigger: Story = {
  render: () => ({
    components: { RlInlineEdit },
    setup() {
      const done = ref(false)
      const thread = ref('')
      const body = ref(
        'Atlas needs a detail view that reads as a document, not a form. ' +
          'See the design notes for the brief.',
      )
      const draft = ref(body.value)

      /*
       * Delegated, the way a body full of comment highlights is handled: one
       * listener on the content rather than one per mark. Calling
       * `preventDefault` is what tells the inline edit the click was spent.
       */
      function onBodyClick(event: MouseEvent) {
        const mark = (event.target as Element | null)?.closest('mark[data-comment-id]')
        if (!mark) return
        event.preventDefault()
        thread.value = mark.getAttribute('data-comment-id') ?? ''
      }

      return { body, draft, done, thread, onBodyClick }
    },
    template: `
      <div style="max-width:420px">
        <RlInlineEdit
          block
          trigger="explicit"
          label="Description"
          @edit="draft = body"
          @commit="body = draft"
        >
          <template #read>
            <div @click="onBodyClick">
              <p style="margin:0 0 8px">
                <mark data-comment-id="c1" style="cursor:pointer">Atlas needs a detail view</mark>
                that reads as a document, not a form.
                See the <a href="#atlas-notes">design notes</a> for the brief.
              </p>
              <label style="display:flex; align-items:center; gap:8px">
                <input v-model="done" type="checkbox" />
                <span>Brief reviewed</span>
              </label>

              <p v-if="thread" style="margin:8px 0 0; color:var(--rl-color-text-subtle)">
                Thread {{ thread }} is open.
              </p>
            </div>
          </template>

          <template #edit>
            <textarea
              v-model="draft"
              rows="4"
              class="rl-control"
              aria-label="Description"
              style="width:100%"
            ></textarea>
          </template>
        </RlInlineEdit>
      </div>
    `,
  }),
}

/**
 * `keepOpenWithin` names elements that count as inside the editor while the
 * edit is going on. An editor with a floating toolbar or a mention menu renders
 * those at the end of the body rather than inside itself, so clicking one
 * would otherwise look like leaving the edit and commit halfway through
 * using it.
 *
 * The toolbar here is teleported to the body, the way a real one is. Without
 * the prop, pressing one of its buttons would end the edit.
 *
 * Its rows are non-focusable divs, which is the harder case: pressing one
 * moves no focus, so focus alone can never say when the edit is over. A
 * press outside every part of the edit is what ends it.
 */
export const FloatingToolbar: Story = {
  render: () => ({
    components: { RlInlineEdit },
    setup() {
      const value = ref('A note with a toolbar that lives outside the editor.')
      const draft = ref(value.value)
      const toolbar = ref<HTMLElement | null>(null)
      // Read at focusout rather than captured once, so it answers for
      // whatever is mounted at that moment.
      const keepOpenWithin = () => (toolbar.value ? [toolbar.value] : [])
      const append = (text: string) => {
        draft.value += text
      }
      return { value, draft, toolbar, keepOpenWithin, append }
    },
    template: `
      <div style="max-width:420px">
        <RlInlineEdit
          block
          label="Note"
          :keep-open-within="keepOpenWithin"
          @edit="draft = value"
          @commit="value = draft"
        >
          <template #read>{{ value }}</template>

          <template #edit>
            <div style="width:100%">
              <textarea
                v-model="draft"
                rows="3"
                class="rl-control"
                aria-label="Note"
                style="width:100%"
              ></textarea>

              <Teleport to="body">
                <div
                  ref="toolbar"
                  class="rl-panel"
                  style="position:fixed; left:16px; bottom:16px; display:flex; gap:8px; padding:8px"
                >
                  <button type="button" class="rl-control rl-control--sm" @click="append(' **bold**')">
                    Bold
                  </button>
                  <button type="button" class="rl-control rl-control--sm" @click="append(' @Hanna')">
                    Mention
                  </button>
                  <!-- Non-focusable, the way a drawn option list's rows are. -->
                  <div
                    role="option"
                    aria-selected="false"
                    style="padding:4px 8px; cursor:pointer"
                    @click="append(' #atlas')"
                  >
                    Tag
                  </div>
                </div>
              </Teleport>
            </div>
          </template>
        </RlInlineEdit>
      </div>
    `,
  }),
}
