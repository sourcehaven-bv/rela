import type {
  Attachment,
  Comment,
  DetailField,
  NavGroup,
  Section,
  StatusOption,
  Swimlane,
  Task,
} from '../types'
import type { AnchoredComment, CommentAnchorOption } from '../components/comment/types'
import type { TableColumn } from '../components/table/types'
import type { CalendarEvent, CalendarSource } from '../components/calendar/types'
import type { CalendarDay } from '../components/calendar/calendarGrid'

export const navGroups: NavGroup[] = [
  {
    id: 'main',
    items: [
      { id: 'search', label: 'Search', icon: 'search' },
      { id: 'notifications', label: 'Notifications', icon: 'bell' },
      { id: 'my-tasks', label: 'My tasks', icon: 'done', opensFlyout: true },
      { id: 'timeline', label: 'Timeline', icon: 'chart-no-axes-gantt' },
    ],
  },
  {
    id: 'initiatives',
    label: 'Initiatives',
    showMenu: true,
    items: [
      { id: 'atlas', label: 'Atlas for Sourcehaven', initial: 'R' },
      { id: 'tender', label: 'Company tender ready', initial: 'R' },
      { id: 'admin', label: 'Administration improvement', initial: 'J' },
      { id: 'crm', label: 'CRM for Sourcehaven', initial: 'J' },
      { id: 'iso', label: 'ISO27001', initial: 'T' },
    ],
  },
]

export const meetingNavGroups: NavGroup[] = [
  ...navGroups,
  {
    id: 'meetings',
    label: 'Meetings',
    showMenu: true,
    items: [
      {
        id: 'mt',
        label: 'MT meeting',
        initial: 'D',
        expanded: true,
        children: [
          { id: 'mt-1', label: '23-09-2026' },
          { id: 'mt-2', label: '30-09-2026' },
          { id: 'mt-3', label: '07-10-2026' },
          { id: 'mt-4', label: '14-10-2026' },
        ],
      },
      { id: 'weekly', label: 'Weekly meeting', initial: 'R' },
      { id: 'iso-status', label: 'ISO27001 Status update', initial: 'R' },
    ],
  },
]

export const boardSections: Section<Task>[] = [
  {
    id: 'backlog',
    title: 'Backlog',
    color: 'grey',
    count: 4,
    items: [
      {
        id: 'b1',
        title: 'Contact customers with failed new payents or who churned',
        tags: [
          { id: 't1', label: 'design', color: 'blue' },
          { id: 't2', label: 'design', color: 'green' },
        ],
        assignee: 'Hanry',
        commentCount: 2,
        subtaskCount: 5,
        dueDate: 'Aug 6',
      },
      {
        id: 'b2',
        title: 'Reporting: Design concept of visual dashboard',
        tags: [
          { id: 't3', label: 'bug', color: 'red' },
          { id: 't4', label: 'API', color: 'amber' },
        ],
        assignee: 'Hanna',
      },
      { id: 'b3', title: 'Task detail modal: ideas' },
      {
        id: 'b4',
        title: '@dev QA: regression ( before/after release)',
        tags: [
          { id: 't5', label: 'bud', color: 'red' },
          { id: 't6', label: 'design', color: 'green' },
        ],
        assignee: 'Hanry',
        dueDate: 'Sep 2',
      },
    ],
  },
  {
    id: 'postpone',
    title: 'Postpone',
    color: 'red',
    count: 6,
    collapsed: true,
    items: [],
  },
  {
    id: 'in-progress',
    title: 'In progress',
    color: 'green',
    count: 5,
    items: [
      {
        id: 'p1',
        title: 'Lead feedback sessions',
        tags: [
          { id: 't7', label: 'design', color: 'blue' },
          { id: 't8', label: 'design', color: 'green' },
        ],
        assignee: 'Alex',
        commentCount: 1,
        dueDate: 'Sep 22',
      },
      { id: 'p2', title: 'Add Projects to templates and layouts [draft 2023]' },
      { id: 'p3', title: 'Extension: show totals', assignee: 'Alice', commentCount: 2 },
      {
        id: 'p4',
        title: 'Help Docs: update screenshot',
        tags: [
          { id: 't9', label: 'plan', color: 'amber' },
          { id: 't10', label: 'bug', color: 'red' },
        ],
        assignee: 'Billy',
      },
      {
        id: 'p5',
        title: 'Help Docs: update screenshot',
        assignee: 'Alice',
        subtaskCount: 2,
        dueDate: 'Aug 6',
      },
    ],
  },
  {
    id: 'qa',
    title: 'QA',
    color: 'amber',
    count: 4,
    items: [
      {
        id: 'q1',
        title: 'Invoices: fixed-fee projects',
        tags: [
          { id: 't11', label: 'design', color: 'blue' },
          { id: 't12', label: 'design', color: 'green' },
        ],
        assignee: 'Adam',
        commentCount: 2,
        subtaskCount: 5,
      },
      {
        id: 'q2',
        title: 'Time: search - not last response with results appears',
        tags: [{ id: 't13', label: 'bug', color: 'red' }],
        assignee: 'Henry',
        subtaskCount: 5,
        dueDate: 'Sep 8',
      },
      { id: 'q3', title: 'Pricing page: new iteration and few mockups and ideas' },
      {
        id: 'q4',
        title: '@dev QA: regression ( before/after release)',
        assignee: 'Alex',
        dueDate: 'Nov 3',
      },
    ],
  },
]

/**
 * The board's own cards, grouped a second time by who holds them.
 *
 * Built from `boardSections` rather than written out again, so the swimlane
 * stories and the board stories always show the same work: a lane board is a
 * regrouping of a board, and a fixture that drifted from it would hide that.
 * A task with no assignee falls in the trailing lane, which is the case the
 * layout has to survive.
 */
const laneOwners: { id: string; title: string; assignee?: string }[] = [
  { id: 'hanry', title: 'Hanry', assignee: 'Hanry' },
  { id: 'alex', title: 'Alex', assignee: 'Alex' },
  { id: 'alice', title: 'Alice', assignee: 'Alice' },
  { id: 'unassigned', title: 'Unassigned' },
]

export const swimlanes: Swimlane<Task>[] = laneOwners.map((owner) => ({
  id: owner.id,
  title: owner.title,
  color: owner.assignee ? 'blue' : 'grey',
  sections: boardSections.map((section) => ({
    ...section,
    count: undefined,
    items: section.items.filter((task) =>
      owner.assignee ? task.assignee === owner.assignee : !task.assignee,
    ),
  })),
}))

const feedbackTask = (id: string, extra: Partial<Task> = {}): Task => ({
  id,
  title: 'Give awesome feedback on UX for Atlas Projects',
  ...extra,
})

export const tableColumns: TableColumn[] = [
  { key: 'assignee', header: 'Assignee', field: 'assignee', width: 128 },
  { key: 'dueDate', header: 'Due date', field: 'dueDate', width: 128 },
  { key: 'label', header: 'Label', field: 'tags', width: 148 },
  { key: 'priority', header: 'Priority', field: 'priority', width: 150 },
]

export const tableSections: Section<Task>[] = [
  {
    id: 'in-progress',
    title: 'In progress',
    color: 'green',
    count: 5,
    items: [
      {
        id: 'i1',
        title: 'Create awesome UX for Atlas Projects',
        assignee: 'Rowdy',
        dueDate: '18 sep, 2026',
        commentCount: 3,
        tags: [
          { id: 'g1', label: 'Design', color: 'amber' },
          { id: 'g2', label: 'Web', color: 'blue' },
        ],
        priority: { id: 'pr1', label: 'High', color: 'green' },
      },
      feedbackTask('i2', { subtaskCount: 5 }),
      feedbackTask('i3', { subtaskCount: 4 }),
      feedbackTask('i4', { commentCount: 3 }),
      feedbackTask('i5'),
    ],
  },
  {
    id: 'on-hold',
    title: 'On Hold',
    color: 'red',
    count: 5,
    items: [
      { id: 'h1', title: 'Create awesome UX for Atlas Projects' },
      feedbackTask('h2'),
      feedbackTask('h3'),
      feedbackTask('h4'),
      feedbackTask('h5'),
    ],
  },
  { id: 'review', title: 'Review', color: 'amber', count: 3, collapsed: true, items: [] },
  { id: 'backlog', title: 'Backlog', color: 'grey', count: 55, collapsed: true, items: [] },
]

/**
 * The statuses a task can be moved between. Shared by the detail fields and
 * the table, so editing a status in either place offers the same list.
 */
export const statusOptions: StatusOption[] = [
  { value: 'Not started', label: 'Not started', status: 'grey' },
  { value: 'In progress', label: 'In progress', status: 'green' },
  { value: 'At risk', label: 'At risk', status: 'amber' },
  { value: 'Blocked', label: 'Blocked', status: 'red' },
  { value: 'Done', label: 'Done', status: 'blue' },
]

export const detailFields: DetailField[] = [
  { id: 'status', label: 'Status', type: 'status', value: 'In progress', status: 'green' },
  { id: 'assignee', label: 'Assignee', type: 'text', value: 'Rowdy' },
  {
    id: 'priority',
    label: 'Priority',
    type: 'tag',
    tags: [{ id: 'p', label: 'High', color: 'green' }],
  },
  { id: 'due', label: 'Due Date', type: 'date', value: '18 sep, 2026' },
  {
    id: 'tags',
    label: 'Tags',
    type: 'tags',
    tags: [
      { id: 'd', label: 'Design', color: 'amber' },
      { id: 'w', label: 'Web', color: 'blue' },
    ],
  },
]

const lorem =
  "Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since 1966, when designers at Letraset and James Mosley, the librarian at St Bride Printing Library in London, took a 1914 Cicero translation and scrambled it to make dummy text for Letraset's Body Type sheets."

export const detailDescription = [
  `${lorem} It has survived not only many decades, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised thanks to these sheets and more recently with desktop publishing software like Aldus PageMaker and Microsoft Word including versions of Lorem Ipsum.`,
  lorem,
]

export const detailAttachments: Attachment[] = [
  { id: 'a1', name: 'Awesome.pdf', kind: 'pdf' },
  { id: 'a2', name: 'Awesome.pdf', kind: 'doc' },
]

export const detailSubtasks: Task[] = [
  feedbackTask('s1', { dueDate: 'Aug 23', assignee: 'Rowdy' }),
  feedbackTask('s2', { dueDate: 'Aug 27', assignee: 'Jeroen' }),
]

export const detailComments: Comment[] = [
  {
    id: 'c1',
    author: 'Rowdy van Looy',
    timestamp: '18 Sep, 00:08',
    body: `${lorem} It has survived not only many decades, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised thanks to these sheets and more recently with desktop publishing software like Aldus PageMaker and Microsoft Word including versions of Lorem Ipsum.`,
  },
  {
    id: 'c2',
    author: 'Jeroen Vloothuis',
    timestamp: '18 Sep, 06:18',
    body: `${lorem} It has survived not only many decades, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised thanks to these sheets and more recently with desktop publishing software like Aldus PageMaker.`,
  },
]

export const anchoredComments: AnchoredComment[] = [
  {
    id: 'ac1',
    author: 'Rowdy van Looy',
    timestamp: '18 Sep, 09:12',
    body: 'Is this the launch date we agreed, or the one from the old plan?',
    anchor: { kind: 'property', ref: 'dueDate', label: 'Due date' },
    editable: true,
    deletable: true,
  },
  {
    id: 'ac2',
    author: 'Jeroen Vloothuis',
    timestamp: '18 Sep, 10:40',
    body: 'The old one. I have moved it to the 24th.',
    anchor: { kind: 'property', ref: 'dueDate', label: 'Due date' },
    resolved: true,
    editable: true,
    deletable: true,
  },
  {
    id: 'ac3',
    author: 'Tess de Vries',
    timestamp: '19 Sep, 08:05',
    body: 'This section still describes the pilot rather than the rollout.',
    anchor: { kind: 'section', ref: 'scope', label: 'Section: Scope' },
    editable: true,
    deletable: true,
  },
  {
    id: 'ac4',
    author: 'Rowdy van Looy',
    timestamp: '19 Sep, 11:30',
    body: 'Worth keeping for the record, even though the field is gone.',
    anchor: { kind: 'property', ref: 'legacyOwner', label: 'Legacy owner' },
    detached: true,
    deletable: true,
  },
]

export const commentAnchorOptions: CommentAnchorOption[] = [
  { key: 'property:title', label: 'Title', anchor: { kind: 'property', ref: 'title', label: 'Title' } },
  {
    key: 'property:dueDate',
    label: 'Due date',
    anchor: { kind: 'property', ref: 'dueDate', label: 'Due date' },
  },
  {
    key: 'property:owner',
    label: 'Owner',
    anchor: { kind: 'property', ref: 'owner', label: 'Owner' },
  },
  {
    key: 'section:scope',
    label: 'Section: Scope',
    anchor: { kind: 'section', ref: 'scope', label: 'Section: Scope' },
  },
]

/**
 * A September 2026 calendar: two sources, a couple of multi-day events and one
 * day loaded enough to hit a per-day cap.
 */
export const calendarAnchor: CalendarDay = { year: 2026, month: 9, day: 15 }
export const calendarToday: CalendarDay = { year: 2026, month: 9, day: 22 }

export const calendarSources: CalendarSource[] = [
  { id: 'tasks', label: 'Tasks', color: 'blue' },
  { id: 'meetings', label: 'Meetings', color: 'purple' },
  { id: 'releases', label: 'Releases', color: 'green' },
]

const day = (d: number): CalendarDay => ({ year: 2026, month: 9, day: d })

export const calendarEvents: CalendarEvent[] = [
  {
    id: 'e1',
    summary: 'Atlas design review',
    startDay: day(15),
    endDay: day(15),
    timeLabel: '09:30',
    color: 'purple',
    fields: [{ label: 'Owner', value: 'Jeroen' }],
    draggable: true,
  },
  {
    id: 'e2',
    summary: 'ISO27001 audit week',
    startDay: day(14),
    endDay: day(18),
    color: 'amber',
    fields: [{ label: 'Auditor', value: 'T. de Vries' }],
  },
  {
    id: 'e3',
    summary: 'Release 2.4',
    startDay: day(22),
    endDay: day(22),
    timeLabel: '16:00',
    color: 'green',
    draggable: true,
  },
  {
    id: 'e4',
    summary: 'Write the release notes',
    startDay: day(22),
    endDay: day(22),
    color: 'blue',
    fields: [{ label: 'Status', value: 'In progress' }],
    draggable: true,
  },
  {
    id: 'e5',
    summary: 'Weekly meeting',
    startDay: day(22),
    endDay: day(22),
    timeLabel: '11:00',
    color: 'purple',
  },
  {
    id: 'e6',
    summary: 'Tender deadline',
    startDay: day(22),
    endDay: day(22),
    timeLabel: '17:00',
    color: 'red',
  },
  {
    id: 'e7',
    summary: 'Sprint planning for the Atlas migration',
    startDay: day(28),
    endDay: day(30),
    timeLabel: '10:00',
    color: 'blue',
    draggable: true,
  },
  {
    id: 'e8',
    summary: 'CRM discovery call',
    startDay: day(3),
    endDay: day(3),
    timeLabel: '14:00',
    color: 'purple',
  },
]

/*
 * The "My tasks" slide-out list, grouped by when each task is due, which is
 * what that view is for. Shared so every screen offering the nav item shows
 * the same list. See `RlMyTasksPanels`.
 */
export const myTaskGroups = [
  {
    title: 'Today',
    rows: [
      { id: 't1', label: 'Review the Q4 initiative brief', meta: '07:00' },
      { id: 't2', label: 'Atlas mobile redesign: sign off copy', meta: '09:00' },
      { id: 't3', label: 'Invoices: fixed-fee projects', meta: '12:00' },
      { id: 't4', label: 'Pricing page: new iteration', meta: '13:00' },
    ],
  },
  {
    title: 'Tomorrow',
    rows: [
      { id: 't5', label: 'Docs: update the board screenshot', meta: '08:00' },
      { id: 't6', label: 'Payment provider migration: dry run', meta: '17:00' },
      { id: 't7', label: 'Contact churned customers', meta: '18:00' },
    ],
  },
  {
    title: 'Next 7 days',
    rows: [
      { id: 't8', label: 'Migrate projects to templates', meta: 'Mon' },
      { id: 't9', label: 'QA regression before release', meta: 'Mon' },
      { id: 't10', label: 'Team feedback sessions', meta: 'Tue' },
      { id: 't11', label: 'Design token audit', meta: 'Wed' },
    ],
  },
]
