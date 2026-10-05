/**
 * Playwright fixtures for the faced project (TKT-WCMW47).
 *
 * `facedTest` is `test` from `fixtures.ts` with two overrides: the project is
 * the faced one from `faced-project.ts`, and the server runs as the principal
 * named by the `facedUser` option. Choose it per file or describe block:
 *
 *     facedTest.use({ facedUser: FACED_USERS.reader });
 *
 * Every other fixture (`appPage`, `api`, the off-origin and Vue-error guards)
 * is inherited unchanged. `facedApi` adds the typed helpers a faced type needs
 * and the base `api` lacks: a create names its face, a read names an address
 * (`ID@face`) and optionally a world.
 *
 * `facedPgTest` is the same on the postgres backend, for version history. The
 * postgres server does not read the seed files, so a spec on it seeds through
 * `facedApi` first. Wait for captured versions with the base
 * `api.waitForEntityVersions`.
 */
import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import type { APIRequestContext, APIResponse, Page, TestInfo } from "@playwright/test";
import {
  test,
  postgresTest,
  spawnServer,
  waitForExit,
  pgDsnForSchema,
  type EntityResponse,
} from "./fixtures";
import { FACED_USERS, type FacedUser, writeFacedProject } from "./faced-project";

export {
  FACED_USERS,
  FACED_SEED,
  FACE,
  WORLD,
} from "./faced-project";
export { expect, POSTGRES_E2E_ENABLED } from "./fixtures";

export interface FacedApi {
  /** Create a policy in `face`. A faced create must name its face. */
  createPolicy(
    face: string,
    properties: Record<string, unknown>,
    content?: string,
  ): Promise<EntityResponse>;
  /** Read a policy by address (`ID` or `ID@face`), optionally in a world.
   *  Resolves to null on 404: a hidden face and a missing one look alike. */
  getPolicy(address: string, world?: string): Promise<EntityResponse | null>;
  /** Set properties on one face, addressed as `ID@face`. */
  updatePolicy(address: string, properties: Record<string, unknown>): Promise<void>;
  /** Invoke a copy definition (e.g. `publish`) on a source entity. */
  invokeCopy(name: string, sourceId: string): Promise<void>;
  /** A face's comment thread. */
  listComments(type: string, address: string): Promise<Array<{ id: string; body: string }>>;
  /** The incoming edges of `relation` on `plural/id`, as the relations route
   *  serves them: a content-scoped edge names its source face. */
  listIncoming(
    plural: string,
    id: string,
    relation: string,
  ): Promise<Array<{ id: string; face?: string; editable?: boolean }>>;
}

function readFacedApi(request: APIRequestContext, serverUrl: string): FacedApi {
  async function send(method: string, apiPath: string, data?: unknown): Promise<APIResponse> {
    const options: Record<string, unknown> = { method, headers: { Origin: serverUrl } };
    if (data !== undefined) options.data = data;
    return request.fetch(`${serverUrl}/api/v1/${apiPath}`, options);
  }
  async function call(method: string, apiPath: string, data?: unknown): Promise<APIResponse> {
    const resp = await send(method, apiPath, data);
    if (!resp.ok()) {
      throw new Error(`${method} /api/v1/${apiPath} → ${resp.status()}: ${await resp.text()}`);
    }
    return resp;
  }
  const withWorld = (p: string, world?: string) =>
    world ? `${p}?world=${encodeURIComponent(world)}` : p;

  return {
    async createPolicy(face, properties, content) {
      const body: Record<string, unknown> = { face, properties };
      if (content !== undefined) body.content = content;
      return (await call("POST", "policies", body)).json();
    },
    async getPolicy(address, world) {
      const resp = await send("GET", withWorld(`policies/${address}`, world));
      if (resp.status() === 404) return null;
      if (!resp.ok()) {
        throw new Error(`GET policies/${address} → ${resp.status()}: ${await resp.text()}`);
      }
      return resp.json();
    },
    async updatePolicy(address, properties) {
      await call("PATCH", `policies/${address}`, { properties });
    },
    async invokeCopy(name, sourceId) {
      await call("POST", `_copies/${name}`, { source_id: sourceId });
    },
    async listComments(type, address) {
      const body = await (await call("GET", `_comments/${type}/${address}`)).json();
      return body.comments ?? [];
    },
    async listIncoming(plural, id, relation) {
      return (await call("GET", `${plural}/${id}/relations/${relation}?direction=incoming`)).json();
    },
  };
}

interface FacedFixtures {
  /** The principal the server runs as. Defaults to the all-faces editor. */
  facedUser: FacedUser;
  facedApi: FacedApi;
}

// ---- Fixture bodies shared by facedTest and facedPgTest ----

async function facedProject(use: (dir: string) => Promise<void>): Promise<void> {
  const dir = fs.mkdtempSync(path.join(fs.realpathSync(os.tmpdir()), "rela-e2e-faced-"));
  try {
    writeFacedProject(dir);
    await use(dir);
  } finally {
    try {
      fs.rmSync(dir, { recursive: true, force: true });
    } catch (e) {
      console.warn(`Failed to remove temp project ${dir}: ${String(e)}`);
    }
  }
}

/** Spawn a server on `dir` with `env`, attach its log on failure, and stop it
 *  afterwards. Mirrors the base `serverUrl` fixtures. */
async function serveFaced(
  binary: string,
  dir: string,
  env: NodeJS.ProcessEnv,
  logName: string,
  testInfo: TestInfo,
  use: (url: string) => Promise<void>,
): Promise<void> {
  const { proc, url, logs } = await spawnServer(binary, dir, env);
  try {
    await use(url);
  } finally {
    if (testInfo.status !== testInfo.expectedStatus) {
      await testInfo.attach(logName, { body: logs(), contentType: "text/plain" });
    }
    await waitForExit(proc);
  }
}

async function facedApiFixture(
  appPage: Page,
  serverUrl: string,
  use: (api: FacedApi) => Promise<void>,
): Promise<void> {
  await use(readFacedApi(appPage.request, serverUrl));
}

export const facedTest = test.extend<FacedFixtures>({
  facedUser: [FACED_USERS.editor, { option: true }],
  // eslint-disable-next-line no-empty-pattern
  testProject: async ({}, use) => facedProject(use),
  serverUrl: async ({ testProject, serverBinary, facedUser }, use, testInfo) =>
    serveFaced(
      serverBinary,
      testProject,
      { RELA_DATAENTRY_USER: facedUser },
      "rela-server.log",
      testInfo,
      use,
    ),
  facedApi: async ({ appPage, serverUrl }, use) => facedApiFixture(appPage, serverUrl, use),
});

export const facedPgTest = postgresTest.extend<FacedFixtures>({
  facedUser: [FACED_USERS.editor, { option: true }],
  // eslint-disable-next-line no-empty-pattern
  testProject: async ({}, use) => facedProject(use),
  serverUrl: async ({ testProject, serverBinary, pgSchema, facedUser }, use, testInfo) =>
    serveFaced(
      serverBinary,
      testProject,
      { RELA_DATABASE_URL: pgDsnForSchema(pgSchema), RELA_DATAENTRY_USER: facedUser },
      "rela-server-postgres.log",
      testInfo,
      use,
    ),
  facedApi: async ({ appPage, serverUrl }, use) => facedApiFixture(appPage, serverUrl, use),
});
