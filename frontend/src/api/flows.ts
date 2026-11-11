import { api } from './client'

/**
 * Typed client for the flow routes under `/_flows` (TKT-ZAXRXND). The shapes
 * mirror `internal/flowview`.
 *
 * Every read has passed the caller's row gate on the server: a run the caller
 * cannot read answers 404, like a hidden entity, and a project without flows
 * answers 404 on every route. The `_actions` of a run are computed with the
 * same check its POST runs, so the client shows a control only when the key
 * says so and still handles the refusal.
 */

/** A run's status. The values are those of MCP tasks. */
export type FlowRunStatus = 'working' | 'input_required' | 'completed' | 'cancelled' | 'failed'

/** The statuses in the order a filter offers them. */
export const FLOW_RUN_STATUSES: readonly FlowRunStatus[] = [
  'input_required',
  'working',
  'completed',
  'cancelled',
  'failed',
]

/** One flow that flows.yaml declares. */
export interface Flow {
  name: string
  label: string
  /** The entity type the flow runs on. */
  subject: string
  version: string
  /** The flow has an `on: manual` start. */
  manual: boolean
  /** The flow as a Mermaid state diagram. */
  diagram: string
}

export interface FlowTimer {
  name: string
  label?: string
  to: string
  /** RFC 3339; absent when the timer has no due time yet. */
  due?: string
}

export interface FlowBranch {
  to: string
  label?: string
  roles?: string[]
  guard?: string
  /** The guard's verdict on the subject now. */
  holds: boolean
  /** The guard could not be evaluated. */
  failed?: boolean
}

export interface FlowPending {
  failures: number
  next_attempt?: string
}

/** Why an active run waits and what can move it. */
export interface FlowWaiting {
  roles?: string[]
  /** The caller holds one of `roles` on the subject. */
  you_hold: boolean
  /** The subject properties the step form edits. */
  writes?: string[]
  timers?: FlowTimer[]
  branches?: FlowBranch[]
  /** Set while the run writes the outputs of its state. */
  pending?: FlowPending
  /** Who is completing the step right now. */
  claimed_by?: string
  /** The run's state or definition is gone. */
  stranded?: boolean
}

export interface FlowEvent {
  at: string
  state: string
  /** The state's label, or its name when the flow lost the state. */
  label?: string
  from?: string
  via: 'start' | 'done' | 'change' | 'timer' | string
  timer?: string
  by?: string
}

export interface FlowActions {
  complete: boolean
  cancel: boolean
  retry: boolean
}

/** One run as the caller may see it. */
export interface FlowRun {
  id: string
  flow: string
  flow_label: string
  version: string
  /** The subject address; absent when the caller cannot read the subject. */
  subject?: string
  state: string
  state_label: string
  status: FlowRunStatus | string
  /** Counts the run's moves; a step names the one it saw. */
  generation: number
  started?: string
  completed?: string
  entered_at?: string
  /** False for a finished run, which keeps only its header. */
  active: boolean
  waiting?: FlowWaiting
  /** The timeline, oldest first. */
  log?: FlowEvent[]
  /** The flow's Mermaid diagram with the current state marked. */
  diagram?: string
  _actions: FlowActions
}

/** A run that waits for a role the caller holds on its subject. */
export interface FlowTask {
  run: string
  flow: string
  flow_label: string
  state: string
  state_label: string
  subject: string
  started?: string
}

export interface FlowTaskPage {
  tasks: FlowTask[]
  /** The tasks found, before paging. */
  total: number
  /** More candidates exist than one call reads; a flow filter narrows them. */
  truncated: boolean
}

export interface FlowRunPage {
  runs: FlowRun[]
  /** The runs found, before paging. */
  total: number
  /** More runs exist than one call reads; a flow or status filter narrows them. */
  truncated: boolean
}

/** The answer to a completed step. */
export interface FlowCompleted {
  /** The properties the step wrote. */
  written: string[]
  /** A change branch moved the run. */
  moved: boolean
  /** The run's state after the step. */
  state: string
  /** The run ended. */
  finished: boolean
  /**
   * Set when the values were saved but the run did not follow cleanly:
   * a guard could not be evaluated, the run moved on meanwhile, or an
   * output step failed after the move and is retried.
   */
  problem?: 'guard' | 'moved' | 'settling'
}

export interface FlowRunQuery {
  flow?: string
  status?: string
  page?: number
  perPage?: number
}

function runPath(id: string): string {
  return `/_flows/runs/${encodeURIComponent(id)}`
}

/** Drops empty values, so a filter left blank is not sent. */
function query(params: Record<string, string | number | undefined>): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') out[k] = v
  }
  return out
}

/** GET the flows of flows.yaml. Answers 404 when the project declares none. */
export async function listFlows(signal?: AbortSignal): Promise<Flow[]> {
  const res = await api.get<{ data?: Flow[] }>('/_flows', undefined, signal)
  return res.data ?? []
}

/** GET one page of the caller's tasks. */
export async function listFlowTasks(
  q: { flow?: string; page?: number; perPage?: number } = {},
  signal?: AbortSignal
): Promise<FlowTaskPage> {
  return api.get<FlowTaskPage>(
    '/_flows/tasks',
    query({ flow: q.flow, page: q.page, per_page: q.perPage }),
    signal
  )
}

/** GET one page of the run list. */
export async function listFlowRuns(
  q: FlowRunQuery = {},
  signal?: AbortSignal
): Promise<FlowRunPage> {
  return api.get<FlowRunPage>(
    '/_flows/runs',
    query({ flow: q.flow, status: q.status, page: q.page, per_page: q.perPage }),
    signal
  )
}

/** GET every run on one entity the caller can see, newest first. */
export async function listSubjectRuns(subject: string, signal?: AbortSignal): Promise<FlowRun[]> {
  const res = await api.get<FlowRunPage>('/_flows/runs', { subject }, signal)
  return res.runs ?? []
}

/** GET one run, with why it waits, its timeline and `_actions`. */
export async function getFlowRun(id: string, signal?: AbortSignal): Promise<FlowRun> {
  return api.get<FlowRun>(runPath(id), undefined, signal)
}

/** POST a manual start of `flow` on the entity at `subject`. Answers the new run id. */
export async function startFlowRun(flow: string, subject: string): Promise<string> {
  const res = await api.post<{ id: string }>('/_flows/runs', { flow, subject })
  return res.id
}

/**
 * POST the step of a run. `generation` is the one the form was rendered at;
 * the server answers 409 when the run has moved on since. `properties` may
 * name only the state's `writes`.
 */
export async function completeFlowStep(
  id: string,
  generation: number,
  properties: Record<string, unknown>
): Promise<FlowCompleted> {
  return api.post<FlowCompleted>(`${runPath(id)}/_complete`, { generation, properties })
}

/** POST a cancel of an unfinished run. */
export async function cancelFlowRun(id: string): Promise<void> {
  await api.post<void>(`${runPath(id)}/_cancel`, {})
}

/** POST a retry of the pending outputs of a run. */
export async function retryFlowRun(id: string): Promise<void> {
  await api.post<void>(`${runPath(id)}/_retry`, {})
}
