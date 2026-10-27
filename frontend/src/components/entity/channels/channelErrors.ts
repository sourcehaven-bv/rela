import { ApiError, getErrorMessage } from '@/api/errors'

/**
 * How the channel panel answers a failed channel operation (TKT-91BK6F R12).
 *
 * The answer is chosen by the problem `type`, not the status, because one
 * status covers refusals that need different handling: a 409 can mean the
 * channel closed (read the channel again) or the message type is gone
 * (reload the descriptor).
 *
 * - `place` says where the message shows: `form` in the composer or dialog
 *   that sent the request, `toast` everywhere else. An operation without a
 *   form (an action, a lifecycle button) shows a `form` error as a toast.
 * - `then` says what the panel does next: `refetch` reads the channel again
 *   (its affordances or state moved), `list` returns to the channel list
 *   (the channel is gone), `schema` reloads the channel type descriptor.
 *
 * None of these clears the open channel by itself. Only a refused refetch
 * does that, inside useChannel, per shouldDropHeldContent: a write the
 * server refuses says nothing about whether the user may still read.
 */
export type ChannelErrorPlace = 'form' | 'toast'
export type ChannelErrorThen = 'none' | 'refetch' | 'list' | 'schema'

export interface ChannelErrorRule {
  place: ChannelErrorPlace
  then: ChannelErrorThen
}

const FORM: ChannelErrorRule = { place: 'form', then: 'none' }
const TOAST: ChannelErrorRule = { place: 'toast', then: 'none' }
const REFETCH: ChannelErrorRule = { place: 'toast', then: 'refetch' }
const LIST: ChannelErrorRule = { place: 'toast', then: 'list' }
const SCHEMA: ChannelErrorRule = { place: 'toast', then: 'schema' }

/**
 * The rule per realtime refusal type (internal/realtime/errors.go).
 * channelErrors.test.ts reads that file and fails when a type is missing.
 *
 * Denials refetch: the button was offered, so the affordances the panel
 * holds are older than the server's answer.
 */
export const CHANNEL_ERROR_RULES: Readonly<Record<string, ChannelErrorRule>> = {
  // The request itself: shown where it was written.
  invalid_message: FORM,
  too_many_props: FORM,
  reason_required: FORM,
  too_large: FORM,
  invalid_ref: FORM,
  promote_invalid: FORM,
  face_required: FORM,
  // Gone: the message refetches the page, the channel returns to the list.
  message_not_found: REFETCH,
  channel_not_found: LIST,
  unknown_channel_type: LIST,
  // Only the MCP read_session tool returns it; the SPA reads sessions on the channel.
  no_session: LIST,
  // The configuration moved under the request.
  message_type_gone: SCHEMA,
  channel_type_conflict: SCHEMA,
  config_changed: SCHEMA,
  // Refused for this caller now.
  no_principal: TOAST,
  channel_read_denied: REFETCH,
  channel_post_denied: REFETCH,
  channel_create_denied: REFETCH,
  message_edit_denied: REFETCH,
  edit_window_closed: REFETCH,
  message_delete_denied: REFETCH,
  lifecycle_not_offered: REFETCH,
  lifecycle_denied: REFETCH,
  action_not_offered: REFETCH,
  // useTyping never shows a typing refusal; the rule keeps the map complete.
  typing_not_offered: REFETCH,
  action_denied: REFETCH,
  promote_denied: REFETCH,
  session_output_denied: REFETCH,
  // The channel or message is not in the state the request needs.
  channel_not_open: REFETCH,
  removed: REFETCH,
  conflict: REFETCH,
  lifecycle_wrong_state: REFETCH,
  action_taken: REFETCH,
  action_pending: REFETCH,
  action_not_pending: REFETCH,
  promote_conflict: REFETCH,
  session_notice: REFETCH,
  session_message: REFETCH,
  reminder_message: REFETCH,
  outputs_pending: REFETCH,
  // Limits and rules that reading again does not change.
  channel_not_creatable: TOAST,
  too_many_channels: TOAST,
  retention_active: TOAST,
  signals_not_kept: TOAST,
  rate_limited: TOAST,
}

export interface ChannelErrorOutcome extends ChannelErrorRule {
  message: string
}

const TYPE_PREFIX = 'https://rela.dev/errors/'

/** The bare problem type, such as `channel_not_open`, or '' for none. */
export function problemType(err: unknown): string {
  const type = err instanceof ApiError ? err.problem?.type : undefined
  if (!type) return ''
  return type.startsWith(TYPE_PREFIX) ? type.slice(TYPE_PREFIX.length) : type
}

/** The whole seconds a 429 asks to wait, from its Retry-After header. */
function retryAfterSeconds(err: ApiError): number | undefined {
  const original = err.original as { response?: { headers?: Record<string, unknown> } } | null
  const raw = original?.response?.headers?.['retry-after']
  const n = typeof raw === 'string' || typeof raw === 'number' ? Number(raw) : NaN
  return Number.isFinite(n) && n > 0 ? Math.ceil(n) : undefined
}

/** The server's title as written, with the rule a 403 names in `detail`. */
function problemMessage(err: unknown): string {
  const problem = err instanceof ApiError ? err.problem : undefined
  const title = problem?.title?.trim()
  if (!title) return getErrorMessage(err, 'The channel request failed.')
  const detail = problem?.detail?.trim()
  return detail ? `${title} (${detail})` : title
}

/**
 * Classifies a failed channel request. The message is the server's: for a
 * 403 it names the rule the operation needs, which the operator wrote in
 * realtime.yaml. An error without a known type is a toast that changes
 * nothing, except a 404 `not_found` (the entity went away), which reads
 * the channel again so useChannel can drop what it holds.
 */
export function classifyChannelError(err: unknown): ChannelErrorOutcome {
  const type = problemType(err)
  const message = problemMessage(err)
  if (type === 'rate_limited' && err instanceof ApiError) {
    const secs = retryAfterSeconds(err)
    return {
      ...TOAST,
      message: secs
        ? `Too many messages. Try again in ${secs} s.`
        : 'Too many messages. Try again shortly.',
    }
  }
  const rule = CHANNEL_ERROR_RULES[type]
  if (rule) return { ...rule, message }
  if (err instanceof ApiError && err.status === 404) return { ...REFETCH, message }
  return { ...TOAST, message }
}
