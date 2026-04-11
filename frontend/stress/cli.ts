// CLI entrypoint for the stress runner. Run via:
//   npm run stress -- --scenario=watcher-pressure --duration=30s
//
// Argument parsing is intentionally minimal: this is a developer tool, not
// a production CLI. Unknown flags fail loudly so typos don't silently
// fall back to defaults.

import * as fs from 'node:fs'
import * as path from 'node:path'

import { run, RunOptions } from './runner.js'
import { runFuzz } from './fuzzRunner.js'
import { scenarios } from './scenarios/index.js'

interface CliArgs {
  mode: 'stress' | 'fuzz'
  scenario: string
  durationMs: number
  users?: number
  sourceProject: string
  reportDir: string
  seed: number
  browser: 'chromium' | 'firefox' | 'webkit'
  // Fuzz-only:
  numRuns: number
  maxActions: number
  oracleNavTimeoutMs: number
  actionLatencyBudgetMs: number
  mutate: boolean
}

function parseArgs(argv: string[]): CliArgs {
  const args: Record<string, string> = {}
  for (const raw of argv) {
    const m = raw.match(/^--([^=]+)(?:=(.*))?$/)
    if (!m) throw new Error(`unrecognised argument: ${raw}`)
    args[m[1]!] = m[2] ?? 'true'
  }
  const known = new Set([
    'mode',
    'scenario',
    'duration',
    'users',
    'source-project',
    'report-dir',
    'seed',
    'browser',
    'num-runs',
    'max-actions',
    'oracle-timeout',
    'action-budget',
    'mutate',
  ])
  for (const k of Object.keys(args)) {
    if (!known.has(k)) throw new Error(`unknown flag --${k}`)
  }
  const mode = (args.mode ?? 'stress') as 'stress' | 'fuzz'
  if (mode !== 'stress' && mode !== 'fuzz') {
    throw new Error(`unknown mode: ${mode} (expected stress|fuzz)`)
  }
  const scenarioName = args.scenario ?? 'watcher-pressure'
  if (mode === 'stress' && !scenarios[scenarioName]) {
    throw new Error(
      `unknown scenario "${scenarioName}". Known: ${Object.keys(scenarios).join(', ')}`,
    )
  }
  const durationMs =
    mode === 'stress'
      ? args.duration
        ? parseDuration(args.duration)
        : scenarios[scenarioName]!.defaultDurationMs
      : 0
  const users = args.users ? parseInt(args.users, 10) : undefined
  const sourceProject = args['source-project'] ?? 'tickets'
  const reportDir =
    args['report-dir'] ?? path.join('/tmp', `rela-${mode}-${Date.now()}`)
  const seed = args.seed ? parseInt(args.seed, 10) : Math.floor(Math.random() * 2 ** 31)
  const browserName = (args.browser ?? 'chromium') as 'chromium' | 'firefox' | 'webkit'
  if (!['chromium', 'firefox', 'webkit'].includes(browserName)) {
    throw new Error(`unknown browser: ${browserName} (expected chromium|firefox|webkit)`)
  }
  const numRuns = args['num-runs'] ? parseInt(args['num-runs'], 10) : 200
  const maxActions = args['max-actions'] ? parseInt(args['max-actions'], 10) : 15
  // Default oracle timeout bumped to 10s because the SPA now reloads
  // the page on Vite chunk-load failures (see main.ts and router/index.ts).
  // A reload takes the full initial-load time (~2s on Firefox) plus the
  // pre-reload failed attempt, so 5s is too tight for sequences that
  // trigger a chunk-load race. 10s is still well under the "long long
  // time hang" threshold the original user-reported symptom described.
  const oracleNavTimeoutMs = args['oracle-timeout']
    ? parseDuration(args['oracle-timeout'])
    : 10_000
  const actionLatencyBudgetMs = args['action-budget']
    ? parseDuration(args['action-budget'])
    : 3_000
  // --mutate is a flag: `--mutate` or `--mutate=true` enables it.
  // Default off because mutations expose state-dependent bugs that
  // build up across iterations and are harder to make deterministic.
  const mutate = args.mutate !== undefined && args.mutate !== 'false'
  return {
    mode,
    scenario: scenarioName,
    durationMs,
    users,
    sourceProject,
    reportDir,
    seed,
    browser: browserName,
    numRuns,
    maxActions,
    oracleNavTimeoutMs,
    actionLatencyBudgetMs,
    mutate,
  }
}

function parseDuration(s: string): number {
  const m = s.match(/^(\d+)(ms|s|m)?$/)
  if (!m) throw new Error(`invalid duration: ${s}`)
  const n = parseInt(m[1]!, 10)
  switch (m[2] ?? 's') {
    case 'ms':
      return n
    case 's':
      return n * 1000
    case 'm':
      return n * 60_000
    default:
      throw new Error(`invalid duration unit: ${m[2]}`)
  }
}

async function main(): Promise<void> {
  const args = parseArgs(process.argv.slice(2))
  fs.mkdirSync(args.reportDir, { recursive: true })

  if (args.mode === 'fuzz') {
    if (args.browser === 'webkit') {
      throw new Error('fuzz mode supports only chromium or firefox')
    }
    console.log(
      `[fuzz] browser=${args.browser} project=${args.sourceProject} ` +
        `seed=${args.seed} numRuns=${args.numRuns} maxActions=${args.maxActions} ` +
        `oracleTimeout=${args.oracleNavTimeoutMs}ms ` +
        `actionBudget=${args.actionLatencyBudgetMs}ms mutate=${args.mutate}`,
    )
    console.log(`[fuzz] report dir: ${args.reportDir}`)

    const result = await runFuzz({
      sourceProject: args.sourceProject,
      reportDir: args.reportDir,
      browser: args.browser,
      numRuns: args.numRuns,
      maxActions: args.maxActions,
      oracleNavTimeoutMs: args.oracleNavTimeoutMs,
      actionLatencyBudgetMs: args.actionLatencyBudgetMs,
      mutate: args.mutate,
      seed: args.seed,
    })
    console.log('\n=== FUZZ RESULT ===')
    console.log(JSON.stringify(result, null, 2))
    if (!result.passed) {
      console.error(`\n[fuzz] FAILED — see report dir for full log + pprof captures`)
      process.exit(1)
    }
    console.log('\n[fuzz] PASS')
    return
  }

  const scenario = scenarios[args.scenario]!
  const opts: RunOptions = {
    scenario,
    durationMs: args.durationMs,
    users: args.users ?? scenario.users,
    sourceProject: args.sourceProject,
    reportDir: args.reportDir,
    seed: args.seed,
    browser: args.browser,
  }

  console.log(
    `[stress] scenario=${opts.scenario.name} duration=${opts.durationMs}ms ` +
      `users=${opts.users} browser=${opts.browser} project=${opts.sourceProject} seed=${opts.seed}`,
  )
  console.log(`[stress] report dir: ${opts.reportDir}`)

  const report = await run(opts)
  console.log('\n=== STRESS REPORT ===')
  console.log(JSON.stringify(report, null, 2))

  if (report.breaches.length > 0) {
    console.error(`\n[stress] FAILED — ${report.breaches.length} invariant(s) breached`)
    for (const b of report.breaches) {
      console.error(`  - ${b}`)
    }
    process.exit(1)
  }
  console.log('\n[stress] PASS')
}

main().catch((err) => {
  console.error('[stress] fatal:', err)
  process.exit(2)
})
