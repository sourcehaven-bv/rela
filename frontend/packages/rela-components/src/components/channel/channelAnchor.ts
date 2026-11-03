/**
 * The scroll anchor of a channel log: the row whose offset from the top of
 * the log is kept across a change, so the reader's view does not move. A
 * message row carries `data-seq`; a fold row carries `data-fold-seq`, its
 * last seq, and `data-fold-first`, its first seq, and is measured by its
 * button, not by its expanded run.
 */
import { rowFor, type ChannelRow } from './channelFold'

/** A row the view is kept on: a message, or a fold line by its last seq. */
export interface Anchor {
  seq: number
  /** For a fold line, the seq of its run's first member; otherwise seq. */
  first: number
  fold: boolean
  offset: number
}

/** The part of a row that is measured: a fold's button, not its expanded run. */
export function measured(row: HTMLElement): Element {
  return row.dataset.foldSeq !== undefined ? (row.firstElementChild ?? row) : row
}

/** The first row whose bottom is below the top of the log, and its offset. */
export function firstVisible(el: HTMLElement): Anchor | null {
  const top = el.getBoundingClientRect().top
  for (const row of el.querySelectorAll<HTMLElement>('[data-seq], [data-fold-seq]')) {
    const fold = row.dataset.foldSeq !== undefined
    const r = measured(row).getBoundingClientRect()
    if (r.bottom <= top) continue
    const seq = Number(fold ? row.dataset.foldSeq : row.dataset.seq)
    return { seq, first: fold ? Number(row.dataset.foldFirst) : seq, fold, offset: r.top - top }
  }
  return null
}

/**
 * The element of an anchor now, also when its seq moved into a fold or out of
 * one. A fold line that is gone (Show all, an expand that split it, a reload
 * without display) gives way to the row of its run's first member, the line
 * the reader was looking at.
 */
export function findAnchor(el: HTMLElement, a: Anchor, rows: readonly ChannelRow[]): HTMLElement | null {
  const direct = el.querySelector<HTMLElement>(a.fold ? `[data-fold-seq="${a.seq}"]` : `[data-seq="${a.seq}"]`)
  if (direct) return direct
  const row = rowFor(rows, a.first)
  if (!row) return null
  return el.querySelector<HTMLElement>(
    row.kind === 'fold' ? `[data-fold-seq="${row.last}"]` : `[data-seq="${row.message.seq}"]`,
  )
}
