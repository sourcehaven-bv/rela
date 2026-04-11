// File-system helper for the runner's isolated /tmp project copy.
//
// Workload operations call these to fire the rela-server file watcher
// "out of band" — i.e. without going through the API. This is what
// reproduces BUG-FMS1's hypothesised lock contention: a watcher event
// fires while a request is mid-flight on the same App.mu.

import * as fs from 'node:fs'
import * as path from 'node:path'

import type { ProjectFs, Rng } from './types.js'

export function makeProjectFs(root: string): ProjectFs {
  // Cache the list of entity files once at construction. The runner copies
  // the project into /tmp before starting, so the file set is stable for
  // the duration of the run (workloads create new files via the API; we
  // do not need to discover those for touching).
  const entityRoot = path.join(root, 'entities')
  const entityFiles: string[] = []
  walk(entityRoot, (p) => {
    if (p.endsWith('.md')) entityFiles.push(p)
  })
  if (entityFiles.length === 0) {
    throw new Error(`projectFs: no entity files found under ${entityRoot}`)
  }
  return {
    root,
    touchRandomEntity(rand: Rng) {
      const file = rand.pick(entityFiles)
      const now = new Date()
      // Use utimes rather than appending to avoid corrupting the file.
      fs.utimesSync(file, now, now)
      return file
    },
    rewriteRandomEntity(rand: Rng) {
      // Append a no-op trailing newline if not already present, otherwise
      // strip it. This produces a real Write event the watcher can debounce.
      const file = rand.pick(entityFiles)
      const buf = fs.readFileSync(file)
      if (buf.length > 0 && buf[buf.length - 1] === 0x0a) {
        fs.writeFileSync(file, buf.subarray(0, buf.length - 1))
      } else {
        fs.appendFileSync(file, '\n')
      }
      return file
    },
  }
}

function walk(dir: string, visit: (file: string) => void): void {
  let entries: fs.Dirent[]
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true })
  } catch {
    return
  }
  for (const entry of entries) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) walk(full, visit)
    else visit(full)
  }
}
