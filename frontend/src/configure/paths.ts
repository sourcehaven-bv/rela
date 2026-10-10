/**
 * Key paths: which keys the Configure space may change, and which screen a
 * problem the server reports belongs on.
 *
 * The server decides what is editable (`internal/configedit/allowlist.go`)
 * and refuses a save that changes anything else. The screens ask the same
 * question first, so they show a locked key read-only instead of letting
 * someone edit it and fail at save. The patterns arrive with the snapshot;
 * the matcher here is a port of the server's.
 */
import type { ConfigFile, ConfigProblem } from '@/api/configure'
import type { TreePath } from './tree'
import { configureRoute } from './routes'

/** A path in pattern form: list indexes become `[]`. */
function patternForm(path: TreePath): string[] {
  return path.map((p) => (typeof p === 'number' ? '[]' : p))
}

const NAV_PREFIXES = [
  ['navigation', '[]'],
  ['spaces', '[]', 'navigation', '[]'],
  ['spaces', '[]', 'home'],
]

function startsWith(path: string[], prefix: string[]): boolean {
  return prefix.length <= path.length && prefix.every((p, i) => path[i] === p)
}

/** A navigation entry has the same keys wherever it sits, so its path becomes `nav.…`. */
function normalize(file: ConfigFile, path: string[]): string[] {
  if (file !== 'data-entry.yaml') return path
  for (const prefix of NAV_PREFIXES) {
    if (!startsWith(path, prefix)) continue
    let rest = path.slice(prefix.length)
    while (rest.length >= 2 && rest[0] === 'items' && rest[1] === '[]') rest = rest.slice(2)
    return ['nav', ...rest]
  }
  return path
}

/** Whether a pattern matches a whole path. `[]` may also match nothing. */
function match(pattern: string[], path: string[]): boolean {
  if (pattern.length === 0) return path.length === 0
  const [head, ...tail] = pattern
  if (head === '**') return true
  if (head === '[]') {
    if (path[0] === '[]' && match(tail, path.slice(1))) return true
    return match(tail, path)
  }
  if (path.length === 0 || path[0] === '[]') return false
  if (head !== '*' && head !== path[0]) return false
  return match(tail, path.slice(1))
}

export type EditablePatterns = Partial<Record<ConfigFile, string[]>>

/** Whether the Configure space may change the value at a path. */
export function isEditable(
  patterns: EditablePatterns | undefined,
  file: ConfigFile,
  path: TreePath
): boolean {
  const list = patterns?.[file]
  if (!list) return false
  const p = normalize(file, patternForm(path))
  return list.some((pattern) => match(pattern.split('.'), p))
}

function segments(problem: ConfigProblem): string[] {
  return problem.path ? problem.path.split('.') : []
}

/**
 * The screen a problem is shown on. A problem with no path falls back to the
 * entity type it names, and otherwise to the first screen of the file.
 */
export function problemRoute(problem: ConfigProblem): string {
  const [a, b, c, d] = segments(problem)
  const withProperty = (type: string, property?: string) =>
    property
      ? `${configureRoute.entityType(type)}?property=${encodeURIComponent(property)}`
      : configureRoute.entityType(type)
  if (problem.file === 'data-entry.yaml') {
    switch (a) {
      case 'forms':
        return b ? configureRoute.form(b) : configureRoute.forms()
      case 'lists':
        return b ? configureRoute.list(b) : configureRoute.lists()
      case 'kanbans':
        return b ? configureRoute.board(b) : configureRoute.boards()
      case 'dashboard':
        return configureRoute.dashboard()
      case 'styles':
        return b ? configureRoute.choiceList(b) : configureRoute.choiceLists()
      default:
        return configureRoute.navigation()
    }
  }
  switch (a) {
    case 'entities':
      if (b) return withProperty(b, c === 'properties' ? d : undefined)
      return configureRoute.entityTypes()
    case 'types':
      return b ? configureRoute.choiceList(b) : configureRoute.choiceLists()
    case 'relations':
      return b ? configureRoute.relation(b) : configureRoute.relations()
    case 'validations':
      return b !== undefined && /^\d+$/.test(b)
        ? configureRoute.rule(Number(b))
        : configureRoute.rules()
    case 'automations':
      return b !== undefined && /^\d+$/.test(b)
        ? configureRoute.automation(Number(b))
        : configureRoute.automations()
  }
  if (problem.entity_type) return withProperty(problem.entity_type, problem.property)
  return configureRoute.entityTypes()
}

/**
 * The problems that belong on a screen. A route with a query (a property on
 * an entity type) also counts for the screen without it.
 */
export function problemsFor(problems: ConfigProblem[], route: string): ConfigProblem[] {
  return problems.filter((p) => {
    const to = problemRoute(p)
    return to === route || to.split('?')[0] === route
  })
}
