import { ESLint } from 'eslint'
import { readFileSync, writeFileSync } from 'node:fs'
import { relative, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('..', import.meta.url))
const baselinePath = new URL('../comment-body-baseline.json', import.meta.url)
const update = process.argv.includes('--update')

const eslint = new ESLint({ cwd: root, overrideConfigFile: 'eslint.comments.config.js' })
const results = await eslint.lintFiles(['.'])

const found = {}
const problems = []
for (const result of results) {
  if (result.fatalErrorCount > 0) {
    problems.push(`${relative(root, result.filePath)}: could not be parsed`)
  }
  const messages = result.messages.filter((m) => m.ruleId === 'rela/no-body-comments')
  if (messages.length > 0) found[relative(root, result.filePath).split(sep).join('/')] = messages
}
const counts = Object.fromEntries(
  Object.entries(found)
    .map(([file, messages]) => [file, messages.length])
    .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
)

if (update) {
  writeFileSync(baselinePath, JSON.stringify(counts, null, 2) + '\n')
  process.exit(0)
}

const baseline = JSON.parse(readFileSync(baselinePath, 'utf8'))
for (const [file, count] of Object.entries(counts)) {
  const allowed = baseline[file] ?? 0
  if (count > allowed) {
    problems.push(`${file}: ${count} findings, baseline allows ${allowed}`)
    for (const m of found[file]) problems.push(`  ${file}:${m.line}: ${m.message}`)
  }
}
for (const [file, allowed] of Object.entries(baseline)) {
  if ((counts[file] ?? 0) < allowed) {
    console.error(
      `note: ${file} has ${counts[file] ?? 0} findings, baseline says ${allowed}; \`npm run lint:comments:update\` locks in the gain`
    )
  }
}
if (problems.length > 0) {
  console.error(problems.join('\n'))
  process.exit(1)
}
