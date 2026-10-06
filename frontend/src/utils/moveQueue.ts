/**
 * Sends row moves one at a time, in the order they were made, and settles
 * the screen once all of them are sent.
 *
 * One at a time: two arrow-key presses in quick succession are two
 * requests, and each is planned on the screen the last one left. Sent
 * together they could arrive in either order, and the stored order would
 * then differ from the one on screen.
 *
 * `settle` runs once the queue is empty, not after each move: refetching
 * between two queued moves would briefly show the first move's result over
 * the second's optimistic one.
 *
 * A send or settle that fails does not stop later moves. Each caller
 * reports its own errors; the queue only swallows them so the next move
 * still runs.
 */
export function createMoveQueue(settle: () => Promise<void>) {
  let tail: Promise<void> = Promise.resolve()
  let pending = 0

  return function enqueue(send: () => Promise<void>): Promise<void> {
    pending++
    tail = tail
      .then(send)
      .catch(() => {})
      .then(async () => {
        pending--
        if (pending === 0) await settle().catch(() => {})
      })
    return tail
  }
}
