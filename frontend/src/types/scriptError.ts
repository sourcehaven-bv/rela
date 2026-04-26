/**
 * Mirrors internal/dataentry/script_errors.go ScriptErrorEnvelope.
 *
 * Returned by every Lua surface (action, document, automation,
 * MCP lua_run/lua_eval) on failure. Caller-side branching in
 * api/client.ts: `error === 'script_error'`.
 *
 * Loopback-gated fields (source, stack, captured_output) are absent
 * for non-loopback callers unless the operator opted in via
 * data-entry.yaml.
 */
export interface ScriptError {
  error: 'script_error'
  correlation_id?: string
  script: ScriptIdentity
  lua: ScriptErrorLua
  source?: SourceLine[]
  stack?: StackFrame[]
  captured_output?: string
}

export interface ScriptIdentity {
  surface: 'action' | 'document' | 'automation' | 'lua_run' | 'lua_eval' | 'validation'
  path: string
  entity_id?: string
  args?: Record<string, unknown>
}

export interface ScriptErrorLua {
  message: string
  line?: number
}

export interface SourceLine {
  n: number
  text: string
  highlight?: boolean
}

export interface StackFrame {
  path?: string
  line?: number
  func?: string
}

/** Type guard: does this caught error look like a ScriptError envelope? */
export function isScriptError(err: unknown): err is ScriptError {
  return (
    typeof err === 'object' &&
    err !== null &&
    (err as { error?: unknown }).error === 'script_error'
  )
}
