/** Severity shared by the banner, the callout and the toast. */
export type MessageTone = 'info' | 'success' | 'warning' | 'danger'

/** One queued toast. `id` is what a dismiss refers to. */
export interface ToastMessage {
  id: string
  title: string
  description?: string
  tone?: MessageTone
  /** Milliseconds before it leaves on its own; 0 keeps it until dismissed. */
  duration?: number
  /**
   * One control in the toast, for the act the message invites: undoing a
   * delete, retrying a failed save, opening what was just created.
   *
   * One rather than several, because a toast leaves on a timer and a choice
   * the user has five seconds to weigh is not a choice. Anything needing two
   * options needs a dialog instead.
   */
  action?: ToastAction
}

export interface ToastAction {
  /** What the control says. A verb: "Undo", "Retry", "View". */
  label: string
  /**
   * Run when the control is pressed. The toast dismisses itself afterwards,
   * because the message described a state the action has now changed.
   */
  onAction: () => void
}
