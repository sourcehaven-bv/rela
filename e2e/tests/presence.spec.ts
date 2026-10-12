import type { BrowserContext } from "@playwright/test";
import { test as base, SEED, spawnServer, waitForExit } from "./fixtures";
import { LeaseClient, PresencePage } from "../pages/presence.page";

// Presence avatars and the claim badge (TKT-IMBF7U), seen by two people at
// once. Each user is a browser context the proxy header identifies; the
// server runs without $RELA_DATAENTRY_USER, which would win over the header
// and make both contexts the same user.

const HEADER = "X-Forwarded-User";
const ALICE = "alice@example.com";
const BOB = "bob@example.com";
const FEATURE = SEED.features.authentication;
const FEATURE_TITLE = "User Authentication";

interface PresenceFixtures {
  alice: PresencePage;
  bob: PresencePage;
  bobContext: BrowserContext;
}

const test = base.extend<PresenceFixtures>({
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    const { proc, url, logs } = await spawnServer(
      serverBinary,
      testProject,
      { RELA_DATAENTRY_USER: undefined },
      ["-principal-header", HEADER],
    );
    try {
      await use(url);
    } finally {
      if (testInfo.status !== testInfo.expectedStatus) {
        await testInfo.attach("rela-server.log", { body: logs(), contentType: "text/plain" });
      }
      await waitForExit(proc);
    }
  },
  alice: async ({ browser, serverUrl }, use) => {
    const ctx = await browser.newContext({ baseURL: serverUrl, extraHTTPHeaders: { [HEADER]: ALICE } });
    await use(new PresencePage(await ctx.newPage()));
    await ctx.close();
  },
  bobContext: async ({ browser, serverUrl }, use) => {
    const ctx = await browser.newContext({ baseURL: serverUrl, extraHTTPHeaders: { [HEADER]: BOB } });
    await use(ctx);
    await ctx.close().catch(() => {});
  },
  bob: async ({ bobContext }, use) => {
    await use(new PresencePage(await bobContext.newPage()));
  },
});

test.describe("presence", () => {
  test.describe.configure({ timeout: 120_000 });

  test("shows who else has the entity open, and drops them after they leave", async ({ alice, bob, bobContext }) => {
    await alice.openEntity("feature", FEATURE);
    await bob.openEntity("feature", FEATURE);

    await alice.expectViewer(BOB);
    await bob.expectViewer(ALICE);

    // Closing the page closes the stream; the server keeps bob listed for 15
    // seconds, then drops him.
    await bobContext.close();
    await alice.expectNoViewer(BOB, 45_000);
  });

  test("shows the claim badge on the entity, the list and the board", async ({ alice, serverUrl }) => {
    const bobAgent = await LeaseClient.as(serverUrl, HEADER, BOB);
    try {
      await alice.openEntity("feature", FEATURE);
      await alice.expectNoDetailBadge();

      const lease = await bobAgent.claim("features", FEATURE, {
        label: "e2e",
        status: "<img src=x onerror=alert(1)>",
      });
      const text = `Claimed by ${BOB} (e2e)`;
      // The live view subscription carries the claim to the open page.
      await alice.expectDetailBadge(text);

      await alice.open("/list/features");
      await alice.expectListBadge(FEATURE_TITLE, text);

      await alice.open("/kanban/feature-board");
      await alice.expectCardBadge(FEATURE_TITLE, text);

      // The board's own subscription carries the release.
      await bobAgent.release(lease);
      await alice.expectNoCardBadge(FEATURE_TITLE);

      await alice.openEntity("feature", FEATURE);
      await alice.expectNoDetailBadge();
    } finally {
      await bobAgent.dispose();
    }
  });
});
