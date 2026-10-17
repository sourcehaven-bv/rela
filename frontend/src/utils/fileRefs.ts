/**
 * The display names of the files a `file`-type property value references.
 *
 * A stored entry is a storage path such as
 * `attachments/TASK-1/bijlagen/c54fb91819f593e9/report.pdf`, or a bare name
 * for a file stored before tokens existed. The display name is the last path
 * segment either way, which is also the `{fileName}` the download route takes.
 * This follows the server's `metamodel.FileRefs` for every value the
 * attachment service writes: backslashes count as separators, empty and dot
 * entries are dropped, and the names come back sorted, in the order the
 * entity page lists them. Two differences, both reachable only by hand-edited
 * values: duplicates are dropped by name (the server keeps two entries with
 * one name and different tokens), and `..` is not resolved (`path.Clean`).
 */
export function fileRefNames(value: unknown): string[] {
  const raw = Array.isArray(value) ? value : [value]
  const names = new Set<string>()
  for (const entry of raw) {
    if (typeof entry !== 'string' || entry === '') continue
    const segments = entry
      .replace(/\\/g, '/')
      .split('/')
      .filter((s) => s !== '' && s !== '.')
    const name = segments[segments.length - 1]
    if (name && name !== '..') names.add(name)
  }
  return [...names].sort((a, b) => (a < b ? -1 : a > b ? 1 : 0))
}
