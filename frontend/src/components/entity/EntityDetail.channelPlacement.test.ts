import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { defineComponent, h, onMounted } from 'vue'
import EntityDetail from './EntityDetail.vue'
import { useSchemaStore } from '@/stores/schema'
import type { Entity } from '@/types'
import type { ViewResponse, ViewSection } from '@/api'

// Where EntityDetail puts the channels blocks (TKT-VXBE16H): the
// above_relations block before the first view section that does not show the
// entry, the default block after the comments. ChannelsPanel is stubbed; its
// own tests cover which channel types each block shows.

const fetchViewMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  fetchView: (...a: unknown[]) => fetchViewMock(...a),
  getCommands: async () => [],
}))
type Handler = (data: { type: string }) => void
const handlers = new Set<Handler>()
vi.mock('@/composables/useEvents', async (orig) => ({
  ...(await orig<typeof import('@/composables/useEvents')>()),
  useEvents: () => ({
    on: (event: string, fn: Handler) => {
      if (event === 'entity:changed') handlers.add(fn)
    },
    off: (event: string, fn: Handler) => {
      if (event === 'entity:changed') handlers.delete(fn)
    },
  }),
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/entity/topic/TOPIC-1', name: 'entity' }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

/** The placement of each ChannelsPanel mounted, in order. */
const mounts: string[] = []

const ChannelsPanelStub = defineComponent({
  name: 'ChannelsPanel',
  props: { placement: { type: String, default: 'end' } },
  setup: (props) => {
    onMounted(() => mounts.push(props.placement))
    return () => h('div', { class: 'channels-stub', 'data-placement': props.placement })
  },
})

function section(sectionId: string, entry: boolean): ViewSection {
  return {
    heading: sectionId,
    sectionId,
    display: entry ? 'content' : 'cards',
    isEmpty: false,
    isGrouped: false,
    hasContent: entry,
    ...(entry ? { source: 'entry' } : { source: 'subs', entities: [] }),
  } as ViewSection
}

function view(sections: ViewSection[]): ViewResponse {
  const entry = {
    id: 'TOPIC-1',
    type: 'topic',
    _title: 'A topic',
    content: 'Body',
    properties: { titel: 'A topic' },
    _actions: { update: true },
  } as unknown as Entity
  return { entry, sections }
}

let wrapper: VueWrapper | undefined
let pinia: ReturnType<typeof createPinia>

beforeEach(() => {
  mounts.length = 0
  handlers.clear()
  pinia = createPinia()
  setActivePinia(pinia)
  useSchemaStore().entityTypes.set('topic', {
    name: 'topic',
    label: 'Topic',
    properties: { titel: { type: 'string', values: null } },
  } as never)
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  document.body.innerHTML = ''
})

/** The page order of the view sections and channels blocks. */
async function order(sections: ViewSection[]): Promise<string[]> {
  fetchViewMock.mockReset().mockResolvedValue(view(sections))
  wrapper = mount(EntityDetail, {
    props: { entityType: 'topic', entityId: 'TOPIC-1' },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada], stubs: { ChannelsPanel: ChannelsPanelStub } },
  })
  await flushPromises()
  return wrapper
    .findAll('.view-section, .channels-stub')
    .map((el) =>
      el.classes('channels-stub')
        ? `channels:${el.attributes('data-placement')}`
        : el.attributes('id')!
    )
}

describe('EntityDetail channel placement', () => {
  it('puts the above_relations block before the first relation section', async () => {
    expect(
      await order([
        section('props', true),
        section('body', true),
        section('subs', false),
        section('deps', false),
      ])
    ).toEqual(['props', 'body', 'channels:above_relations', 'subs', 'deps', 'channels:end'])
  })

  it('puts it first when the first section is a relation', async () => {
    expect(await order([section('subs', false), section('body', true)])).toEqual([
      'channels:above_relations',
      'subs',
      'body',
      'channels:end',
    ])
  })

  it('puts it after the last section when every section shows the entry', async () => {
    expect(await order([section('props', true), section('body', true)])).toEqual([
      'props',
      'body',
      'channels:above_relations',
      'channels:end',
    ])
  })

  it('renders one block of each kind', async () => {
    const got = await order([section('props', true), section('subs', false)])
    expect(got.filter((x) => x === 'channels:above_relations')).toHaveLength(1)
    expect(got.filter((x) => x === 'channels:end')).toHaveLength(1)
  })
})

describe('EntityDetail channel placement on reload', () => {
  it('moves the above_relations block, not remounts it, when the sections change', async () => {
    vi.useFakeTimers()
    try {
      const before = [section('body', true), section('subs', false)]
      expect(await order(before)).toEqual([
        'body',
        'channels:above_relations',
        'subs',
        'channels:end',
      ])
      expect(mounts.sort()).toEqual(['above_relations', 'end'])

      // A reload on the same entity adds a relation section before `subs`,
      // and an entry section after the body: the block moves down one.
      fetchViewMock.mockResolvedValue(
        view([
          section('body', true),
          section('props', true),
          section('deps', false),
          section('subs', false),
        ])
      )
      for (const fn of handlers) fn({ type: 'topic' })
      await vi.advanceTimersByTimeAsync(2000)
      await flushPromises()

      const after = wrapper!
        .findAll('.view-section, .channels-stub')
        .map((el) =>
          el.classes('channels-stub')
            ? `channels:${el.attributes('data-placement')}`
            : el.attributes('id')!
        )
      expect(after).toEqual([
        'body',
        'props',
        'channels:above_relations',
        'deps',
        'subs',
        'channels:end',
      ])
      expect(mounts).toHaveLength(2)
    } finally {
      vi.useRealTimers()
    }
  })
})
