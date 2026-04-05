#!/usr/bin/env node
/* global process */

/**
 * Generate or check coverage baseline for frontend tests.
 * Similar to go-test-coverage ratchet mechanism.
 *
 * Usage:
 *   node scripts/coverage-baseline.js generate  # Generate new baseline
 *   node scripts/coverage-baseline.js check     # Check coverage against baseline
 */

import { readFileSync, writeFileSync, existsSync } from 'fs'
import { resolve, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const COVERAGE_JSON = resolve(__dirname, '../coverage/coverage-final.json')
const BASELINE_FILE = resolve(__dirname, '../.coverage-baseline')

function parseCoverage() {
  if (!existsSync(COVERAGE_JSON)) {
    console.error('Coverage JSON not found. Run `npm run test:coverage` first.')
    process.exit(1)
  }

  const coverage = JSON.parse(readFileSync(COVERAGE_JSON, 'utf8'))
  const results = []

  for (const [filePath, data] of Object.entries(coverage)) {
    // Convert absolute path to relative
    const relativePath = filePath.replace(/.*\/frontend\//, '')

    // Skip files outside src/
    if (!relativePath.startsWith('src/')) continue

    // Calculate statement coverage
    const statements = Object.values(data.s || {})
    const total = statements.length
    const covered = statements.filter((count) => count > 0).length

    if (total > 0) {
      results.push({ file: relativePath, total, covered })
    }
  }

  // Sort by file path for consistent output
  results.sort((a, b) => a.file.localeCompare(b.file))
  return results
}

function generateBaseline() {
  const results = parseCoverage()
  const lines = results.map((r) => `${r.file};${r.total};${r.covered}`)
  writeFileSync(BASELINE_FILE, lines.join('\n') + '\n')
  console.log(`Generated baseline with ${results.length} files`)

  // Print summary
  const totalStmts = results.reduce((sum, r) => sum + r.total, 0)
  const coveredStmts = results.reduce((sum, r) => sum + r.covered, 0)
  const pct = totalStmts > 0 ? ((coveredStmts / totalStmts) * 100).toFixed(1) : 0
  console.log(`Overall: ${coveredStmts}/${totalStmts} statements (${pct}%)`)
}

function checkBaseline() {
  if (!existsSync(BASELINE_FILE)) {
    console.error('Baseline file not found. Run with "generate" first.')
    process.exit(1)
  }

  const baseline = new Map()
  const baselineContent = readFileSync(BASELINE_FILE, 'utf8')
  for (const line of baselineContent.trim().split('\n')) {
    if (!line) continue
    const [file, total, covered] = line.split(';')
    baseline.set(file, { total: parseInt(total), covered: parseInt(covered) })
  }

  const current = parseCoverage()
  const violations = []

  for (const { file, total, covered } of current) {
    const base = baseline.get(file)
    if (!base) continue // New file, OK

    // Coverage must not decrease (covered lines must not go down)
    if (covered < base.covered) {
      const basePct = ((base.covered / base.total) * 100).toFixed(1)
      const currPct = ((covered / total) * 100).toFixed(1)
      violations.push(
        `  ${file}: ${base.covered}/${base.total} (${basePct}%) -> ${covered}/${total} (${currPct}%)`
      )
    }
  }

  // Check for files removed from coverage that still exist in baseline
  for (const [file, _base] of baseline) {
    if (!current.find((c) => c.file === file)) {
      // File removed from coverage - only warn, don't fail
      // (file might have been deleted or moved)
      console.warn(`Warning: ${file} in baseline but not in coverage`)
    }
  }

  if (violations.length > 0) {
    console.error('')
    console.error(`ERROR: Coverage decreased for ${violations.length} file(s)!`)
    console.error('')
    console.error('The .coverage-baseline must not be manually lowered.')
    console.error('It is automatically updated after merging.')
    console.error('')
    console.error('Violations:')
    violations.forEach((v) => console.error(v))
    console.error('')
    console.error('To fix: add tests to improve coverage.')
    process.exit(1)
  }

  console.log('Coverage baseline check passed - no coverage decreased.')
}

const command = process.argv[2]
if (command === 'generate') {
  generateBaseline()
} else if (command === 'check') {
  checkBaseline()
} else {
  console.error('Usage: coverage-baseline.js <generate|check>')
  process.exit(1)
}
