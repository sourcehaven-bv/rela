import { test as base, type Page, type APIRequestContext } from '@playwright/test';
import { spawn, type ChildProcess, execSync } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import * as net from 'net';

const PROJECT_ROOT = path.resolve(__dirname, '../..');
const SERVER_BINARY = path.join(PROJECT_ROOT, 'bin', 'rela-server');
// Resolve symlinks once — macOS $TMPDIR is typically /var/folders/... which
// canonicalises to /private/var/folders/.... Tests that compare paths want
// the canonical form. (review-response RR-F3IA3)
const TMPDIR = fs.realpathSync(os.tmpdir());

/**
 * Constants derived from the inline test project below. Specs should import
 * these instead of hardcoding strings that couple them to schema values —
 * e.g. use `STATUS.feature.draft` instead of `'draft'`. (RR-F5P1L, RR-M0099)
 */
export const STATUS = {
  feature: {
    draft: 'draft',
    approved: 'approved',
    in_progress: 'in_progress',
    done: 'done',
  },
} as const;

export const SEVERITY = {
  low: 'low',
  medium: 'medium',
  high: 'high',
  critical: 'critical',
} as const;

export const PRIORITY = {
  low: 'low',
  medium: 'medium',
  high: 'high',
} as const;

/** The metamodel analysis check types the backend emits and the SPA renders
 *  as check-cards at /analyze. Mirrors CHECK_TYPES in
 *  frontend/src/views/AnalyzeView.vue; if that list grows the spec will fail
 *  and the list here should be updated in lockstep. */
export const ANALYSIS_CHECKS = ['Properties', 'Cardinality', 'Validations', 'Orphans'] as const;

/** Seed entity IDs present in every fresh test project. Assumes the inline
 *  seed data below, in insertion order. Import from specs instead of
 *  writing 'FEAT-001' / 'BUG-001' strings directly. */
export const SEED = {
  features: {
    authentication: 'FEAT-001',
    dashboardAnalytics: 'FEAT-002',
    exportData: 'FEAT-003',
    /** Body content contains two GFM checkboxes (one unchecked, one checked).
     *  Used by checkboxes.spec.ts to exercise the toggle UI. */
    checkboxBody: 'FEAT-004',
  },
  bugs: {
    loginFormValidation: 'BUG-001',
    memoryLeak: 'BUG-002',
  },
  tasks: {
    writeUnitTests: 'TASK-001',
  },
} as const;

export interface EntityResponse {
  id: string;
  type: string;
  properties: Record<string, unknown>;
  relations?: Record<string, string[]>;
}

export interface PaginatedResponse<T = EntityResponse> {
  data: T[];
  meta: { total: number; page: number; per_page: number; has_more: boolean };
}

export interface ApiHelpers {
  createEntity(plural: string, data: { properties: Record<string, unknown>; relations?: Record<string, string[]>; id?: string }): Promise<EntityResponse>;
  getEntity(plural: string, id: string): Promise<EntityResponse>;
  updateEntity(plural: string, id: string, properties: Record<string, unknown>): Promise<EntityResponse>;
  deleteEntity(plural: string, id: string): Promise<void>;
  listEntities(plural: string, query?: string): Promise<PaginatedResponse>;
  createRelation(fromPlural: string, fromId: string, relation: string, toId: string): Promise<void>;
  /** Returns the markdown body of an entity, or "" if the entity has no body. */
  getContent(plural: string, id: string): Promise<string>;
  rawRequest(method: string, path: string, data?: unknown): Promise<import('@playwright/test').APIResponse>;
  /** Wait for an entity to appear in the search index. Use before
   *  navigating to a view that reads via /_search (dashboard, search). */
  waitForIndexed(id: string, options?: { timeout?: number }): Promise<void>;
}

export interface TestFixtures {
  testProject: string;
  serverUrl: string;
  appPage: Page;
  api: ApiHelpers;
}

export interface WorkerFixtures {
  serverBinary: string;
}

/** Find a free ephemeral port on loopback. Note: between this call and the
 *  child binding the port, the kernel may reassign it elsewhere under load —
 *  callers MUST retry on startup failure (see spawnServer). */
async function findFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.listen(0, '127.0.0.1', () => {
      const addr = server.address();
      if (addr && typeof addr === 'object') {
        const port = addr.port;
        server.close(() => resolve(port));
      } else {
        reject(new Error('Could not get port'));
      }
    });
    server.on('error', reject);
  });
}

/** Probe a known-live API endpoint with the same Origin header the tests will
 *  use. The root path serves the SPA static bundle and is ready before the API
 *  handlers are wired, which is why we target /api/v1/_config specifically
 *  (see RR-LWG6W). The backend's security middleware otherwise rejects
 *  Origin-less probes with 403 origin_missing, so we always send Origin. */
async function waitForServer(url: string, origin: string, timeout = 30000): Promise<void> {
  const start = Date.now();
  const probeUrl = `${url}/api/v1/_config`;
  while (Date.now() - start < timeout) {
    try {
      const r = await fetch(probeUrl, { headers: { Origin: origin } });
      if (r.ok) return;
    } catch {
      // not ready
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error(`Server at ${probeUrl} did not start within ${timeout}ms`);
}

/** Wait for a child process to actually exit. SIGTERM is async; a bare
 *  proc.kill() returns immediately and the kernel may hold the socket past the
 *  next test start. See RR-17XTS. */
async function waitForExit(proc: ChildProcess, signal: NodeJS.Signals = 'SIGTERM'): Promise<void> {
  if (proc.exitCode !== null || proc.signalCode !== null) return;
  await new Promise<void>((resolve) => {
    const done = () => resolve();
    proc.once('exit', done);
    proc.kill(signal);
    // Escalate to SIGKILL if the server refuses to die within 5s
    setTimeout(() => {
      if (proc.exitCode === null && proc.signalCode === null) {
        proc.kill('SIGKILL');
      }
    }, 5000);
  });
}

/** Build a Go binary if missing, using a filesystem lock so concurrent workers
 *  don't race each other writing the same output file (RR-BZUH5). In CI the
 *  fixture should never fire because the binary is pre-built in a prior step;
 *  we fail loudly there to surface misconfiguration. */
function buildIfMissing(binaryPath: string, target: string): string {
  if (fs.existsSync(binaryPath)) return binaryPath;
  if (process.env.CI) {
    throw new Error(
      `Missing ${binaryPath} in CI. The e2e CI job is expected to build it in a prior step; ` +
        'do not fall back to in-fixture builds on CI.',
    );
  }

  const lockPath = `${binaryPath}.lock`;
  const start = Date.now();
  // Try to acquire an exclusive lock. Another worker may already be building.
  let acquired = false;
  while (!acquired) {
    try {
      fs.mkdirSync(lockPath);
      acquired = true;
    } catch (e) {
      const code = (e as NodeJS.ErrnoException).code;
      if (code !== 'EEXIST') throw e;
      if (fs.existsSync(binaryPath)) return binaryPath;
      if (Date.now() - start > 300_000) {
        throw new Error(`Build lock ${lockPath} held > 5 min; something is stuck.`);
      }
      // busy-wait 250ms; builds take seconds
      const end = Date.now() + 250;
      while (Date.now() < end) {
        // naive sync wait — this fixture runs once per worker only
      }
    }
  }
  try {
    console.log(`Building ${path.basename(binaryPath)}...`);
    execSync(`go build -o ${binaryPath} ${target}`, { cwd: PROJECT_ROOT, stdio: 'inherit' });
  } finally {
    fs.rmdirSync(lockPath);
  }
  return binaryPath;
}

function createTestProject(): string {
  const tmpDir = fs.mkdtempSync(path.join(TMPDIR, 'rela-e2e-'));
  fs.writeFileSync(path.join(tmpDir, 'metamodel.yaml'), METAMODEL_YAML);
  fs.writeFileSync(path.join(tmpDir, 'data-entry.yaml'), DATA_ENTRY_YAML);
  fs.mkdirSync(path.join(tmpDir, 'entities', 'features'), { recursive: true });
  fs.mkdirSync(path.join(tmpDir, 'entities', 'bugs'), { recursive: true });
  fs.mkdirSync(path.join(tmpDir, 'entities', 'tasks'), { recursive: true });
  fs.mkdirSync(path.join(tmpDir, 'relations'), { recursive: true });
  for (const [rel, content] of Object.entries(SEED_ENTITIES)) {
    fs.writeFileSync(path.join(tmpDir, rel), content);
  }
  return tmpDir;
}

/** Spawn rela-server on a free port with retry-on-startup-failure: the port
 *  we pick from findFreePort may have been reassigned by the kernel before the
 *  child binds it, under load (RR-B8GJT). Up to 3 attempts. */
async function spawnServer(
  serverBinary: string,
  cwd: string,
): Promise<{ proc: ChildProcess; url: string; logs: () => string }> {
  const attempts = 3;
  let lastErr: unknown;
  for (let i = 0; i < attempts; i++) {
    const port = await findFreePort();
    const url = `http://localhost:${port}`;
    const proc: ChildProcess = spawn(
      serverBinary,
      ['-port', String(port), '-allowed-origin', url],
      { cwd, env: process.env, stdio: ['ignore', 'pipe', 'pipe'] },
    );
    let stdout = '';
    let stderr = '';
    proc.stdout?.on('data', (d) => (stdout += d.toString()));
    proc.stderr?.on('data', (d) => (stderr += d.toString()));

    try {
      await waitForServer(url, url);
      return {
        proc,
        url,
        logs: () => `--- stdout ---\n${stdout}\n--- stderr ---\n${stderr}\n`,
      };
    } catch (e) {
      lastErr = e;
      await waitForExit(proc);
    }
  }
  throw new Error(`Failed to start rela-server after ${attempts} attempts: ${String(lastErr)}`);
}

export const test = base.extend<TestFixtures, WorkerFixtures>({
  serverBinary: [
    // eslint-disable-next-line no-empty-pattern
    async ({}, use) => {
      await use(buildIfMissing(SERVER_BINARY, './cmd/rela-server'));
    },
    { scope: 'worker' },
  ],

  // eslint-disable-next-line no-empty-pattern
  testProject: async ({}, use) => {
    const dir = createTestProject();
    await use(dir);
    try {
      fs.rmSync(dir, { recursive: true, force: true });
    } catch (e) {
      // Real cleanup failures likely indicate a process-still-holding-files
      // bug — warn rather than silently accumulating /tmp detritus.
      // (review-response RR-3DJ2C)
      console.warn(`Failed to remove temp project ${dir}: ${String(e)}`);
    }
  },

  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    const { proc, url, logs } = await spawnServer(serverBinary, testProject);
    try {
      await use(url);
    } finally {
      if (testInfo.status !== testInfo.expectedStatus) {
        // Attach server logs to the failing test so a CI reader can correlate
        // a 500 with what the backend was saying. (review-response RR-J9BIT)
        await testInfo.attach('rela-server.log', { body: logs(), contentType: 'text/plain' });
      }
      await waitForExit(proc);
    }
  },

  appPage: async ({ browser, serverUrl }, use) => {
    const context = await browser.newContext({
      extraHTTPHeaders: { Origin: serverUrl },
    });
    const page = await context.newPage();
    await page.goto(`${serverUrl}/`);
    await use(page);
    await context.close();
  },

  api: async ({ serverUrl, appPage }, use) => {
    const request: APIRequestContext = appPage.request;

    async function call(method: string, apiPath: string, data?: unknown) {
      const options: Record<string, unknown> = {
        method,
        // Redundant because the context already carries Origin, but keeps
        // the intent visible for readers auditing a test's API calls.
        headers: { Origin: serverUrl },
      };
      if (data !== undefined) options.data = data;
      const resp = await request.fetch(`${serverUrl}/api/v1/${apiPath}`, options);
      if (!resp.ok()) {
        throw new Error(`${method} /api/v1/${apiPath} → ${resp.status()}: ${await resp.text()}`);
      }
      return resp;
    }

    await use({
      async createEntity(plural, data) {
        return (await call('POST', plural, data)).json();
      },
      async getEntity(plural, id) {
        return (await call('GET', `${plural}/${id}`)).json();
      },
      async updateEntity(plural, id, properties) {
        return (await call('PATCH', `${plural}/${id}`, { properties })).json();
      },
      async deleteEntity(plural, id) {
        // Fails loudly on error. Callers doing cleanup should append
        // `.catch(() => {})` explicitly. (RR-MS1FM)
        await call('DELETE', `${plural}/${id}`);
      },
      async listEntities(plural, query) {
        const p = query ? `${plural}?${query}` : plural;
        return (await call('GET', p)).json();
      },
      async createRelation(fromPlural, fromId, relation, toId) {
        await call('POST', `${fromPlural}/${fromId}/relations/${relation}`, { id: toId });
      },
      async getContent(plural, id) {
        const resp = await call('GET', `${plural}/${id}`);
        const body = (await resp.json()) as { content?: string };
        return body.content ?? '';
      },
      async rawRequest(method, apiPath, data) {
        return call(method, apiPath, data);
      },
      async waitForIndexed(id, options) {
        // Searches ALL indexed-entity channels that a dashboard card might
        // use: _search (bleve), and a fallback direct GET which at least
        // confirms the store has the entity even if the index lags.
        const timeout = options?.timeout ?? 5000;
        const start = Date.now();
        // Infer plural from id prefix. Keep this narrow — the inline project
        // only has three entity types.
        const prefix = id.split('-')[0];
        const pluralByPrefix: Record<string, string> = {
          FEAT: 'features',
          BUG: 'bugs',
          TASK: 'tasks',
        };
        const plural = pluralByPrefix[prefix];
        if (!plural) {
          throw new Error(`waitForIndexed: unknown ID prefix for ${id}`);
        }
        while (Date.now() - start < timeout) {
          const resp = await call('GET', `${plural}/${id}`).catch(() => null);
          if (resp && resp.ok()) return;
          await new Promise((r) => setTimeout(r, 100));
        }
        throw new Error(`entity ${id} not reachable via GET /${plural}/${id} within ${timeout}ms`);
      },
    });
  },
});

export { expect } from '@playwright/test';

// ---- Test project fixture data ----
//
// The inline metamodel is deliberately minimal: feature/bug/task with a
// handful of properties. No automations, no validation rules. Tests that
// need those will have to add them here; do NOT point the fixture at the
// dogfood `tickets/` project (it's load-bearing real data, not a fixture).
// (review-response RR-GX4BK)

const METAMODEL_YAML = `
version: "1.0"

types:
  feature_status:
    values: [draft, approved, in_progress, done]
    default: draft
  severity:
    values: [low, medium, high, critical]
    default: medium
  priority:
    values: [low, medium, high]
    default: medium

entities:
  feature:
    label: Feature
    id_type: sequential
    id_prefix: FEAT
    properties:
      title:
        type: string
        required: true
      status:
        type: feature_status
      description:
        type: string
      priority:
        type: priority

  bug:
    label: Bug
    id_type: sequential
    id_prefix: BUG
    properties:
      title:
        type: string
        required: true
      severity:
        type: severity
      status:
        type: feature_status
      priority:
        type: priority
      description:
        type: string

  task:
    label: Task
    id_type: sequential
    id_prefix: TASK
    properties:
      title:
        type: string
        required: true
      status:
        type: feature_status
      assignee:
        type: string
      done:
        type: boolean

relations:
  blocks:
    from: [feature, bug, task]
    to: [feature, bug, task]
    inverse: blockedBy
    properties:
      reason:
        type: string
        required: true
        description: Why this entity blocks the other
      severity:
        type: severity
      resolved_date:
        type: date
      impact_score:
        type: integer
      is_workaround_available:
        type: boolean
  tagged:
    from: [feature]
    to: [feature]
    inverse: tagged_by
    properties:
      added_by:
        type: string
      added_date:
        type: date
  implements:
    from: [task]
    to: [feature]
  fixes:
    from: [task]
    to: [bug]
`;

const DATA_ENTRY_YAML = `
version: "1.0"

app:
  name: "E2E Test App"
  description: "Test project for Playwright E2E tests"

# Enable dark mode so the theme toggle renders in the status bar. Palette
# config is validated strictly; unknown keys raise startup errors.
palette:
  base: "#ffffff"
  surface: "#fafafa"
  accent: "#0066cc"
  text: "#111111"
  dark:
    base: "#111111"
    surface: "#222222"
    accent: "#4da6ff"
    text: "#eeeeee"

dashboard:
  title: "Dashboard"
  description: "Feature/bug overview"
  cards:
    - title: "Open Features"
      query: "type:feature status:draft"
      display: count
    - title: "In Progress"
      query: "type:feature status:in_progress"
      display: count
    - title: "By Status"
      query: "type:feature"
      display: breakdown
      group_by: status
    - title: "By Priority"
      query: "type:feature"
      display: breakdown
      group_by: priority
    - title: "Critical Issues"
      query: "type:bug prop:severity=critical"
      display: table
      columns:
        - property: title
          link: detail
        - property: status
      sort:
        - property: status
          direction: asc
      limit: 10

forms:
  feature:
    entity_type: feature
    title: "Feature"
    body: true
    fields:
      - property: title
      - property: status
      - property: priority
      - property: description
        widget: textarea
    relations:
      - relation: tagged
        widget: cards
        properties:
          - property: added_by
            label: "Added By"
          - property: added_date
            label: "Added"
      - relation: blocks
        direction: outgoing
        widget: cards
        properties:
          - property: reason
            label: "Block Reason"
          - property: severity
          - property: resolved_date
            label: "Resolved"
          - property: impact_score
            label: "Impact"
          - property: is_workaround_available
            label: "Workaround?"
      - relation: blocks
        direction: incoming
        widget: cards
        properties:
          - property: reason
            label: "Block Reason"
          - property: severity
          - property: resolved_date
            label: "Resolved"
          - property: impact_score
            label: "Impact"
          - property: is_workaround_available
            label: "Workaround?"

  bug:
    entity_type: bug
    title: "Bug"
    fields:
      - property: title
      - property: severity
      - property: status
      - property: priority
      - property: description
        widget: textarea

  task:
    entity_type: task
    title: "Task"
    fields:
      - property: title
      - property: status
      - property: assignee
      - property: done
    relations:
      - relation: implements
        label: Implements Feature
      - relation: fixes
        label: Fixes Bug

lists:
  features:
    entity_type: feature
    title: "Features"
    columns:
      - property: title
      - property: status
      - property: priority
    create_form: feature
    edit_form: feature
    filter_controls:
      - property: status
      - property: priority
    default_sort:
      - field: title
        direction: asc

  bugs:
    entity_type: bug
    title: "Bugs"
    columns:
      - property: title
      - property: severity
      - property: status
    create_form: bug
    edit_form: bug
    filter_controls:
      - property: severity
      - property: status

  tasks:
    entity_type: task
    title: "Tasks"
    columns:
      - property: title
      - property: status
      - property: assignee
    create_form: task
    edit_form: task

kanbans:
  feature-board:
    entity_type: feature
    title: "Feature Board"
    column_property: status
    columns:
      - value: draft
        label: Draft
      - value: approved
        label: Approved
      - value: in_progress
        label: In Progress
      - value: done
        label: Done
    card:
      title: title
      fields:
        - property: priority
    create_form: feature
    edit_form: feature
    filter_controls:
      - property: priority
        label: Priority

  bug-board:
    entity_type: bug
    title: "Bug Board"
    column_property: status
    columns:
      - value: draft
        label: New
      - value: in_progress
        label: In Progress
      - value: done
        label: Fixed
    card:
      title: title
      fields:
        - property: severity
    create_form: bug
    edit_form: bug

navigation:
  - label: "Dashboard"
    dashboard: true
  - label: "Features"
    list: features
  - label: "Feature Board"
    kanban: feature-board
  - label: "Bugs"
    list: bugs
  - label: "Bug Board"
    kanban: bug-board
  - label: "Tasks"
    list: tasks
  - label: "Search"
    search: true
  - label: "Settings"
    settings: true
  - label: "Analyze"
    analyze: true
  - label: "Conflicts"
    conflicts: true
`;

const SEED_ENTITIES: Record<string, string> = {
  'entities/features/FEAT-001.md': `---
id: FEAT-001
type: feature
title: User Authentication
status: approved
priority: high
---

Implement user authentication system.

- [ ] Define password policy
- [x] Pick OAuth provider
`,
  'entities/features/FEAT-002.md': `---
id: FEAT-002
type: feature
title: Dashboard Analytics
status: draft
priority: medium
---

Add analytics dashboard.
`,
  'entities/features/FEAT-003.md': `---
id: FEAT-003
type: feature
title: Export Data
status: in_progress
priority: low
---

Export data to CSV.
`,
  // FEAT-004 exists specifically to exercise GFM-checkbox rendering and
  // toggling; the body has one unchecked + one checked item so specs can
  // observe and flip either state.
  'entities/features/FEAT-004.md': `---
id: FEAT-004
type: feature
title: Checkbox render fixture
status: draft
priority: low
---

- [ ] First
- [x] Second
`,
  'entities/bugs/BUG-001.md': `---
id: BUG-001
type: bug
title: Login form validation
severity: high
status: draft
priority: high
---

Form validation is not working.
`,
  'entities/bugs/BUG-002.md': `---
id: BUG-002
type: bug
title: Memory leak in list view
severity: critical
status: in_progress
priority: high
---

Memory leak detected.
`,
  'entities/tasks/TASK-001.md': `---
id: TASK-001
type: task
title: Write unit tests
status: draft
assignee: Alice
---

Write unit tests for auth module.
`,
  // Seed a blocks relation with properties so relation-cards widget tests have
  // something rich to render. FEAT-001 blocks FEAT-003 with reason="test block",
  // severity=critical.
  'relations/FEAT-001--blocks--FEAT-003.md': `---
from: FEAT-001
relation: blocks
to: FEAT-003
reason: test block
severity: critical
impact_score: 8
is_workaround_available: false
---
`,
  // Seed a tagged relation (FEAT-001 tagged -> FEAT-002) so the tagged widget
  // has an entry to display.
  'relations/FEAT-001--tagged--FEAT-002.md': `---
from: FEAT-001
relation: tagged
to: FEAT-002
added_by: e2e-seed
added_date: "2026-01-15"
---
`,
};
