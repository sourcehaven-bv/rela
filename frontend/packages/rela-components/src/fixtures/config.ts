/**
 * Sample configuration for the "Configure" mockups, taken from rela's own
 * ticket tracker (`tickets/schema.yaml` and `tickets/data-entry.yaml`).
 *
 * Everything is named the way the screens name it, not the way the files do:
 * an enum type is a "choice list", a validation is a "rule". The files are an
 * implementation detail these screens never show.
 */
import type { NavGroup, TagColor } from '../types'

// --- Navigation of the Configure space ---

export const configNavGroups: NavGroup[] = [
  {
    id: 'model',
    label: 'Data model',
    items: [
      { id: 'entity-types', label: 'Entity types', icon: 'boxes' },
      { id: 'choice-lists', label: 'Choice lists', icon: 'list' },
      { id: 'relations', label: 'Relations', icon: 'network' },
      { id: 'rules', label: 'Rules', icon: 'shield-check' },
      { id: 'automations', label: 'Automations', icon: 'zap' },
    ],
  },
  {
    id: 'screens',
    label: 'Screens',
    items: [
      { id: 'navigation', label: 'Navigation', icon: 'menu' },
      { id: 'forms', label: 'Forms', icon: 'document' },
      { id: 'lists', label: 'Lists', icon: 'table' },
      { id: 'boards', label: 'Boards', icon: 'kanban' },
      { id: 'dashboard', label: 'Dashboard', icon: 'dashboard' },
    ],
  },
  {
    id: 'history',
    label: 'History',
    items: [{ id: 'migrations', label: 'Migrations', icon: 'history' }],
  },
]

// --- Entity types ---

export interface EntityTypeRow {
  id: string
  title: string
  idPrefix?: string
  idStyle: 'Short code' | 'Chosen by hand'
  properties: number
  relations: number
  color: TagColor
  area: string
}

export const entityTypes: EntityTypeRow[] = [
  { id: 'ticket', title: 'Ticket', idPrefix: 'TKT-', idStyle: 'Short code', properties: 8, relations: 9, color: 'blue', area: 'Delivery' },
  { id: 'bug', title: 'Bug', idPrefix: 'BUG-', idStyle: 'Short code', properties: 13, relations: 7, color: 'red', area: 'Delivery' },
  { id: 'feature', title: 'Feature', idPrefix: 'FEAT-', idStyle: 'Short code', properties: 5, relations: 6, color: 'green', area: 'Delivery' },
  { id: 'review-response', title: 'Review response', idPrefix: 'RR-', idStyle: 'Short code', properties: 6, relations: 1, color: 'amber', area: 'Delivery' },
  { id: 'concept', title: 'Concept', idStyle: 'Chosen by hand', properties: 6, relations: 8, color: 'blue', area: 'Architecture' },
  { id: 'decision', title: 'Decision', idPrefix: 'DEC-', idStyle: 'Short code', properties: 5, relations: 3, color: 'purple', area: 'Architecture' },
  { id: 'research', title: 'Research', idPrefix: 'RES-', idStyle: 'Short code', properties: 3, relations: 2, color: 'purple', area: 'Architecture' },
  { id: 'risk', title: 'Risk', idPrefix: 'RISK-', idStyle: 'Short code', properties: 6, relations: 3, color: 'red', area: 'Quality' },
  { id: 'test-case', title: 'Test case', idPrefix: 'TC-', idStyle: 'Short code', properties: 6, relations: 2, color: 'green', area: 'Quality' },
  { id: 'idea', title: 'Idea', idPrefix: 'IDEA-', idStyle: 'Short code', properties: 8, relations: 4, color: 'purple', area: 'Ideation' },
  { id: 'guide', title: 'Guide', idStyle: 'Chosen by hand', properties: 6, relations: 2, color: 'grey', area: 'Docs' },
  { id: 'planning-checklist', title: 'Planning checklist', idPrefix: 'PLAN-', idStyle: 'Short code', properties: 4, relations: 1, color: 'grey', area: 'Workflow' },
]

// --- Properties of one entity type (Ticket) ---

export interface PropertyRow {
  id: string
  title: string
  name: string
  type: string
  required?: boolean
  many?: boolean
  description?: string
  setBy?: string
}

export const ticketProperties: PropertyRow[] = [
  { id: 'title', title: 'Title', name: 'title', type: 'Text', required: true },
  { id: 'kind', title: 'Kind', name: 'kind', type: 'Ticket kind', required: true },
  { id: 'status', title: 'Status', name: 'status', type: 'Ticket status', required: true },
  { id: 'priority', title: 'Priority', name: 'priority', type: 'Priority' },
  { id: 'effort', title: 'Effort', name: 'effort', type: 'Effort' },
  { id: 'tags', title: 'Tags', name: 'tags', type: 'Ticket tag', many: true },
  { id: 'started', title: 'Started', name: 'started', type: 'Date', description: 'Set by automation when work starts', setBy: 'Automation' },
  { id: 'completed', title: 'Completed', name: 'completed', type: 'Date', description: 'Set by automation when status becomes done', setBy: 'Automation' },
]

/** What a property's type can be: the built-in kinds, then every choice list. */
export const propertyTypeOptions = [
  { value: 'Text', label: 'Text' },
  { value: 'Long text', label: 'Long text' },
  { value: 'Number', label: 'Number' },
  { value: 'Date', label: 'Date' },
  { value: 'Yes / no', label: 'Yes / no' },
  { value: 'Person', label: 'Person' },
  { value: 'File', label: 'File' },
  { value: 'Ticket status', label: 'Choice list: Ticket status' },
  { value: 'Ticket kind', label: 'Choice list: Ticket kind' },
  { value: 'Priority', label: 'Choice list: Priority' },
  { value: 'Effort', label: 'Choice list: Effort' },
  { value: 'Ticket tag', label: 'Choice list: Ticket tag' },
]

// --- Relations ---

export interface RelationRow {
  id: string
  title: string
  name: string
  from: string[]
  to: string[]
  inverse: string
  atLeast?: number
  description: string
}

export const relations: RelationRow[] = [
  { id: 'implements', title: 'implements', name: 'implements', from: ['Ticket'], to: ['Feature'], inverse: 'implemented by', atLeast: 1, description: 'Ticket delivers this feature' },
  { id: 'affects', title: 'affects', name: 'affects', from: ['Ticket', 'Bug', 'Doc task'], to: ['Concept'], inverse: 'affected by', atLeast: 1, description: 'Work touches this architectural concept' },
  { id: 'depends-on', title: 'depends on', name: 'depends-on', from: ['Ticket', 'Bug', 'Feature', 'Doc task'], to: ['Ticket', 'Bug', 'Feature', 'Doc task'], inverse: 'blocks', description: 'Requires this to be completed first' },
  { id: 'fixes', title: 'fixes', name: 'fixes', from: ['Bug'], to: ['Feature'], inverse: 'fixed by', atLeast: 1, description: 'Bug fixes an issue in this feature' },
  { id: 'has-planning', title: 'has planning', name: 'has-planning', from: ['Ticket'], to: ['Planning checklist'], inverse: 'planning for', description: 'The planning checklist of this ticket' },
  { id: 'has-review', title: 'has review', name: 'has-review', from: ['Ticket', 'Bug'], to: ['Review checklist'], inverse: 'review for', description: 'The review checklist of this work item' },
]

// --- Choice lists ---

export interface ChoiceValueRow {
  id: string
  title: string
  color: TagColor
  isDefault?: boolean
  inUse: number
}

export interface ChoiceListRow {
  id: string
  title: string
  values: number
  usedBy: string[]
}

export const choiceLists: ChoiceListRow[] = [
  { id: 'ticket-status', title: 'Ticket status', values: 8, usedBy: ['Ticket'] },
  { id: 'bug-status', title: 'Bug status', values: 8, usedBy: ['Bug'] },
  { id: 'priority', title: 'Priority', values: 4, usedBy: ['Ticket', 'Bug', 'Risk'] },
  { id: 'effort', title: 'Effort', values: 5, usedBy: ['Ticket', 'Bug'] },
  { id: 'ticket-kind', title: 'Ticket kind', values: 5, usedBy: ['Ticket'] },
  { id: 'ticket-tag', title: 'Ticket tag', values: 7, usedBy: ['Ticket'] },
]

export const ticketStatusValues: ChoiceValueRow[] = [
  { id: 'backlog', title: 'Backlog', color: 'grey', isDefault: true, inUse: 312 },
  { id: 'ready', title: 'Ready', color: 'blue', inUse: 41 },
  { id: 'planning', title: 'Planning', color: 'blue', inUse: 6 },
  { id: 'in-progress', title: 'In progress', color: 'purple', inUse: 9 },
  { id: 'review', title: 'Review', color: 'amber', inUse: 4 },
  { id: 'done', title: 'Done', color: 'green', inUse: 1187 },
  { id: 'wont-fix', title: "Won't fix", color: 'grey', inUse: 58 },
  { id: 'blocked', title: 'Blocked', color: 'red', inUse: 3 },
]

export const colorOptions: TagColor[] = ['grey', 'blue', 'green', 'amber', 'red', 'purple']

export const colorLabels: Record<TagColor, string> = {
  grey: 'Grey',
  blue: 'Blue',
  green: 'Green',
  amber: 'Amber',
  red: 'Red',
  purple: 'Purple',
}

// --- Rules and automations ---

export interface RuleRow {
  id: string
  title: string
  appliesTo: string
  when: string
  then: string
  severity: 'Error' | 'Warning'
}

export const rules: RuleRow[] = [
  { id: 'ready-tickets-need-effort', title: 'Ready tickets need an effort', appliesTo: 'Ticket', when: "entity.status == 'ready'", then: 'entity.effort != nil', severity: 'Error' },
  { id: 'ready-tickets-need-priority', title: 'Ready tickets need a priority', appliesTo: 'Ticket', when: "entity.status == 'ready'", then: 'entity.priority != nil', severity: 'Error' },
  { id: 'done-bugs-need-5-whys', title: 'Done bugs need a 5-whys analysis', appliesTo: 'Bug', when: "entity.status == 'done'", then: 'entity.why1 != nil and entity.why2 != nil and entity.why3 != nil', severity: 'Error' },
  { id: 'done-research-needs-summary', title: 'Done research needs a summary', appliesTo: 'Research', when: "entity.status == 'done'", then: "entity.summary != ''", severity: 'Error' },
  { id: 'accepted-decisions-need-date', title: 'Accepted decisions need a date', appliesTo: 'Decision', when: "entity.status == 'accepted'", then: 'entity.date != nil', severity: 'Warning' },
]

export interface AutomationRow {
  id: string
  title: string
  trigger: string
  actions: string
}

export const automations: AutomationRow[] = [
  { id: 'ticket-planning-checklist', title: 'Create a planning checklist', trigger: 'Ticket status becomes Planning', actions: 'Create Planning checklist, linked by has planning' },
  { id: 'ticket-implementation-checklist', title: 'Create an implementation checklist', trigger: 'Ticket status becomes In progress', actions: 'Create Implementation checklist, linked by has implementation' },
  { id: 'ticket-review-checklist', title: 'Create a review checklist', trigger: 'Ticket status becomes Review', actions: 'Create Review checklist, linked by has review' },
  { id: 'ticket-started', title: 'Record when work starts', trigger: 'Ticket status becomes In progress', actions: 'Set Started to today' },
]

// --- Screens: forms, lists, boards, navigation ---

export interface FormFieldRow {
  id: string
  title: string
  type: string
  help?: string
  placeholder?: string
  hidden?: boolean
  width: 'Full' | 'Half' | 'Third'
}

export const editTicketFields: FormFieldRow[] = [
  { id: 'title', title: 'Title', type: 'Text', width: 'Full', placeholder: 'What needs doing?' },
  { id: 'kind', title: 'Kind', type: 'Ticket kind', width: 'Third' },
  { id: 'priority', title: 'Priority', type: 'Priority', width: 'Third' },
  { id: 'effort', title: 'Effort', type: 'Effort', width: 'Third' },
  { id: 'tags', title: 'Tags', type: 'Ticket tag', width: 'Full' },
  { id: 'status', title: 'Status', type: 'Ticket status', width: 'Half' },
]

export interface FormRelationRow {
  id: string
  title: string
  relation: string
  picker: 'Single' | 'Multiple'
  createInline?: string
}

export const editTicketRelations: FormRelationRow[] = [
  { id: 'implements', title: 'Implements feature', relation: 'implements', picker: 'Single' },
  { id: 'affects', title: 'Affects concepts', relation: 'affects', picker: 'Multiple' },
  { id: 'depends-on', title: 'Depends on', relation: 'depends on', picker: 'Multiple' },
  { id: 'has-planning', title: 'Planning checklist', relation: 'has planning', picker: 'Single', createInline: 'Create planning checklist' },
  { id: 'has-review', title: 'Review checklist', relation: 'has review', picker: 'Single', createInline: 'Create review checklist' },
]

export interface ScreenRow {
  id: string
  title: string
  entityType: string
  usedIn: string
}

export const forms: ScreenRow[] = [
  { id: 'create_ticket', title: 'New ticket', entityType: 'Ticket', usedIn: 'Delivery › All tickets' },
  { id: 'edit_ticket', title: 'Edit ticket', entityType: 'Ticket', usedIn: 'Delivery › All tickets, Ticket board' },
  { id: 'create_bug', title: 'Report bug', entityType: 'Bug', usedIn: 'Delivery › Bugs' },
  { id: 'edit_bug', title: 'Edit bug', entityType: 'Bug', usedIn: 'Delivery › Bugs, Bug board' },
  { id: 'create_idea', title: 'Capture idea', entityType: 'Idea', usedIn: 'Ideas › All ideas' },
  { id: 'edit_idea', title: 'Edit idea', entityType: 'Idea', usedIn: 'Ideas › All ideas, Idea pipeline' },
]

export interface ListColumnRow {
  id: string
  title: string
  sortable: boolean
  opensDetail?: boolean
}

export const allTicketsColumns: ListColumnRow[] = [
  { id: 'title', title: 'Title', sortable: true, opensDetail: true },
  { id: 'status', title: 'Status', sortable: true },
  { id: 'priority', title: 'Priority', sortable: true },
  { id: 'effort', title: 'Effort', sortable: true },
  { id: 'kind', title: 'Kind', sortable: false },
]

export const sampleTickets = [
  { id: 'TKT-8UCV32', title: 'Data classification overlay', status: 'Done', priority: 'High', effort: 'L', kind: 'Enhancement' },
  { id: 'TKT-1U8XYN', title: 'Content-free collection reads', status: 'In progress', priority: 'High', effort: 'XL', kind: 'Refactor' },
  { id: 'TKT-PEKL8L', title: 'Entity rows in the sidebar', status: 'Review', priority: 'Medium', effort: 'M', kind: 'Enhancement' },
  { id: 'TKT-UFV01M', title: 'Drop PR URL from the review checklist', status: 'Ready', priority: 'Low', effort: 'XS', kind: 'Chore' },
]

export const boardColumns: ChoiceValueRow[] = ticketStatusValues.filter((value) =>
  ['backlog', 'ready', 'planning', 'in-progress', 'review', 'done'].includes(value.id),
)

export interface NavEntryRow {
  id: string
  title: string
  kind: 'Dashboard' | 'List' | 'Board' | 'Entities' | 'Calendar'
  target: string
  badges?: string
}

export interface NavGroupDraft {
  id: string
  title: string
  entries: NavEntryRow[]
}

export const deliveryNavigation: NavGroupDraft[] = [
  {
    id: 'top',
    title: 'Top of the space',
    entries: [{ id: 'dashboard', title: 'Dashboard', kind: 'Dashboard', target: 'Rela Development' }],
  },
  {
    id: 'work',
    title: 'Work',
    entries: [
      { id: 'active', title: 'Active tickets', kind: 'List', target: 'Active tickets', badges: 'blocked, in progress' },
      { id: 'all', title: 'All tickets', kind: 'List', target: 'All tickets', badges: 'in review' },
      { id: 'bugs', title: 'Bugs', kind: 'List', target: 'All bugs', badges: 'critical' },
      { id: 'board', title: 'Ticket board', kind: 'Board', target: 'Ticket board' },
    ],
  },
  {
    id: 'plan',
    title: 'Planning',
    entries: [
      { id: 'features', title: 'Features', kind: 'List', target: 'All features' },
      { id: 'feature-board', title: 'Feature board', kind: 'Board', target: 'Feature board' },
      { id: 'roadmap', title: 'Roadmap', kind: 'Calendar', target: 'Release calendar' },
    ],
  },
]

export interface DashboardCardRow {
  id: string
  title: string
  shows: 'Count' | 'Breakdown' | 'Table'
  query: string
}

export const dashboardCards: DashboardCardRow[] = [
  { id: 'active-work', title: 'Active work', shows: 'Count', query: "Tickets where status is In progress" },
  { id: 'open-bugs', title: 'Open bugs', shows: 'Count', query: "Bugs where status is not Done or Won't fix" },
  { id: 'features', title: 'Features', shows: 'Breakdown', query: 'Features, grouped by status' },
  { id: 'ideas', title: 'Ideas', shows: 'Breakdown', query: 'Ideas, grouped by status' },
  { id: 'critical', title: 'Critical issues', shows: 'Table', query: 'Bugs where priority is Critical and status is not Done' },
]

// --- Migrations ---

/** One step of a data migration, worded like a change in the review. */
export interface MigrationStep {
  kind: 'added' | 'changed' | 'removed'
  label: string
  detail?: string
  before?: string
  after?: string
}

export interface MigrationRow {
  id: string
  title: string
  date: string
  by: string
  records: number
  /** Generated from a saved draft, or written by hand for a data-only fix. */
  origin: 'Saved draft' | 'Data only'
  steps: MigrationStep[]
}

export const migrations: MigrationRow[] = [
  {
    id: 'm5',
    title: 'Remove the Blocked status',
    date: 'Today, 22:41',
    by: 'You',
    records: 3,
    origin: 'Saved draft',
    steps: [
      { kind: 'changed', label: 'Status of 3 tickets', before: 'Blocked', after: 'On hold' },
      { kind: 'removed', label: 'Option Blocked', detail: 'from Ticket status' },
    ],
  },
  {
    id: 'm4',
    title: 'Rename Estimate to Effort',
    date: '12 Sep 2026',
    by: 'Maartje de Boer',
    records: 418,
    origin: 'Saved draft',
    steps: [{ kind: 'changed', label: 'Property of Ticket', before: 'Estimate', after: 'Effort', detail: '418 tickets' }],
  },
  {
    id: 'm3',
    title: 'Fill in Started for tickets in progress',
    date: '2 Sep 2026',
    by: 'Sam Okafor',
    records: 37,
    origin: 'Data only',
    steps: [{ kind: 'changed', label: 'Started of 37 tickets', before: 'empty', after: 'date of first In progress' }],
  },
  {
    id: 'm2',
    title: 'Turn Priority into a choice list',
    date: '18 Aug 2026',
    by: 'Maartje de Boer',
    records: 1204,
    origin: 'Saved draft',
    steps: [
      { kind: 'added', label: 'Choice list Priority', detail: 'Low, Medium, High, Critical' },
      { kind: 'changed', label: 'Priority of 1,204 tickets', before: 'Text', after: 'Choice list' },
      { kind: 'changed', label: 'Priority of 9 tickets', before: 'urgent', after: 'Critical' },
    ],
  },
  {
    id: 'm1',
    title: 'Split Bug out of Ticket',
    date: '30 Jun 2026',
    by: 'Sam Okafor',
    records: 212,
    origin: 'Saved draft',
    steps: [
      { kind: 'added', label: 'Entity type Bug' },
      { kind: 'changed', label: '212 tickets of kind Bug', before: 'Ticket', after: 'Bug', detail: 'IDs keep working' },
      { kind: 'removed', label: 'Option Bug', detail: 'from Ticket kind' },
    ],
  },
]
