import { test as base, Page, APIResponse } from '@playwright/test'
import {
  SearchPage,
  EntityDetailPage,
  createEntityDetailPage,
  FormPage,
  CreateTicketFormPage,
  createFormPage,
  ListPage,
  createListPage,
  KanbanPage,
  createKanbanPage,
  DashboardPage,
  GraphPage,
} from './page-objects'
import { spawn, ChildProcess, execSync } from 'child_process'
import * as net from 'net'
import * as os from 'os'
import * as path from 'path'
import * as fs from 'fs'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const FRONTEND_ROOT = path.resolve(__dirname, '..')
const PROJECT_ROOT = path.resolve(FRONTEND_ROOT, '..')
const DATA_ENTRY_PROJECT = path.resolve(PROJECT_ROOT, 'prototypes/data-entry/project')
const TEMP_PROJECT_PREFIX = 'rela-e2e-'

/**
 * Find a free port on localhost
 */
async function findFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = net.createServer()
    server.listen(0, '127.0.0.1', () => {
      const address = server.address()
      if (address && typeof address === 'object') {
        const port = address.port
        server.close(() => resolve(port))
      } else {
        reject(new Error('Could not get port'))
      }
    })
    server.on('error', reject)
  })
}

/**
 * Check if a server is responding
 */
async function isServerRunning(url: string): Promise<boolean> {
  try {
    const response = await fetch(url)
    return response.ok || response.status === 404
  } catch {
    return false
  }
}

/**
 * Recursively copy a directory
 */
function copyDirSync(src: string, dest: string): void {
  fs.mkdirSync(dest, { recursive: true })
  const entries = fs.readdirSync(src, { withFileTypes: true })
  for (const entry of entries) {
    const srcPath = path.join(src, entry.name)
    const destPath = path.join(dest, entry.name)
    if (entry.isDirectory()) {
      // Skip .rela directory (cache, etc)
      if (entry.name !== '.rela') {
        copyDirSync(srcPath, destPath)
      }
    } else {
      fs.copyFileSync(srcPath, destPath)
    }
  }
}

/**
 * Recursively remove a directory
 */
function rmDirSync(dir: string): void {
  if (!fs.existsSync(dir)) return
  const entries = fs.readdirSync(dir, { withFileTypes: true })
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      rmDirSync(fullPath)
    } else {
      fs.unlinkSync(fullPath)
    }
  }
  fs.rmdirSync(dir)
}

/**
 * Wait for a server to become available
 */
async function waitForServer(url: string, timeout = 30000): Promise<void> {
  const start = Date.now()
  while (Date.now() - start < timeout) {
    if (await isServerRunning(url)) {
      return
    }
    await new Promise((resolve) => setTimeout(resolve, 200))
  }
  throw new Error(`Server at ${url} did not start within ${timeout}ms`)
}

/**
 * Build the rela-server binary if it doesn't exist
 */
function ensureServerBinary(): string {
  const relaBinary = path.join(PROJECT_ROOT, 'bin/rela-server')
  if (!fs.existsSync(relaBinary)) {
    console.log('Building rela-server...')
    execSync('go build -o bin/rela-server ./cmd/rela-server', {
      cwd: PROJECT_ROOT,
      stdio: 'inherit',
    })
  }
  return relaBinary
}

/**
 * Shared API response types
 */
export interface EntityResponse {
  id: string
  type: string
  properties: Record<string, unknown>
  relations?: Record<string, string[]>
}

export interface PaginatedResponse {
  data: EntityResponse[]
  meta: {
    total: number
    page: number
    per_page: number
    has_more: boolean
  }
}

/**
 * API helper methods injected as a fixture, pre-bound to request and backend URL
 */
export interface ApiHelpers {
  createEntity(type: string, data: { properties: Record<string, unknown>; relations?: Record<string, string[]>; id?: string }): Promise<EntityResponse>
  getEntity(type: string, id: string): Promise<EntityResponse>
  updateEntity(type: string, id: string, properties: Record<string, unknown>): Promise<EntityResponse>
  deleteEntity(type: string, id: string): Promise<void>
  listEntities(type: string, query?: string): Promise<PaginatedResponse>
  getOrCreateCategory(name?: string): Promise<string>
}

/**
 * Backend server context for tests
 */
export interface BackendContext {
  port: number
  baseUrl: string
  projectPath: string
}

/**
 * Page object factories - creates page objects bound to apiPage
 */
export interface PageFactories {
  search(): SearchPage
  dashboard(): DashboardPage
  graph(): GraphPage
  entityDetail(type: string, id: string): EntityDetailPage
  list(listName: string): ListPage
  kanban(boardName: string): KanbanPage
  form(formName: string): FormPage
  createTicketForm(): CreateTicketFormPage
}

/**
 * Extended test fixtures
 */
export interface TestFixtures {
  backend: BackendContext
  apiPage: Page // Page with API routing configured
  api: ApiHelpers
  pages: PageFactories // Page object factories
}

/**
 * Worker-scoped fixtures (shared across tests in a worker)
 */
export interface WorkerFixtures {
  serverBinary: string
}

/**
 * Create extended test with backend fixtures
 */
export const test = base.extend<TestFixtures, WorkerFixtures>({
  // Worker-scoped: ensure binary is built once per worker
  serverBinary: [
    // eslint-disable-next-line no-empty-pattern
    async ({}, use) => {
      const binary = ensureServerBinary()
      await use(binary)
    },
    { scope: 'worker' },
  ],

  // Test-scoped: each test gets its own backend instance
  backend: async ({ serverBinary }, use) => {
    // Create temp project directory
    const tempDir = path.join(fs.realpathSync(os.tmpdir()), `${TEMP_PROJECT_PREFIX}${Date.now()}-${Math.random().toString(36).slice(2)}`)
    copyDirSync(DATA_ENTRY_PROJECT, tempDir)

    // Find a free port
    const port = await findFreePort()
    const baseUrl = `http://localhost:${port}`

    // Start the server
    const serverProcess: ChildProcess = spawn(serverBinary, ['-project', tempDir, '-port', String(port)], {
      cwd: PROJECT_ROOT,
      stdio: 'pipe',
    })

    // Log server output for debugging
    serverProcess.stdout?.on('data', (data) => {
      if (process.env.DEBUG) {
        console.log(`[backend:${port}] ${data.toString().trim()}`)
      }
    })
    serverProcess.stderr?.on('data', (data) => {
      if (process.env.DEBUG) {
        console.error(`[backend:${port}] ${data.toString().trim()}`)
      }
    })

    // Wait for server to be ready
    await waitForServer(`${baseUrl}/api/v1/_config`)

    // Provide backend context to test
    await use({
      port,
      baseUrl,
      projectPath: tempDir,
    })

    // Cleanup: kill server and remove temp directory
    serverProcess.kill('SIGTERM')
    rmDirSync(tempDir)
  },

  // API helpers pre-bound to request and backend URL
  api: async ({ request, backend }, use) => {
    async function call(method: string, path: string, data?: unknown): Promise<APIResponse> {
      const options: Record<string, unknown> = { method }
      if (data !== undefined) options.data = data
      const response = await request.fetch(`${backend.baseUrl}/api/v1/${path}`, options)
      if (!response.ok()) {
        throw new Error(`${method} /api/v1/${path} failed: ${response.status()} ${await response.text()}`)
      }
      return response
    }

    await use({
      async createEntity(type, data) {
        return (await call('POST', type, data)).json()
      },
      async getEntity(type, id) {
        return (await call('GET', `${type}/${id}`)).json()
      },
      async updateEntity(type, id, properties) {
        return (await call('PATCH', `${type}/${id}`, { properties })).json()
      },
      async deleteEntity(type, id) {
        try {
          await call('DELETE', `${type}/${id}`)
        } catch {
          // Ignore cleanup errors
        }
      },
      async listEntities(type, query) {
        const path = query ? `${type}?${query}` : type
        return (await call('GET', path)).json()
      },
      async getOrCreateCategory(name) {
        const result: PaginatedResponse = await (await call('GET', 'categories')).json()
        if (result.data.length > 0) return result.data[0].id
        const categoryId = `e2e-cat-${Date.now()}`
        const created: EntityResponse = await (await call('POST', 'categories', {
          id: categoryId,
          properties: { name: name ?? 'Test Category', description: 'Auto-created for e2e testing' },
        })).json()
        return created.id
      },
    })
  },

  // Convenience fixture: page with API routing already configured
  apiPage: async ({ page, backend }, use) => {
    // Route all /api/v1/* requests to the test's backend
    // Use a specific pattern to avoid matching source files like /src/api/*.ts
    // Route all /api/* requests to the test's backend
    await page.route(/\/api\//, async (route) => {
      const originalUrl = route.request().url()
      // Skip if it looks like a source file path
      if (originalUrl.includes('/src/api/')) {
        await route.continue()
        return
      }
      const url = new URL(originalUrl)
      url.host = `localhost:${backend.port}`
      await route.continue({ url: url.toString() })
    })

    await use(page)
  },

  // Page object factories - creates page objects bound to apiPage
  pages: async ({ apiPage }, use) => {
    await use({
      search: () => new SearchPage(apiPage),
      dashboard: () => new DashboardPage(apiPage),
      graph: () => new GraphPage(apiPage),
      entityDetail: (type: string, id: string) => createEntityDetailPage(apiPage, type, id),
      list: (listName: string) => createListPage(apiPage, listName),
      kanban: (boardName: string) => createKanbanPage(apiPage, boardName),
      form: (formName: string) => createFormPage(apiPage, formName),
      createTicketForm: () => new CreateTicketFormPage(apiPage),
    })
  },
})

export { expect } from '@playwright/test'
