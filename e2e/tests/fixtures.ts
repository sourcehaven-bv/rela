import { test as base, type Page, type Browser } from '@playwright/test';
import { spawn, type ChildProcess, execSync } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import * as net from 'net';

export interface DesktopFixtures {
  /** The Playwright page connected to the app */
  appPage: Page;
  /** Path to the test project directory */
  testProject: string;
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

// Create a minimal test project with metamodel and data-entry config
function createTestProject(): string {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rela-e2e-'));

  fs.writeFileSync(path.join(tmpDir, 'metamodel.yaml'), `
version: "1.0"

types:
  feature_status:
    values: [draft, approved, done]
    default: draft
  severity:
    values: [low, medium, high, critical]
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

relations:
  blocks:
    from: [feature, bug]
    to: [feature, bug]
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
      - property: description
        widget: textarea

  bug:
    entity_type: bug
    title: "Bug"
    fields:
      - property: title
      - property: severity

lists:
  features:
    entity_type: feature
    title: "Features"
    columns:
      - property: title
      - property: status
    create_form: feature
    edit_form: feature

  bugs:
    entity_type: bug
    title: "Bugs"
    columns:
      - property: title
      - property: severity
    create_form: bug
    edit_form: bug

navigation:
  - label: "Features"
    list: features
  - label: "Bugs"
    list: bugs
`);

  // Create entity directories
  fs.mkdirSync(path.join(tmpDir, 'entities', 'feature'), { recursive: true });
  fs.mkdirSync(path.join(tmpDir, 'entities', 'bug'), { recursive: true });
  fs.mkdirSync(path.join(tmpDir, 'relations'), { recursive: true });

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

  appPage: async ({ browser, testProject }, use) => {
    const projectRoot = path.resolve(__dirname, '../..');

    // Use rela-server instead of rela-desktop for HTTP-based testing
    // This tests the same data entry UI without needing CDP/WebKit complexity
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
    const maxAttempts = 30;

    for (let i = 0; i < maxAttempts && !ready; i++) {
      try {
        const response = await fetch(serverUrl);
        if (response.ok) {
          ready = true;
        }
      } catch {
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
    }

    if (!ready) {
      console.error('Server stdout:', stdout);
      console.error('Server stderr:', stderr);
      throw new Error(`Server failed to start on ${serverUrl}`);
    }

    // Create a new page and navigate to the app
    const context = await browser.newContext();
    const page = await context.newPage();
    await page.goto(serverUrl);

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
