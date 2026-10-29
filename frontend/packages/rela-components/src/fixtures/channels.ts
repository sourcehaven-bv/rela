/**
 * Sample data for the channel stories: an incident channel on a data breach,
 * with a plain text type, a structured decision, a finding that can be
 * promoted to a risk, and an ephemeral status signal.
 */
import type {
  ChannelEnumOption,
  ChannelEphemeralView,
  ChannelGroupView,
  ChannelLifecycleDef,
  ChannelMessageTypeDef,
  ChannelComposerTransition,
  ChannelDecisionView,
  ChannelMessageView,
  ChannelPresentation,
  ChannelReadableRef,
  ChannelSessionView,
} from '../components/channel/types'

/**
 * The stories' stand-in for the app's markdown renderer: it escapes the text
 * and wraps paragraphs, nothing more. The real renderer and its sanitizer
 * live in the app and are tested there; the library inserts whatever HTML it
 * is given, so its stories must not invent a second sanitizer.
 */
export function storyHtml(src: string): string {
  const escaped = src
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
  return escaped
    .split(/\n{2,}/)
    .map((p) => `<p>${p.replace(/`([^`]+)`/g, '<code>$1</code>')}</p>`)
    .join('')
}

export const channelMessageTypes: ChannelMessageTypeDef[] = [
  {
    name: 'text',
    label: 'Message',
    persist: 'log',
    reply: true,
    properties: [{ name: 'body', label: 'Message', type: 'markdown', required: true, max: 4000 }],
    actions: [],
  },
  {
    name: 'decision',
    label: 'Decision',
    persist: 'log',
    reply: false,
    properties: [
      { name: 'summary', label: 'summary', type: 'string', required: true, max: 140 },
      { name: 'due', label: 'due', type: 'date' },
      { name: 'internal_note', label: 'internal_note', type: 'string' },
      { name: 'notify_dpa', label: 'Notify the DPA', type: 'boolean', required: true },
      { name: 'owner', label: 'owner', type: 'ref', to: ['person'] },
      {
        name: 'severity',
        label: 'severity',
        type: 'enum',
        enum: 'severity',
        values: ['low', 'medium', 'high'],
      },
    ],
    actions: [],
  },
  {
    name: 'finding',
    label: 'Finding',
    persist: 'log',
    reply: true,
    properties: [
      { name: 'title', label: 'Title', type: 'string', required: true },
      { name: 'detail', label: 'Detail', type: 'markdown' },
      { name: 'affected', label: 'Affected records', type: 'integer' },
      { name: 'system', label: 'System', type: 'ref', to: ['system'] },
    ],
    actions: [
      { name: 'promote', label: 'Promote to risk', promote_to: 'risk' },
      { name: 'ack', label: 'Acknowledge' },
    ],
  },
  {
    name: 'status',
    label: 'Status',
    persist: 'ephemeral',
    reply: false,
    properties: [{ name: 'text', label: 'Status', type: 'string', max: 80 }],
    actions: [],
  },
]

export const channelPresentation: ChannelPresentation = {
  messages: {
    decision: {
      fields: [
        { property: 'summary', label: 'Decision', widget: 'textarea', help: 'One sentence.' },
        { property: 'severity', label: 'Severity', span: 6 },
        { property: 'owner', label: 'Owner', span: 6, placeholder: 'Choose a person…' },
        { property: 'internal_note', hidden: true },
      ],
    },
  },
}

export const channelEnumOptions: Record<string, ChannelEnumOption[]> = {
  severity: [
    { value: 'low', label: 'Low', color: 'green' },
    { value: 'medium', label: 'Medium', color: 'amber' },
    { value: 'high', label: 'High', color: 'red' },
  ],
}

export const channelLifecycle: ChannelLifecycleDef[] = [
  { name: 'close', label: 'Close' },
  { name: 'reopen', label: 'Reopen' },
  { name: 'archive', label: 'Archive' },
  { name: 'delete', label: 'Delete channel', danger: true },
]

const marieke = { name: 'Marieke de Vries' }
const joost = { name: 'Joost Bakker' }
const agent = { name: 'Triage agent', kind: 'agent' as const }
const system = { name: 'rela', kind: 'system' as const }

export const channelMessages: ChannelMessageView[] = [
  {
    seq: 1,
    rev: 1,
    type: 'text',
    author: system,
    at: '08:55',
    datetime: '2026-10-11T08:55:00Z',
    props: { body: 'Incident opened from the `DL-2026-014` intake form.' },
    html: { body: storyHtml('Incident opened from the `DL-2026-014` intake form.') },
  },
  {
    seq: 2,
    rev: 2,
    type: 'finding',
    author: agent,
    at: '09:02',
    datetime: '2026-10-11T09:02:00Z',
    props: {
      title: 'Bucket policy allows public read',
      detail: 'The export bucket has `AllUsers: READ` since the migration on 3 October.',
      affected: 18240,
      system: { id: 'SYS-12', type: 'system', title: 'Customer export' },
    },
    html: { detail: storyHtml('The export bucket has `AllUsers: READ` since the migration on 3 October.') },
    actions: ['promote', 'ack'],
  },
  {
    seq: 3,
    rev: 4,
    type: 'text',
    author: marieke,
    at: '09:10',
    datetime: '2026-10-11T09:10:00Z',
    editedAt: 'Edited at 09:12',
    props: { body: 'Policy reverted. Checking the access logs now.' },
    html: { body: storyHtml('Policy reverted. Checking the access logs now.') },
    replyTo: { seq: 2, author: 'Triage agent', excerpt: 'Bucket policy allows public read' },
    actions: ['edit', 'delete'],
  },
  {
    seq: 4,
    rev: 5,
    removed: { kind: 'moderator', by: 'Joost Bakker', reason: 'Posted customer data in the clear', at: '09:14' },
  },
  {
    seq: 5,
    rev: 6,
    type: 'decision',
    author: joost,
    at: '09:20',
    datetime: '2026-10-11T09:20:00Z',
    props: {
      summary: 'Notify the DPA within 72 hours.',
      severity: 'high',
      owner: { restricted: true },
      notify_dpa: true,
      due: '2026-10-14',
      internal_note: 'Not shown: hidden by the presentation.',
    },
    text: { due: '14 Oct 2026' },
    actions: ['remove'],
  },
  {
    seq: 6,
    rev: 8,
    type: 'finding',
    author: agent,
    at: '09:31',
    datetime: '2026-10-11T09:31:00Z',
    props: { title: 'Access log shows 3 external downloads', affected: 3 },
    action: { name: 'promote', status: 'pending' },
    actions: ['release'],
  },
  {
    seq: 7,
    rev: 9,
    type: 'finding',
    author: agent,
    at: '09:40',
    datetime: '2026-10-11T09:40:00Z',
    props: { title: 'Encryption at rest disabled' },
    action: {
      name: 'promote',
      status: 'done',
      promoted: { id: 'RISK-88', type: 'risk', title: 'Unencrypted export storage' },
    },
  },
]

export const channelEphemeral: ChannelEphemeralView[] = [
  { id: 'sig-1', type: 'status', author: joost, props: { text: 'reviewing access logs' } },
]

export const channelGroups: ChannelGroupView[] = [
  {
    type: 'incident',
    label: 'Incident room',
    cardinality: 'one',
    canCreate: false,
    channels: [{ key: 'incident', title: 'Incident room', state: 'open', summary: 'Joost: Notify the DPA within 72 hours.' }],
  },
  {
    type: 'topic',
    label: 'Topics',
    cardinality: 'many',
    canCreate: true,
    createLabel: 'New topic',
    channels: [
      { key: 'topic/comms', title: 'Customer communication', state: 'open', summary: '3 messages today' },
      { key: 'topic/forensics', title: 'Forensics', state: 'closed' },
    ],
  },
  {
    type: 'legal',
    label: 'Legal',
    cardinality: 'one',
    canCreate: false,
    channels: [],
  },
]

/** The person the stories' ref picker always returns. */
export const pickedPerson: ChannelReadableRef = { id: 'PER-7', type: 'person', title: 'Sanne Visser' }

/**
 * A long log, for the scrolling and memory budget the app caps at 500.
 * `first` is the seq of the oldest message, for a page loaded above another.
 */
export function manyChannelMessages(count: number, first = 1): ChannelMessageView[] {
  return Array.from({ length: count }, (_, j) => {
    const i = first - 1 + j
    const body = `Message ${i + 1}.`
    return {
      seq: i + 1,
      rev: i + 1,
      type: 'text',
      author: i % 3 === 0 ? marieke : joost,
      at: `${String(8 + Math.floor(i / 60) % 12).padStart(2, '0')}:${String(i % 60).padStart(2, '0')}`,
      props: { body },
      html: { body: storyHtml(body) },
    }
  })
}

/*
 * The breach room: a channel with a session, ported from the realtime
 * mockup. The session runs Detected, Contained, Risk assessed, AP notified,
 * with Dismissed as a second end state to show a branch.
 */

export const breachMessageTypes: ChannelMessageTypeDef[] = [
  {
    name: 'contained',
    label: 'Contained',
    persist: 'log',
    reply: true,
    properties: [{ name: 'note', label: 'Note', type: 'markdown' }],
    actions: [],
  },
  {
    name: 'assessed',
    label: 'Risk assessed',
    persist: 'log',
    reply: false,
    properties: [{ name: 'risk', label: 'Risk', type: 'string', required: true }],
    actions: [],
  },
  {
    name: 'notified',
    label: 'AP notified',
    persist: 'log',
    reply: false,
    properties: [{ name: 'reference', label: 'Reference', type: 'string', required: true }],
    actions: [],
  },
  {
    name: 'dismiss',
    label: 'Dismiss',
    persist: 'log',
    reply: false,
    properties: [{ name: 'reason', label: 'Reason', type: 'string', required: true }],
    actions: [],
  },
  {
    name: 'text',
    label: 'Text',
    persist: 'log',
    reply: true,
    properties: [{ name: 'body', label: 'Message', type: 'markdown', required: true, max: 4000 }],
    actions: [],
  },
  {
    name: 'session',
    label: 'Session',
    persist: 'log',
    reply: false,
    properties: [],
    actions: [],
  },
]

const breachStates = [
  { name: 'detected', label: 'Detected' },
  { name: 'contained', label: 'Contained' },
  { name: 'assessed', label: 'Risk assessed' },
  { name: 'notified', label: 'AP notified', final: true },
  { name: 'dismissed', label: 'Dismissed', final: true },
]

/** The breach session in Detected, where it may move to Contained or Dismissed. */
export const breachSessionDetected: ChannelSessionView = {
  label: 'GDPR breach response',
  states: breachStates,
  current: 'detected',
  currentLabel: 'Detected',
  next: ['contained', 'dismissed'],
}

/** The breach session in Risk assessed. */
export const breachSessionAssessed: ChannelSessionView = {
  ...breachSessionDetected,
  current: 'assessed',
  currentLabel: 'Risk assessed',
  next: ['notified'],
}

/** The breach session after the AP was notified: it has ended. */
export const breachSessionFinal: ChannelSessionView = {
  ...breachSessionDetected,
  current: 'notified',
  currentLabel: 'AP notified',
  next: [],
  final: true,
}

/** The breach session in a state the configuration no longer declares. */
export const breachSessionStranded: ChannelSessionView = {
  ...breachSessionDetected,
  current: undefined,
  currentLabel: 'triage',
  next: [],
  stranded: true,
}

/** A session whose current state the held states do not include yet. */
export const breachSessionLoading: ChannelSessionView = {
  ...breachSessionDetected,
  current: undefined,
  currentLabel: 'triaged',
  next: [],
  loading: true,
}

/** The breach session in Contained, waiting for the incident lead. */
export const breachSessionInputRequired: ChannelSessionView = {
  ...breachSessionDetected,
  current: 'contained',
  currentLabel: 'Contained',
  next: ['assessed'],
  status: 'input_required',
  waitingFor: [{ label: 'the incident lead' }],
}

/** The breach session in Contained, waiting for the reader. */
export const breachSessionWaitingOnMe: ChannelSessionView = {
  ...breachSessionInputRequired,
  waitingOnMe: true,
}

/** The breach session dismissed: it ended as cancelled. */
export const breachSessionCancelled: ChannelSessionView = {
  ...breachSessionFinal,
  current: 'dismissed',
  currentLabel: 'Dismissed',
  status: 'cancelled',
}

/** The breach session in a final state whose status is failed. */
export const breachSessionFailed: ChannelSessionView = {
  ...breachSessionFinal,
  status: 'failed',
}

/** The transitions the lead may fire in Detected. */
export const breachTransitions: ChannelComposerTransition[] = [
  { messageType: 'contained', toLabels: ['Contained'] },
  { messageType: 'dismiss', toLabels: ['Dismissed'] },
]

const ann = { name: 'Ann' }

/** A session notice as the app maps it. */
export function breachNotice(seq: number, fromLabel: string, toLabel: string, at = '09:12'): ChannelMessageView {
  return {
    seq,
    rev: seq,
    type: 'session',
    author: system,
    at,
    datetime: `2026-10-11T${at}:00Z`,
    props: {},
    notice: { fromLabel, toLabel, reentry: fromLabel === toLabel },
  }
}

export const breachMessages: ChannelMessageView[] = [
  {
    seq: 1,
    rev: 1,
    type: 'text',
    author: marieke,
    at: '09:02',
    datetime: '2026-10-11T09:02:00Z',
    props: { body: 'The export bucket is publicly readable. Revoking access now.' },
    html: { body: storyHtml('The export bucket is publicly readable. Revoking access now.') },
  },
  {
    seq: 2,
    rev: 2,
    type: 'contained',
    author: ann,
    at: '09:11',
    datetime: '2026-10-11T09:11:00Z',
    props: { note: 'Public access revoked.' },
    html: { note: storyHtml('Public access revoked.') },
  },
  breachNotice(3, 'Detected', 'Contained', '09:11'),
  {
    seq: 4,
    rev: 4,
    type: 'assessed',
    author: joost,
    at: '09:30',
    datetime: '2026-10-11T09:30:00Z',
    props: { risk: 'high' },
  },
  breachNotice(5, 'Contained', 'Risk assessed', '09:30'),
]

/** The decisions the lead may make in Contained: one opens the form, two post, one of them ends the session. */
export const breachDecisions: ChannelDecisionView[] = [
  {
    key: 'assessed',
    messageType: 'assessed',
    label: 'Risk assessed',
    toLabel: 'Risk assessed',
    opensForm: true,
  },
  { key: 'contained', messageType: 'contained', label: 'Contained again', toLabel: 'Contained' },
  {
    key: 'dismiss',
    messageType: 'dismiss',
    label: 'Dismiss',
    toLabel: 'Dismissed',
    confirm: true,
    danger: true,
  },
]

/**
 * An agent's run with a question (TKT-03FQPD): `ask` carries a list of
 * options, and a click posts `answer` with the option in `note`.
 */
export const questionMessageTypes: ChannelMessageTypeDef[] = [
  {
    name: 'ask',
    label: 'Question',
    persist: 'log',
    reply: true,
    properties: [
      { name: 'note', label: 'Question', type: 'string' },
      { name: 'options', label: 'Options', type: 'string', list: true, max: 80 },
    ],
    actions: [],
    choices: { from: 'options', answer: 'answer', into: 'note', other: true },
  },
  {
    name: 'answer',
    label: 'Answer',
    persist: 'log',
    reply: true,
    properties: [{ name: 'note', label: 'Answer', type: 'string' }],
    actions: [],
  },
]

/** An option of 200 characters, the default item limit. */
const longOption =
  'Keep the current schema for now and migrate the reporting tables in a separate change after the release, ' +
  'so the importer can ship on PostgreSQL first and the reports follow once the new views pass QA.'

/** A question the agent asked, with three options, one of them long. */
export const questionMessage: ChannelMessageView = {
  seq: 12,
  rev: 12,
  type: 'ask',
  author: agent,
  at: '10:02',
  datetime: '2026-10-29T10:02:00Z',
  props: { note: 'Which storage backend should the importer use?', options: ['PostgreSQL', 'SQLite', longOption] },
}

/** The reply buttons of `questionMessage`, as the app builds them. */
export const questionChoices: ChannelDecisionView[] = [
  { key: 'choice:12:0', messageType: 'answer', label: 'PostgreSQL', props: { note: 'PostgreSQL' } },
  { key: 'choice:12:1', messageType: 'answer', label: 'SQLite', props: { note: 'SQLite' } },
  { key: 'choice:12:2', messageType: 'answer', label: longOption, props: { note: longOption } },
  { key: 'choice:12:other', messageType: 'answer', label: 'Other answer…', opensForm: true },
]

/** The person's answer to `questionMessage`: after it, the question has no buttons. */
export const answerMessage: ChannelMessageView = {
  seq: 13,
  rev: 13,
  type: 'answer',
  author: marieke,
  at: '10:05',
  datetime: '2026-10-29T10:05:00Z',
  props: { note: 'SQLite' },
  replyTo: { seq: 12, author: agent.name, excerpt: 'Which storage backend should the importer use?' },
}

/*
 * An agent run with quiet message types (TKT-03DMXH): the runner's `claim`
 * and `resume` are hidden and its `progress` notes are collapsed, while
 * `plan` and `text` show in full.
 */

export const runMessageTypes: ChannelMessageTypeDef[] = [
  {
    name: 'text',
    label: 'Message',
    persist: 'log',
    reply: true,
    properties: [{ name: 'body', label: 'Message', type: 'markdown', required: true }],
    actions: [],
  },
  {
    name: 'claim',
    label: 'Claim',
    persist: 'log',
    reply: false,
    properties: [{ name: 'worktree', label: 'Worktree', type: 'string', required: true }],
    actions: [],
  },
  {
    name: 'resume',
    label: 'Resume',
    persist: 'log',
    reply: false,
    properties: [{ name: 'worktree', label: 'Worktree', type: 'string', required: true }],
    actions: [],
  },
  {
    name: 'progress',
    label: 'Progress',
    persist: 'log',
    reply: false,
    properties: [{ name: 'body', label: 'Status', type: 'markdown', required: true }],
    actions: [],
  },
  {
    name: 'plan',
    label: 'Plan',
    persist: 'log',
    reply: true,
    properties: [{ name: 'plan', label: 'Plan', type: 'markdown', required: true }],
    actions: [],
  },
]

export const runPresentation: ChannelPresentation = {
  messages: {
    claim: { display: 'hidden' },
    resume: { display: 'hidden' },
    progress: { display: 'collapsed' },
  },
}

/** One message of the run, of the given type, by the runner unless it is `text`. */
export function runMessage(seq: number, type: string, body = `Step ${seq}.`): ChannelMessageView {
  const prop = type === 'claim' || type === 'resume' ? 'worktree' : type === 'plan' ? 'plan' : 'body'
  const value = prop === 'worktree' ? `/work/run-${seq}` : body
  return {
    seq,
    rev: seq,
    type,
    author: type === 'text' ? marieke : agent,
    at: `09:${String(seq % 60).padStart(2, '0')}`,
    props: { [prop]: value },
    ...(prop === 'worktree' ? {} : { html: { [prop]: storyHtml(value) } }),
  }
}

/**
 * A run: the owner asks, the runner claims (hidden) and reports progress
 * (collapsed) around a hidden resume, then posts a plan, and reports more
 * progress at the end.
 */
export const runMessages: ChannelMessageView[] = [
  runMessage(1, 'text', 'Please pick this up.'),
  runMessage(2, 'claim'),
  runMessage(3, 'progress', 'Reading the code.'),
  runMessage(4, 'resume'),
  runMessage(5, 'progress', 'Writing the plan.'),
  runMessage(6, 'plan', 'Add a `display` key and fold runs in the log.'),
  runMessage(7, 'progress', 'Waiting for review.'),
  runMessage(8, 'progress', 'Still waiting.'),
]

/**
 * A long run for the scroll stories: plain messages with a collapsed
 * `progress` and a hidden `claim` after every third one.
 */
export function longRunMessages(count: number): ChannelMessageView[] {
  return Array.from({ length: count }, (_, i) => {
    const seq = i + 1
    return runMessage(seq, seq % 3 === 1 ? 'text' : seq % 3 === 2 ? 'progress' : 'claim', `Message ${seq}.`)
  })
}
