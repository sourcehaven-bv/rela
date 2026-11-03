// Starts one rela-server on the memory store for the contract tests.
//
// The binary comes from RELA_CONTRACT_SERVER, else it is built here into
// bin/rela-server-memory, every run, so a run never tests an old server (go
// build only redoes what changed). Building needs the SPA embedded
// (`just build-frontend`), because rela-server refuses to start without it.
// The fixture project is copied to a temp dir, so the committed copy is never
// written to.

import { execFileSync, spawn, type ChildProcess } from 'node:child_process'
import * as fs from 'node:fs'
import * as net from 'node:net'
import * as os from 'node:os'
import * as path from 'node:path'
import { fileURLToPath } from 'node:url'

import type { TestProject } from 'vitest/node'

declare module 'vitest' {
  export interface ProvidedContext {
    contractServerUrl: string
  }
}

const here = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.resolve(here, '../..')
const fixture = path.join(here, 'fixture/project')

let proc: ChildProcess | undefined
let projectDir: string | undefined

export default async function setup(project: TestProject) {
  const binary = serverBinary()
  projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rela-contract-'))
  fs.cpSync(fixture, projectDir, { recursive: true })
  const port = await freePort()
  const url = `http://127.0.0.1:${port}`

  const log: string[] = []
  proc = spawn(binary, ['-port', String(port), '-project', projectDir, '-allowed-origin', url], {
    cwd: projectDir,
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  proc.stdout?.on('data', (b: Buffer) => log.push(b.toString()))
  proc.stderr?.on('data', (b: Buffer) => log.push(b.toString()))

  await waitReady(url, log)
  project.provide('contractServerUrl', url)

  return () => {
    proc?.kill()
    if (projectDir) fs.rmSync(projectDir, { recursive: true, force: true })
  }
}

function serverBinary(): string {
  const fromEnv = process.env.RELA_CONTRACT_SERVER
  // Resolved here: the server runs in the temp project dir.
  if (fromEnv) return path.resolve(fromEnv)
  const bin = path.join(repoRoot, 'bin/rela-server-memory')
  if (!fs.existsSync(path.join(repoRoot, 'internal/dataentry/static/v2/index.html'))) {
    throw new Error('contract: rela-server needs the SPA embedded; run `just build-frontend` first')
  }
  execFileSync('go', ['build', '-tags', 'memorybackend', '-o', bin, './cmd/rela-server'], {
    cwd: repoRoot,
    stdio: 'inherit',
  })
  return bin
}

function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = net.createServer()
    srv.once('error', reject)
    srv.listen(0, '127.0.0.1', () => {
      const { port } = srv.address() as net.AddressInfo
      srv.close(() => resolve(port))
    })
  })
}

async function waitReady(url: string, log: string[]): Promise<void> {
  const deadline = Date.now() + 30_000
  while (Date.now() < deadline) {
    if (proc?.exitCode !== null && proc?.exitCode !== undefined) break
    try {
      const res = await fetch(`${url}/api/v1/_schema`, { headers: { Origin: url } })
      if (res.ok) return
    } catch {
      // not listening yet
    }
    await new Promise((r) => setTimeout(r, 100))
  }
  proc?.kill()
  throw new Error(`contract: rela-server did not start\n${log.join('')}`)
}
