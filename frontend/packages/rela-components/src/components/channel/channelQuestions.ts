/**
 * Groups the questions of one batch (TKT-GR1MXL5). An agent that asks
 * several questions at once posts them as one batch: each message carries
 * the batch's first seq in `batch`. The log shows a run of such questions
 * as one group, so the reader sees "Question 1 of 3" and so on. A pure
 * function: no DOM, no component state.
 */
import type { ChannelRow } from './channelFold'
import type { ChannelMessageView } from './types'

/** Consecutive questions of one batch, shown as one group. */
export interface ChannelQuestionsRow {
  kind: 'questions'
  /** The Vue key, from the batch id. */
  key: string
  batch: number
  /** The questions in log order; always two or more. */
  messages: ChannelMessageView[]
}

export type ChannelGroupedRow = ChannelRow | ChannelQuestionsRow

/**
 * Merges each run of two or more message rows that share one `batch` and
 * are all questions into one questions row. A row that is not a question
 * (a statement in the same batch), a fold with messages, or a message of
 * another batch ends the run. A session notice does not: when the first
 * question of a batch moves the session, the notice lands between it and
 * the next one. A notice inside a run, alone or in a fold of notices only,
 * is drawn right after the group. A single question stays a message row,
 * so a lone question looks as it did before batches. isQuestion decides
 * what a question is; the channel passes "its type has choices".
 */
export function groupQuestions(
  rows: readonly ChannelRow[],
  isQuestion: (m: ChannelMessageView) => boolean
): ChannelGroupedRow[] {
  const out: ChannelGroupedRow[] = []
  let run: ChannelMessageView[] = []
  let held: ChannelRow[] = []
  const flush = () => {
    if (run.length > 1) {
      const batch = run[0]!.batch!
      out.push({ kind: 'questions', key: `questions-${batch}`, batch, messages: run })
    } else if (run.length === 1) {
      out.push({ kind: 'message', message: run[0]! })
    }
    out.push(...held)
    run = []
    held = []
  }
  for (const row of rows) {
    const m = row.kind === 'message' ? row.message : undefined
    const groupable = !!m && m.batch !== undefined && !m.removed && !m.notice && isQuestion(m)
    if (!groupable) {
      if (run.length && isNoticeRow(row)) {
        held.push(row)
        continue
      }
      flush()
      out.push(row)
      continue
    }
    if (run.length && run[0]!.batch !== m.batch) flush()
    run.push(m)
  }
  flush()
  return out
}

/** Reports whether row is a session notice, or a fold of notices only. */
function isNoticeRow(row: ChannelRow): boolean {
  if (row.kind === 'message') return !!row.message.notice && !row.message.removed
  return row.kind === 'fold' && row.count === 0 && row.moves > 0
}
