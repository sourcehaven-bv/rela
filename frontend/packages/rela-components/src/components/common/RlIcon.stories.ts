import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { computed, ref } from 'vue'
import RlIcon from './RlIcon.vue'
import { iconMeta, iconNames } from './icons'

const meta: Meta<typeof RlIcon> = {
  title: 'Common/Icon',
  component: RlIcon,
  parameters: { layout: 'padded' },
  argTypes: {
    name: { control: { type: 'select' }, options: iconNames },
    size: { control: { type: 'range', min: 12, max: 48, step: 1 } },
  },
  args: { name: 'star', size: 16 },
}
export default meta
type Story = StoryObj<typeof RlIcon>

export const Playground: Story = {}

/**
 * Every icon in the set, grouped as the catalogue groups them, with a filter.
 *
 * This is the picker: the names here are the ones a project author writes in
 * config and a developer passes to `name`, so the label under each glyph is
 * the copyable answer.
 */
export const Gallery: Story = {
  render: () => ({
    components: { RlIcon },
    setup: () => {
      const query = ref('')
      const groups = computed(() => {
        const q = query.value.trim().toLowerCase()
        const out = new Map<string, string[]>()
        for (const name of iconNames) {
          const meta = iconMeta[name]
          // Category counts as a match: "security" is a thing someone looks
          // for, and it names a group rather than any single glyph.
          const hay = `${name} ${meta.category} ${meta.description}`.toLowerCase()
          if (q && !hay.includes(q)) continue
          if (!out.has(meta.category)) out.set(meta.category, [])
          out.get(meta.category)!.push(name)
        }
        return [...out]
      })
      const total = iconNames.length
      return { query, groups, total, iconMeta }
    },
    template: `
      <div>
        <input
          v-model="query"
          class="rl-control"
          :placeholder="'Filter ' + total + ' icons by name or description'"
          style="max-width:360px; margin-bottom:20px"
        />
        <section v-for="[category, names] in groups" :key="category" style="margin-bottom:28px">
          <h3 style="font-size:12px; text-transform:uppercase; letter-spacing:.04em; color:var(--rl-color-text-subtle); margin:0 0 12px">
            {{ category }} <span style="font-weight:400">{{ names.length }}</span>
          </h3>
          <div style="display:grid; grid-template-columns:repeat(auto-fill,minmax(112px,1fr)); gap:14px">
            <div
              v-for="n in names"
              :key="n"
              :title="iconMeta[n].description"
              style="display:flex; flex-direction:column; align-items:center; gap:7px; padding:12px 6px; border:1px solid var(--rl-color-border); border-radius:var(--rl-radius-md)"
            >
              <RlIcon :name="n" :size="20" />
              <span style="font-size:11px; color:var(--rl-color-text-subtle); text-align:center; word-break:break-word">{{ n }}</span>
            </div>
          </div>
        </section>
        <p v-if="!groups.length" style="color:var(--rl-color-text-subtle)">No icon matches that.</p>
      </div>
    `,
  }),
}
