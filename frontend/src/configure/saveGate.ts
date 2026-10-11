/**
 * Whether this tab is saving the configuration right now.
 *
 * A save makes the server broadcast `config-changed`, and every open tab
 * reloads on it. The tab that saved must not: it reloads itself once the
 * save has answered, so it can show what happened first.
 */
import { relaBase } from '@/api/base'

let saving = false

export function beginConfigSave(): void {
  saving = true
}

export function endConfigSave(): void {
  saving = false
}

export function isConfigSaveInFlight(): boolean {
  return saving
}

const SAVED_FLAG = 'rela:configure-saved'

/** Remembers, across the reload a save ends in, to say that it worked. */
export function markConfigSaved(): void {
  try {
    sessionStorage.setItem(SAVED_FLAG, '1')
  } catch {
    // Storage refused: the reload simply goes without the message.
  }
}

/** Whether the page was loaded by a save. Reads the flag once. */
export function takeConfigSaved(): boolean {
  try {
    const set = sessionStorage.getItem(SAVED_FLAG) === '1'
    sessionStorage.removeItem(SAVED_FLAG)
    return set
  } catch {
    return false
  }
}

function incompleteKey(): string {
  return `rela:configure-incomplete:${relaBase() || '/'}`
}

/**
 * Remembers that a save switched the configuration but its migration did not
 * finish. The save also makes the server broadcast `config-changed`, which
 * reloads this tab, so the "Finish migration" prompt must outlive the reload.
 * The flag stays until a retry succeeds.
 */
export function setConfigIncomplete(incomplete: boolean): void {
  try {
    if (incomplete) sessionStorage.setItem(incompleteKey(), '1')
    else sessionStorage.removeItem(incompleteKey())
  } catch {
    // Storage refused: the prompt lasts until the next reload only.
  }
}

/** Whether a save left a migration to finish. Does not clear the flag. */
export function isConfigIncomplete(): boolean {
  try {
    return sessionStorage.getItem(incompleteKey()) === '1'
  } catch {
    return false
  }
}
