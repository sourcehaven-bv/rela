/**
 * Large datasets for the at-scale example screens.
 *
 * The regular fixtures show a screen at a comfortable size. These show the
 * same screens once a real workspace has filled them: hundreds of tasks,
 * sections that scroll past the fold, titles that do not fit their column,
 * and comment threads long enough to need their own scrollbar.
 *
 * Every value is generated from a fixed table, so the data is stable between
 * reloads and a story always renders the same screen.
 */
import type { Attachment, Comment, DetailField, NavGroup, Section, Tag, Task } from '../types'
import type { TableColumn } from '../components/table/types'

/**
 * Deterministic stand-in for a random number generator.
 *
 * The hash matters: indexing a list by a seed that steps in even increments
 * lands on the same entry every time whenever the list length shares a factor
 * with the step, and the screen fills with one repeated value.
 */
function pick<T>(list: readonly T[], seed: number): T {
  let h = seed * 2654435761
  h ^= h >>> 15
  h = Math.imul(h, 2246822507)
  h ^= h >>> 13
  return list[Math.abs(h) % list.length]
}

const verbs = [
  'Rewrite',
  'Audit',
  'Migrate',
  'Instrument',
  'Decommission',
  'Benchmark',
  'Document',
  'Roll out',
  'Investigate',
  'Consolidate',
  'Harden',
  'Translate',
]

const objects = [
  'the invoice export',
  'the onboarding checklist',
  'the permission model',
  'the nightly reconciliation job',
  'the customer import',
  'the notification digest',
  'the search index',
  'the audit trail',
  'the billing webhooks',
  'the seat-count report',
  'the retention policy',
  'the workspace switcher',
]

const qualifiers = [
  'before the ISO27001 evidence freeze',
  'so support can answer without opening a ticket',
  'now that the pilot tenants are live',
  'ahead of the 2.4 release train',
  'and fold the findings back into the runbook',
  'without breaking the public API contract',
  'for the tenants still on the legacy schema',
  'including the Dutch and German copy',
]

const people = [
  'Rowdy',
  'Hanna',
  'Jeroen',
  'Alex',
  'Tess',
  'Billy',
  'Adam',
  'Henry',
  'Alice',
  'Mirjam',
  'Youssef',
  'Kirsten',
]

const tagPool: Tag[] = [
  { id: 'tg-design', label: 'Design', color: 'amber' },
  { id: 'tg-web', label: 'Web', color: 'blue' },
  { id: 'tg-bug', label: 'Bug', color: 'red' },
  { id: 'tg-api', label: 'API', color: 'green' },
  { id: 'tg-research', label: 'Research', color: 'purple' },
  { id: 'tg-infra', label: 'Infrastructure', color: 'grey' },
  { id: 'tg-compliance', label: 'Compliance and audit', color: 'red' },
  { id: 'tg-i18n', label: 'Internationalisation', color: 'purple' },
]

const priorities: Tag[] = [
  { id: 'pr-low', label: 'Low', color: 'grey' },
  { id: 'pr-med', label: 'Medium', color: 'blue' },
  { id: 'pr-high', label: 'High', color: 'green' },
  { id: 'pr-urgent', label: 'Urgent, blocks the release', color: 'red' },
]

const dueDates = [
  '3 sep, 2026',
  '11 sep, 2026',
  '18 sep, 2026',
  '24 sep, 2026',
  '2 oct, 2026',
  '15 oct, 2026',
  '1 nov, 2026',
]

/**
 * One task, built from the tables above. Every third task gets a long title,
 * and tag counts vary from one to three, so each screen has a mix of
 * comfortable and overflowing rows to judge rather than a uniform block.
 */
function bulkTask(prefix: string, index: number): Task {
  const seed = index * 31 + prefix.length * 101
  const long = index % 3 === 0
  const title = long
    ? `${pick(verbs, seed)} ${pick(objects, seed + 1)} ${pick(qualifiers, seed + 2)}`
    : `${pick(verbs, seed)} ${pick(objects, seed + 1)}`

  // One, two or three tags, so the label column has a mix to lay out rather
  // than one uniform width.
  const tagCount = index % 7 === 0 ? 3 : index % 3 === 0 ? 2 : 1
  const tags = Array.from({ length: tagCount }, (_, t) => pick(tagPool, seed + t * 17))

  return {
    id: `${prefix}-${index}`,
    title,
    assignee: pick(people, seed + 3),
    dueDate: index % 4 === 0 ? undefined : pick(dueDates, seed + 4),
    tags,
    priority: pick(priorities, seed + 6),
    commentCount: index % 5 === 0 ? (index % 23) + 1 : undefined,
    subtaskCount: index % 6 === 0 ? (index % 11) + 1 : undefined,
  }
}

function bulkTasks(prefix: string, count: number): Task[] {
  return Array.from({ length: count }, (_, i) => bulkTask(prefix, i + 1))
}

const sectionSpec = [
  { id: 'triage', title: 'Triage', color: 'grey', size: 34 },
  { id: 'in-progress', title: 'In progress', color: 'green', size: 28 },
  { id: 'blocked', title: 'Blocked on another team', color: 'red', size: 17 },
  { id: 'review', title: 'In review', color: 'amber', size: 21 },
  { id: 'qa', title: 'QA and regression', color: 'amber', size: 19 },
  { id: 'ready', title: 'Ready to ship', color: 'blue', size: 12 },
] as const

/** Six open sections, 131 tasks in total. */
export const bulkTableSections: Section<Task>[] = [
  ...sectionSpec.map(({ id, title, color, size }) => ({
    id,
    title,
    color,
    count: size,
    items: bulkTasks(id, size),
  })),
  { id: 'done', title: 'Done this quarter', color: 'blue', count: 214, collapsed: true, items: [] },
  { id: 'archive', title: 'Archive', color: 'grey', count: 1382, collapsed: true, items: [] },
]

/**
 * The same tasks as columns. One column stays collapsed, because a board
 * with every column open is not what a loaded board looks like.
 */
export const bulkBoardSections: Section<Task>[] = bulkTableSections.map((section) =>
  section.id === 'archive' ? { ...section, collapsed: true } : section,
)

export const bulkTableColumns: TableColumn[] = [
  { key: 'assignee', header: 'Assignee', field: 'assignee', width: 128 },
  { key: 'dueDate', header: 'Due date', field: 'dueDate', width: 128 },
  { key: 'label', header: 'Label', field: 'tags', width: 148 },
  { key: 'priority', header: 'Priority', field: 'priority', width: 150 },
]

/** A sidebar with more initiatives than fit, so it scrolls on its own. */
export const bulkNavGroups: NavGroup[] = [
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
      { id: 'billing', label: 'Billing and invoicing overhaul 2026', initial: 'H' },
      { id: 'i18n', label: 'Dutch and German localisation', initial: 'M' },
      { id: 'mobile', label: 'Mobile companion app', initial: 'A' },
      { id: 'legacy', label: 'Legacy schema migration', initial: 'Y' },
      { id: 'support', label: 'Support tooling', initial: 'K' },
      { id: 'analytics', label: 'Product analytics and reporting', initial: 'T' },
      { id: 'sso', label: 'Single sign-on for enterprise tenants', initial: 'B' },
      { id: 'perf', label: 'Performance budget', initial: 'A' },
      { id: 'design-system', label: 'Design system consolidation', initial: 'H' },
    ],
  },
]

/**
 * A sidebar past the point where scanning works, so the filter appears.
 *
 * The names are deliberately close together — several begin "Billing", three
 * name ISO27001 — because that is what a real workspace looks like after a
 * few years, and it is what makes a list this long hard to read rather than
 * merely long.
 */
export const crowdedNavGroups: NavGroup[] = [
  bulkNavGroups[0],
  {
    id: 'initiatives',
    label: 'Initiatives',
    showMenu: true,
    subgroups: [
      { id: 'active', label: 'Active' },
      { id: 'planned', label: 'Planned' },
      { id: 'paused', label: 'Paused' },
    ],
    items: [
      { id: 'atlas', label: 'Atlas for Sourcehaven', initial: 'R', groupId: 'active' },
      { id: 'atlas-2', label: 'Atlas phase 2', initial: 'R', groupId: 'planned' },
      { id: 'atlas-mobile', label: 'Atlas mobile companion', initial: 'A', groupId: 'planned' },
      { id: 'billing', label: 'Billing and invoicing overhaul', initial: 'H', groupId: 'active' },
      { id: 'billing-2026', label: 'Billing and invoicing overhaul 2026', initial: 'H', groupId: 'planned' },
      { id: 'billing-webhooks', label: 'Billing webhooks', initial: 'J', groupId: 'active' },
      { id: 'crm', label: 'CRM for Sourcehaven', initial: 'J', groupId: 'active' },
      { id: 'crm-import', label: 'CRM customer import', initial: 'J', groupId: 'paused' },
      { id: 'design-system', label: 'Design system consolidation', initial: 'H', groupId: 'active' },
      { id: 'design-tokens', label: 'Design tokens and theming', initial: 'H', groupId: 'planned' },
      { id: 'i18n', label: 'Dutch and German localisation', initial: 'M', groupId: 'active' },
      { id: 'i18n-copy', label: 'Dutch copy review', initial: 'M', groupId: 'active' },
      { id: 'iso', label: 'ISO27001', initial: 'T', groupId: 'active' },
      { id: 'iso-evidence', label: 'ISO27001 evidence collection', initial: 'T', groupId: 'active' },
      { id: 'iso-status', label: 'ISO27001 status reporting', initial: 'T', groupId: 'planned' },
      { id: 'legacy', label: 'Legacy schema migration', initial: 'Y', groupId: 'active' },
      { id: 'legacy-rollback', label: 'Legacy rollback runbook', initial: 'Y', groupId: 'paused' },
      { id: 'notifications', label: 'Notification digest', initial: 'K', groupId: 'paused' },
      { id: 'onboarding', label: 'Onboarding checklist', initial: 'A', groupId: 'planned' },
      { id: 'perf', label: 'Performance budget', initial: 'A', groupId: 'active' },
      { id: 'perf-traces', label: 'Performance tracing', initial: 'A', groupId: 'planned' },
      { id: 'analytics', label: 'Product analytics and reporting', initial: 'T', groupId: 'planned' },
      { id: 'retention', label: 'Retention policy', initial: 'K', groupId: 'paused' },
      { id: 'search', label: 'Search index rebuild', initial: 'B', groupId: 'active' },
      { id: 'seat-count', label: 'Seat-count reporting', initial: 'B', groupId: 'active' },
      { id: 'sso', label: 'Single sign-on for enterprise tenants', initial: 'B', groupId: 'planned' },
      { id: 'support', label: 'Support tooling', initial: 'K', groupId: 'active' },
      { id: 'tender', label: 'Company tender ready', initial: 'R', groupId: 'active' },
      { id: 'admin', label: 'Administration improvement', initial: 'J', groupId: 'paused' },
      { id: 'audit', label: 'Audit trail', initial: 'T' },
    ],
  },
]

const paragraphs = [
  'The reconciliation job has been running for three quarters without anyone reading its output, which is how the duplicate invoices reached the tenants in the first place. Before we change the query, we need a record of what it currently produces so the migration can be checked against something.',
  'Two tenants are on the legacy schema and cannot be migrated until their custom fields are mapped. That mapping is not documented anywhere except in the head of the person who built it, so the first task here is an interview, not a code change.',
  'The audit asked for evidence that access is reviewed quarterly. We have the reviews, but they live in a spreadsheet that nobody signs. Moving them into the product gives us the trail and removes a manual step at the same time.',
  'Performance is acceptable at the sizes we test with and unacceptable at the sizes our largest tenant actually has. The gap is the whole problem: every screen in this initiative needs a fixture that matches the loaded case, not the comfortable one.',
  'Copy for the Dutch and German locales has been drafted but not reviewed by a native speaker. The review is cheap and the cost of shipping the draft is a support queue we then have to read, so it goes first.',
  'We agreed to keep the public API contract stable through the migration. That constrains the schema change more than the schema change constrains us, and it is worth writing down why before someone reopens the argument next quarter.',
]

/** A description long enough that the detail pane has to scroll. */
export const bulkDescription: string[] = [
  ...paragraphs,
  ...paragraphs.map((p) => p.replace('The ', 'By the time we looked, the ')),
]

export const bulkDetailFields: DetailField[] = [
  { id: 'status', label: 'Status', type: 'status', value: 'In progress', status: 'green' },
  { id: 'assignee', label: 'Assignee', type: 'text', value: 'Rowdy van Looy' },
  { id: 'reporter', label: 'Reporter', type: 'text', value: 'Jeroen Vloothuis' },
  { id: 'reviewer', label: 'Reviewer', type: 'text', value: 'Tess de Vries' },
  {
    id: 'priority',
    label: 'Priority',
    type: 'tag',
    tags: [{ id: 'p', label: 'Urgent, blocks the release', color: 'red' }],
  },
  { id: 'due', label: 'Due Date', type: 'date', value: '18 sep, 2026' },
  { id: 'start', label: 'Start date', type: 'date', value: '2 sep, 2026' },
  { id: 'estimate', label: 'Estimate', type: 'text', value: '13 points' },
  { id: 'spent', label: 'Time spent', type: 'text', value: '21 hours across 9 sessions' },
  { id: 'initiative', label: 'Initiative', type: 'text', value: 'Legacy schema migration' },
  { id: 'team', label: 'Team', type: 'text', value: 'Platform' },
  { id: 'tags', label: 'Tags', type: 'tags', tags: tagPool },
]

const attachmentNames: Array<[string, Attachment['kind']]> = [
  ['Migration plan v7.pdf', 'pdf'],
  ['Schema mapping (legacy to current).sheet', 'sheet'],
  ['Audit evidence 2026-Q3.pdf', 'pdf'],
  ['Interview notes, support lead.doc', 'doc'],
  ['Before and after, invoice export.image', 'image'],
  ['Tenant list with custom fields.sheet', 'sheet'],
  ['Rollback runbook.doc', 'doc'],
  ['Performance trace, largest tenant.other', 'other'],
  ['Copy review, Dutch.doc', 'doc'],
  ['Copy review, German.doc', 'doc'],
  ['API contract diff.other', 'other'],
  ['Kickoff recording transcript.doc', 'doc'],
]

export const bulkAttachments: Attachment[] = attachmentNames.map(([name, kind], i) => ({
  id: `at-${i + 1}`,
  name,
  kind,
}))

/** Twenty-four subtasks, which is where a flat subtask list starts to hurt. */
export const bulkSubtasks: Task[] = Array.from({ length: 24 }, (_, i) => {
  const task = bulkTask('sub', i + 1)
  return { ...task, tags: undefined, priority: undefined }
})

const commentBodies = [
  'I read the export for last month and the duplicates all share a tenant id, so this is one code path rather than a class of problem. That is better news than the ticket suggests.',
  'Agreed on the interview first. I have asked for an hour on Thursday and will put the mapping straight into this task rather than a separate doc, so it stays next to the work.',
  'Careful with the rollback runbook. It was written against the pre-2.2 deploy process and the step that drains the queue no longer exists. I will mark the stale steps rather than delete them, so we keep the reasoning.',
  'The performance trace is from the largest tenant, taken at 09:00 on a Monday, which is the worst case we have measured. If we are inside budget there we are inside budget everywhere.',
  'Dutch copy is reviewed. German is still with the reviewer; they flagged two strings that do not have a natural translation and suggested we change the source English instead. I think they are right.',
  'This is the third time the public API contract has come up. Writing the constraint down and linking it from the initiative would save us the next round of this conversation.',
  'Moved the due date to the 24th to line up with the release train. Nothing else changed, and the estimate still looks right to me.',
  'Blocked from my side until the schema mapping lands, so I have picked up the audit evidence task in the meantime. Ping me here and I will switch back.',
]

/** A thread long enough that the panel scrolls and the composer stays put. */
export const bulkComments: Comment[] = Array.from({ length: 28 }, (_, i) => ({
  id: `bc-${i + 1}`,
  author: [
    'Rowdy van Looy',
    'Jeroen Vloothuis',
    'Tess de Vries',
    'Hanna Berg',
    'Youssef El Amrani',
  ][i % 5],
  timestamp: `${(i % 28) + 1} Sep, ${String(6 + (i % 12)).padStart(2, '0')}:${String((i * 7) % 60).padStart(2, '0')}`,
  body: commentBodies[i % commentBodies.length],
}))

/** The task the detail views open, long enough to wrap in every header. */
export const bulkDetailTitle =
  'Migrate the legacy invoice schema for the two remaining pilot tenants, keeping the public API contract stable'
