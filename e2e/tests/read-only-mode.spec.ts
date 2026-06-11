/**
 * AC10 (TKT-LFT2): the AWM6L payoff — `rela-server --read-only`
 * produces a SPA with no entity-CRUD write controls. The user lands
 * on a list page and sees no "+ New" button; lands on an entity
 * detail page and sees no Edit / Delete buttons; direct-URL form
 * navigation shows a "not editable" message.
 *
 * Deferred phase-2 sites (Lua command buttons, settings / theme /
 * git, relation add/remove inside form widgets, inline-edit buttons
 * in related-entity cards) remain visible and 403 at the server on
 * click. The assertions below intentionally exclude those.
 *
 * The test reuses the standard `testProject` fixture to get a fresh
 * project directory, then spawns its own `--read-only` server (the
 * default `serverUrl` fixture is unrestricted, so we can't reuse
 * it). The spawn logic is a slim copy of the fixture's; if it ever
 * grows, factor a `spawnReadOnlyServer` helper into fixtures.ts.
 */
import { test as base } from './fixtures';
import { EntityPage, FormPage, ListPage } from '../pages';
import { spawn, type ChildProcess } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import * as net from 'net';

const PROJECT_ROOT = path.resolve(__dirname, '../..');
const SERVER_BINARY = path.join(PROJECT_ROOT, 'bin', 'rela-server');

function findFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.unref();
    srv.on('error', reject);
    srv.listen(0, () => {
      const addr = srv.address();
      if (addr && typeof addr === 'object') {
        const { port } = addr;
        srv.close(() => resolve(port));
      } else {
        srv.close();
        reject(new Error('no port'));
      }
    });
  });
}

async function waitForServer(url: string, origin: string, timeoutMs = 15_000): Promise<void> {
  // Match the production fixture's probe URL — /api/v1/_config is
  // unauthenticated and always renders once the project is loaded.
  const probeUrl = `${url}/api/v1/_config`;
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const resp = await fetch(probeUrl, { headers: { Origin: origin } });
      if (resp.ok) return;
    } catch {
      /* keep polling */
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error(`server at ${probeUrl} did not become ready within ${timeoutMs}ms`);
}

// Extend the test base with a `readOnlyServerUrl` fixture that boots
// a `--read-only` server against the standard testProject. The
// existing `serverUrl` stays unrestricted; this fixture is what
// read-only tests bind to.
const test = base.extend<{ readOnlyServerUrl: string }>({
  readOnlyServerUrl: async ({ testProject }, use, testInfo) => {
    if (!fs.existsSync(SERVER_BINARY)) {
      throw new Error(
        `rela-server binary not found at ${SERVER_BINARY}; run 'just build' first or rely on the buildIfMissing helper in fixtures.ts.`,
      );
    }
    const port = await findFreePort();
    // 127.0.0.1 (not localhost) because rela-server binds IPv4 only;
    // Node 18+ fetch will try IPv6 ::1 first when given `localhost`
    // and timeout if the server is IPv4-only.
    const url = `http://127.0.0.1:${port}`;
    const proc: ChildProcess = spawn(
      SERVER_BINARY,
      ['-port', String(port), '-allowed-origin', url, '--read-only'],
      { cwd: testProject, env: process.env, stdio: ['ignore', 'pipe', 'pipe'] },
    );
    let stdout = '';
    let stderr = '';
    proc.stdout?.on('data', (d) => (stdout += d.toString()));
    proc.stderr?.on('data', (d) => (stderr += d.toString()));
    try {
      try {
        await waitForServer(url, url);
      } catch (e) {
        // Surface server logs in the test error message — fixture
        // attach() doesn't fire when the fixture itself throws.
        throw new Error(
          `${e instanceof Error ? e.message : String(e)}\n--- server stdout ---\n${stdout}\n--- server stderr ---\n${stderr}`,
          { cause: e },
        );
      }
      await use(url);
    } finally {
      if (testInfo.status !== testInfo.expectedStatus) {
        await testInfo.attach('rela-server.log', {
          body: `--- stdout ---\n${stdout}\n--- stderr ---\n${stderr}\n`,
          contentType: 'text/plain',
        });
      }
      proc.kill('SIGTERM');
      await new Promise((r) => setTimeout(r, 200));
      if (!proc.killed) proc.kill('SIGKILL');
    }
  },
});

test.describe('Read-only mode hides entity-CRUD controls (AC10)', () => {
  test('list page has no "+ New" button and no delete buttons', async ({
    browser,
    readOnlyServerUrl,
  }) => {
    const context = await browser.newContext({
      extraHTTPHeaders: { Origin: readOnlyServerUrl },
    });
    const page = await context.newPage();
    await page.goto(`${readOnlyServerUrl}/list/features`);

    const listPage = new ListPage(page);
    await listPage.waitForRowsRendered();
    // "+ New" gated on collection `_actions.create=false`; row delete
    // buttons gated on per-entity `_actions.delete=false`. Both denied
    // under ReadOnlyACL.
    await listPage.expectNoCreateAffordance();
    await listPage.expectNoRowDeleteButtons();

    await context.close();
  });

  test('entity detail page has no Edit or Delete buttons', async ({
    browser,
    readOnlyServerUrl,
  }) => {
    const context = await browser.newContext({
      extraHTTPHeaders: { Origin: readOnlyServerUrl },
    });
    const page = await context.newPage();
    await page.goto(`${readOnlyServerUrl}/entity/feature/FEAT-001`);

    const entityPage = new EntityPage(page);
    await entityPage.waitForHeading();
    // Edit + Delete gated on _actions.{update, delete}, both false
    // under ReadOnlyACL.
    await entityPage.expectNoEditButton();
    await entityPage.expectNoDeleteButton();

    await context.close();
  });

  test('direct form-URL navigation shows "not editable" message', async ({
    browser,
    readOnlyServerUrl,
  }) => {
    const context = await browser.newContext({
      extraHTTPHeaders: { Origin: readOnlyServerUrl },
    });
    const page = await context.newPage();
    await page.goto(`${readOnlyServerUrl}/form/feature/FEAT-001`);

    // Form route guard detects _actions.update === false and renders
    // an inline message instead of the form.
    const formPage = new FormPage(page);
    await formPage.expectNotEditableMessage();

    await context.close();
  });
});
