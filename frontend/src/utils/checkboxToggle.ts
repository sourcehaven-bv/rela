// Markdown task-list parsing without regex. Both the toggler and the stats
// counter (getCheckboxStats in markdown.ts) route through parseCheckboxLine
// so they cannot disagree on which lines count as checkboxes — drift between
// the two manifests as the (n/m) widget showing one number while the click
// hits a different line.
//
// CONTRACT: parseCheckboxLine MUST accept exactly the set of lines that
// marked's task-list extension emits as `<input type="checkbox">`. Verified
// against marked v17:
//   - leading whitespace allowed (indented sub-lists)
//   - bullet is `-`, `*`, `+`, or `N.` (any positive integer)
//   - bullet followed by exactly one space
//   - then `[`, a state character (space, `x`, or `X`), `]`
//   - then exactly one space before the item text (`- [ ]nospace` is NOT a
//     task item per GFM)

export interface CheckboxLine {
  /** Index into the original line where `[` appears. */
  bracketPos: number
  /** Index into the original line where the state char (space/x/X) appears. */
  charPos: number
  /** True if the state character is `x` or `X`. */
  checked: boolean
}

/**
 * Parse a single line. Returns null if the line is not a markdown task-list
 * item. The returned offsets index into the *original* line (not the
 * trimmed view), so callers can mutate the source without losing indentation.
 */
export function parseCheckboxLine(line: string): CheckboxLine | null {
  // Skip leading whitespace.
  let i = 0
  while (i < line.length && (line[i] === ' ' || line[i] === '\t')) i++
  const bulletStart = i

  // Bullet: `-`, `*`, `+`, or one-or-more digits followed by `.`.
  if (line[i] === '-' || line[i] === '*' || line[i] === '+') {
    i++
  } else if (line[i] >= '0' && line[i] <= '9') {
    while (i < line.length && line[i] >= '0' && line[i] <= '9') i++
    if (line[i] !== '.') return null
    i++
  } else {
    return null
  }
  if (bulletStart === i) return null

  // Exactly one space after the bullet.
  if (line[i] !== ' ') return null
  i++

  // Opening bracket.
  if (line[i] !== '[') return null
  const bracketPos = i
  i++

  // State character.
  const stateCh = line[i]
  if (stateCh !== ' ' && stateCh !== 'x' && stateCh !== 'X') return null
  const charPos = i
  i++

  // Closing bracket + space.
  if (line[i] !== ']') return null
  i++
  if (line[i] !== ' ') return null

  return { bracketPos, charPos, checked: stateCh !== ' ' }
}

/**
 * Flip the checkbox at the given 0-based index in a markdown string.
 * Throws if the index is out of range or no checkbox is found.
 *
 * Mirrors what `internal/dataentry/helpers.go:toggleCheckbox` used to do,
 * before the toggle path moved out of the legacy `/api/toggle-checkbox`
 * endpoint and into the PATCH-based reactive flow in EntityDetail.vue.
 */
export function toggleCheckboxInSource(content: string, index: number): string {
  const lines = content.split('\n')
  let cbIdx = 0
  for (let i = 0; i < lines.length; i++) {
    const parsed = parseCheckboxLine(lines[i])
    if (!parsed) continue
    if (cbIdx !== index) {
      cbIdx++
      continue
    }
    const line = lines[i]
    const next = parsed.checked ? ' ' : 'x'
    lines[i] = line.slice(0, parsed.charPos) + next + line.slice(parsed.charPos + 1)
    return lines.join('\n')
  }
  throw new Error(`checkbox index ${index} out of range (found ${cbIdx})`)
}

/**
 * Count checked and total markdown task-list items. Returns null when the
 * content has no task-list items at all (the caller hides the stats widget
 * when there's nothing to count).
 */
export function checkboxStats(content: string): { checked: number; total: number } | null {
  if (!content) return null
  let total = 0
  let checked = 0
  for (const line of content.split('\n')) {
    const parsed = parseCheckboxLine(line)
    if (!parsed) continue
    total++
    if (parsed.checked) checked++
  }
  if (total === 0) return null
  return { checked, total }
}
