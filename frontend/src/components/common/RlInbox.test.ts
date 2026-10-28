import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import RlInbox from 'rela-components/components/channel/RlInbox.vue'
import { provideMessages } from 'rela-components/composables/useMessages'
import { LOCALE_MESSAGES } from 'rela-components/composables/localeMessages'
import type {
  InboxMentionRow,
  InboxUnreadRow,
  InboxWaitingRow,
} from 'rela-components/components/channel/types'

// RlInbox (TKT-9YRRM5, AC 22). The library has no unit-test runner of its
// own (its vitest project runs the stories in a browser), so this lives in
// the app's suite, like RlMenuPress.test.ts.

const link = (href: string) => ({ attrs: { href } })

const waiting: InboxWaitingRow[] = [
  {
    id: 'w1',
    anchorTitle: 'Outage',
    entityType: 'Incident',
    channel: 'Room',
    state: 'Triage',
    since: '09:00',
    sinceIso: '2026-10-28T09:00:00Z',
    link: link('/entity/incident/INC-1?channel=room%2F'),
  },
]
const mentions: InboxMentionRow[] = [
  {
    id: 'm1',
    kind: 'mention',
    author: 'Bob',
    anchorTitle: 'Fix login',
    channel: 'Case',
    at: '10:04',
    link: link('/entity/ticket/TKT-12?channel=case%2F'),
  },
  {
    id: 'm2',
    kind: 'reply',
    author: 'Carol',
    anchorTitle: 'Fix login',
    channel: 'Case',
    at: '10:05',
    link: link('/entity/ticket/TKT-12?channel=case%2F'),
  },
]
const unread: InboxUnreadRow[] = [
  { id: 'u1', anchorTitle: 'Plan', channel: 'Topic', unread: 4, link: link('/x') },
]

let wrapper: VueWrapper | undefined

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
})

function mountInbox(props: Partial<InstanceType<typeof RlInbox>['$props']> = {}) {
  wrapper = mount(RlInbox, { props: { waiting, mentions, unread, ...props } })
  return wrapper
}

describe('RlInbox', () => {
  it('renders three sections with rows that link to their channel', () => {
    const w = mountInbox()
    expect(w.findAll('h2').map((x) => x.text())).toEqual([
      'Waiting on you',
      'Mentions and replies',
      'Unread channels',
    ])
    const rows = w.findAll('li')
    expect(rows).toHaveLength(4)
    expect(rows[0]!.find('a').attributes('href')).toBe('/entity/incident/INC-1?channel=room%2F')
    expect(rows[0]!.find('time').attributes('datetime')).toBe('2026-10-28T09:00:00Z')
    expect(rows[1]!.text()).toContain('Bob mentioned you')
    expect(rows[2]!.text()).toContain('Carol replied to you')
    expect(rows[3]!.text()).toContain('4 unread')
  })

  it('shows an empty state per section', () => {
    const w = mountInbox({ waiting: [], mentions: [], unread: [] })
    expect(w.text()).toContain('Nothing waits on you.')
    expect(w.text()).toContain('No unread mentions or replies.')
    expect(w.text()).toContain('All channels are read.')
  })

  it('offers mark read on mention and unread rows only, and emits the row id', async () => {
    const w = mountInbox()
    const buttons = w.findAll('button')
    expect(buttons).toHaveLength(3)
    await buttons[2]!.trigger('click')
    expect(w.emitted('markRead')).toEqual([['u1']])
  })

  it('notes a cut-off waiting list and the degraded notice', () => {
    const w = mountInbox({ waitingMore: { shown: 50, total: '20000+' }, degraded: true })
    expect(w.text()).toContain('Showing 50 of 20000+')
    expect(w.text()).toContain('Some channel settings could not be loaded.')
  })

  it('renders titles as text', () => {
    const w = mountInbox({
      waiting: [{ ...waiting[0]!, anchorTitle: '<img src=x onerror="alert(1)">' }],
    })
    expect(w.find('img').exists()).toBe(false)
  })

  it('speaks Dutch through provideMessages', () => {
    const Host = defineComponent({
      setup() {
        provideMessages(LOCALE_MESSAGES.nl)
        return () => h(RlInbox, { waiting: [], mentions, unread: [] })
      },
    })
    wrapper = mount(Host)
    expect(wrapper.text()).toContain('Wacht op jou')
    expect(wrapper.text()).toContain('Bob heeft je genoemd')
    expect(wrapper.text()).toContain('Markeer als gelezen')
  })
})
