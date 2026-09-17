/// <reference types="node" />
// Runs in Node (vitest) and needs node:fs / node:url. The project tsconfig
// scopes `types` to vitest/globals, so pull in node types file-locally rather
// than widening them project-wide (same pattern as markdownContentMirror).
//
// Guards the entity-ID grammar across the language boundary.
//
// `isValidEntityRefId` decides whether a code span becomes a rendered
// reference; `entity.ValidateID` decides whether the store will accept that id
// at all. If the editor is looser, it renders links to entities that cannot
// exist. If it is stricter, real references silently stay plain code spans.
//
// The Go half reads the SAME fixture (internal/entity/id_grammar_fixture_test.go),
// so changing one grammar without the other fails a test instead of drifting.
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, it, expect } from 'vitest'
import { isValidEntityRefId } from './entityRefNode'

interface Fixture {
  valid: string[]
  invalid: string[]
}

// Resolved from the vitest cwd (frontend/) rather than import.meta.url, which
// is not a file URL under this config.
const FIXTURE = resolve(
  process.cwd(),
  'src/components/forms/milkdown/testdata/entity-id-grammar.json'
)

const fixture: Fixture = JSON.parse(readFileSync(FIXTURE, 'utf8'))

describe('entity id grammar fixture', () => {
  it('is non-trivial (guards a broken fixture path)', () => {
    expect(fixture.valid.length).toBeGreaterThan(5)
    expect(fixture.invalid.length).toBeGreaterThan(10)
  })

  it.each(fixture.valid)('accepts %j', (id) => {
    expect(isValidEntityRefId(id)).toBe(true)
  })

  it.each(fixture.invalid)('rejects %j', (id) => {
    expect(isValidEntityRefId(id)).toBe(false)
  })
})
