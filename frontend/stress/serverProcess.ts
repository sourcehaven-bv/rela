// Spin up a fresh rela-server against an isolated /tmp project copy.
//
// We deliberately avoid reusing the e2e fixtures.ts setup because:
//   - the e2e fixture is per-Playwright-test scoped and we want one server
//     for the whole stress run, shared across all browser users
//   - the e2e fixture doesn't enable pprof
//   - the stress runner uses raw fetch + Playwright BrowserContexts
//     instead of the test framework's `page` fixture
//
// The cost is a bit of duplicated logic for project copying / port
// allocation. Acceptable for a diagnostic tool.

import { spawn, ChildProcess, execSync } from 'node:child_process'
import * as fs from 'node:fs'
import * as net from 'node:net'
import * as os from 'node:os'
import * as path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const FRONTEND_ROOT = path.resolve(__dirname, '..')
const REPO_ROOT = path.resolve(FRONTEND_ROOT, '..')

export interface ServerHandle {
  baseUrl: string
  port: number
  pprofUrl: string
  projectRoot: string
  stop(): Promise<void>
}

export interface StartServerOptions {
  sourceProject: string
  reportDir: string
  enablePprof: boolean
  /** Pre-built binary to use. Built on demand if missing. */
  binary?: string
}

export async function startServer(opts: StartServerOptions): Promise<ServerHandle> {
  const binary = opts.binary ?? ensureBinary()
  const projectRoot = await prepareTempProject(opts.sourceProject)
  const port = await findFreePort()
  const pprofPort = opts.enablePprof ? await findFreePort() : 0

  const args = ['-port', String(port), '-project', projectRoot]
  if (opts.enablePprof) {
    args.push('-debug-pprof', `127.0.0.1:${pprofPort}`)
  }

  const logFile = path.join(opts.reportDir, 'rela-server.log')
  fs.mkdirSync(opts.reportDir, { recursive: true })
  const logStream = fs.createWriteStream(logFile)

  const child: ChildProcess = spawn(binary, args, {
    stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env },
  })
  child.stdout?.pipe(logStream)
  child.stderr?.pipe(logStream)

  const baseUrl = `http://127.0.0.1:${port}`
  await waitForServer(`${baseUrl}/`)

  return {
    baseUrl,
    port,
    pprofUrl: opts.enablePprof ? `http://127.0.0.1:${pprofPort}` : '',
    projectRoot,
    async stop() {
      child.kill('SIGTERM')
      await new Promise<void>((resolve) => {
        child.once('exit', () => resolve())
        // Hard kill after 3s so a hung server can't block teardown.
        setTimeout(() => {
          try {
            child.kill('SIGKILL')
          } catch {
            /* already dead */
          }
          resolve()
        }, 3000)
      })
      logStream.end()
    },
  }
}

function ensureBinary(): string {
  const bin = path.join(REPO_ROOT, 'bin/rela-server')
  if (!fs.existsSync(bin)) {
    console.log('[stress] building rela-server...')
    execSync('go build -o bin/rela-server ./cmd/rela-server', {
      cwd: REPO_ROOT,
      stdio: 'inherit',
    })
  }
  return bin
}

async function prepareTempProject(source: string): Promise<string> {
  const abs = path.isAbsolute(source) ? source : path.resolve(REPO_ROOT, source)
  if (!fs.existsSync(abs)) {
    throw new Error(`source project does not exist: ${abs}`)
  }
  const dest = fs.mkdtempSync(path.join(os.tmpdir(), 'rela-stress-project-'))
  copyDir(abs, dest)
  return dest
}

function copyDir(src: string, dest: string): void {
  fs.mkdirSync(dest, { recursive: true })
  for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
    if (entry.name === '.rela') continue // skip cache
    const s = path.join(src, entry.name)
    const d = path.join(dest, entry.name)
    if (entry.isDirectory()) copyDir(s, d)
    else fs.copyFileSync(s, d)
  }
}

async function findFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = net.createServer()
    server.listen(0, '127.0.0.1', () => {
      const addr = server.address()
      if (addr && typeof addr === 'object') {
        const p = addr.port
        server.close(() => resolve(p))
      } else {
        reject(new Error('could not allocate port'))
      }
    })
    server.on('error', reject)
  })
}

async function waitForServer(url: string, timeoutMs = 30000): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    try {
      const r = await fetch(url)
      if (r.ok || r.status === 404) return
    } catch {
      /* not yet up */
    }
    await new Promise((r) => setTimeout(r, 100))
  }
  throw new Error(`server at ${url} did not start within ${timeoutMs}ms`)
}
