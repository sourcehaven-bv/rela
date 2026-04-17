import { test as base, type Page } from '@playwright/test';
import { spawn, type ChildProcess } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import * as net from 'net';

export interface DesktopFixtures {
  /** The Playwright page connected to the app */
  appPage: Page;
  /** Path to the test project directory */
  testProject: string;
  /** Base URL for the server */
  serverUrl: string;
}

// Find an available port
async function findAvailablePort(startPort: number = 34115): Promise<number> {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.listen(startPort, () => {
      const port = (server.address() as net.AddressInfo).port;
      server.close(() => resolve(port));
    });
    server.on('error', () => {
      resolve(findAvailablePort(startPort + 1));
    });
  });
}

// Create a rich test project with metamodel, data-entry config, entities, and kanban
function createTestProject(): string {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rela-e2e-'));

  fs.writeFileSync(path.join(tmpDir, 'metamodel.yaml'), `
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

relations:
  blocks:
    from: [feature, bug, task]
    to: [feature, bug, task]
  implements:
    from: [task]
    to: [feature]
  fixes:
    from: [task]
    to: [bug]
`);

  fs.writeFileSync(path.join(tmpDir, 'data-entry.yaml'), `
version: "1.0"

app:
  name: "E2E Test App"
  description: "Test project for Playwright E2E tests"

forms:
  feature:
    entity_type: feature
    title: "Feature"
    fields:
      - property: title
      - property: status
      - property: priority
      - property: description
        widget: textarea

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
`);

  // Create entity directories
  fs.mkdirSync(path.join(tmpDir, 'entities', 'features'), { recursive: true });
  fs.mkdirSync(path.join(tmpDir, 'entities', 'bugs'), { recursive: true });
  fs.mkdirSync(path.join(tmpDir, 'entities', 'tasks'), { recursive: true });
  fs.mkdirSync(path.join(tmpDir, 'relations'), { recursive: true });

  // Create some test entities
  fs.writeFileSync(path.join(tmpDir, 'entities', 'features', 'FEAT-001.md'), `---
id: FEAT-001
type: feature
title: User Authentication
status: approved
priority: high
---

Implement user authentication system.
`);

  fs.writeFileSync(path.join(tmpDir, 'entities', 'features', 'FEAT-002.md'), `---
id: FEAT-002
type: feature
title: Dashboard Analytics
status: draft
priority: medium
---

Add analytics dashboard.
`);

  fs.writeFileSync(path.join(tmpDir, 'entities', 'features', 'FEAT-003.md'), `---
id: FEAT-003
type: feature
title: Export Data
status: in_progress
priority: low
---

Export data to CSV.
`);

  fs.writeFileSync(path.join(tmpDir, 'entities', 'bugs', 'BUG-001.md'), `---
id: BUG-001
type: bug
title: Login form validation
severity: high
status: draft
priority: high
---

Form validation is not working.
`);

  fs.writeFileSync(path.join(tmpDir, 'entities', 'bugs', 'BUG-002.md'), `---
id: BUG-002
type: bug
title: Memory leak in list view
severity: critical
status: in_progress
priority: high
---

Memory leak detected.
`);

  fs.writeFileSync(path.join(tmpDir, 'entities', 'tasks', 'TASK-001.md'), `---
id: TASK-001
type: task
title: Write unit tests
status: draft
assignee: Alice
---

Write unit tests for auth module.
`);

  return tmpDir;
}

let serverProcess: ChildProcess | null = null;
let testProjectDir: string | null = null;
let serverPort: number | null = null;

export const test = base.extend<DesktopFixtures>({
  testProject: async ({}, use) => {
    if (!testProjectDir) {
      testProjectDir = createTestProject();
    }
    await use(testProjectDir);
  },

  serverUrl: async ({}, use) => {
    await use(`http://localhost:${serverPort}`);
  },

  appPage: async ({ browser, testProject }, use) => {
    const projectRoot = path.resolve(__dirname, '../..');

    // Use rela-server for HTTP-based testing
    const binaryPath = path.join(projectRoot, 'bin', 'rela-server');

    if (!fs.existsSync(binaryPath)) {
      throw new Error(
        `Server binary not found at ${binaryPath}. Run 'just build-server' first.`
      );
    }

    // Find available port
    serverPort = await findAvailablePort();

    // Start the server with the test project
    serverProcess = spawn(binaryPath, ['-port', String(serverPort)], {
      cwd: testProject,
      env: process.env,
      stdio: ['ignore', 'pipe', 'pipe'],
    });

    // Collect output for debugging
    let stdout = '';
    let stderr = '';
    serverProcess.stdout?.on('data', (data) => {
      stdout += data.toString();
    });
    serverProcess.stderr?.on('data', (data) => {
      stderr += data.toString();
    });

    // Wait for server to start
    const serverUrl = `http://localhost:${serverPort}`;
    let ready = false;
    const maxAttempts = 100;

    for (let i = 0; i < maxAttempts && !ready; i++) {
      try {
        const response = await fetch(`${serverUrl}/`);
        if (response.ok || response.status === 200) {
          ready = true;
        }
      } catch {
        await new Promise((resolve) => setTimeout(resolve, 50));
      }
    }

    if (!ready) {
      console.error('Server stdout:', stdout);
      console.error('Server stderr:', stderr);
      throw new Error(`Server failed to start on ${serverUrl}`);
    }

    // Create a new page and navigate to the SPA. Provide an Origin header so
    // that direct API calls via page.request pass the same-origin CSRF check.
    const context = await browser.newContext({
      extraHTTPHeaders: { Origin: serverUrl },
    });
    const page = await context.newPage();
    await page.goto(`${serverUrl}/`);

    await use(page);

    // Cleanup
    await context.close();
    if (serverProcess) {
      serverProcess.kill('SIGTERM');
      serverProcess = null;
    }
  },
});

export { expect } from '@playwright/test';

// Cleanup on process exit
process.on('exit', () => {
  if (serverProcess) {
    serverProcess.kill('SIGTERM');
  }
  if (testProjectDir) {
    try {
      fs.rmSync(testProjectDir, { recursive: true, force: true });
    } catch {
      // Ignore cleanup errors
    }
  }
});
