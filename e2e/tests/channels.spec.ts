import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import type { Browser, TestInfo } from "@playwright/test";
import { test as base, SEED, spawnServer, waitForExit } from "./fixtures";
import { FACED_SEED, FACED_USERS, writeFacedProject } from "./faced-project";
import {
  CHANNEL_USERS,
  POLICY_ROOM,
  ROOM,
  writePolicyRealtime,
  writeRoomProject,
  writeRoomRealtime,
} from "./realtime-project";
import { ChannelsPage } from "../pages/channels.page";

// Channels on the entity page (TKT-91BK6F AC 21), seen by several people at
// once. As in presence.spec.ts, each user is a browser context the proxy
// header identifies, and the server runs without $RELA_DATAENTRY_USER, which
// would win over the header.

const HEADER = "X-Forwarded-User";
const FEATURE = SEED.features.authentication;

interface ChannelFixtures {
  /** Opens a page as `user` on the running server. */
  as: (user: string) => Promise<ChannelsPage>;
}

// The probe names a user: with an acl.yaml, an anonymous request is refused.
async function serve(
  serverBinary: string,
  dir: string,
  probeUser: string,
  testInfo: TestInfo,
  use: (url: string) => Promise<void>,
) {
  const { proc, url, logs } = await spawnServer(
    serverBinary,
    dir,
    { RELA_DATAENTRY_USER: undefined },
    ["-principal-header", HEADER],
    { [HEADER]: probeUser },
  );
  try {
    await use(url);
  } finally {
    if (testInfo.status !== testInfo.expectedStatus) {
      await testInfo.attach("rela-server.log", { body: logs(), contentType: "text/plain" });
    }
    await waitForExit(proc);
  }
}

function pagesAs(browser: Browser, serverUrl: string) {
  const contexts: Awaited<ReturnType<Browser["newContext"]>>[] = [];
  return {
    as: async (user: string) => {
      const ctx = await browser.newContext({ baseURL: serverUrl, extraHTTPHeaders: { [HEADER]: user } });
      contexts.push(ctx);
      return new ChannelsPage(await ctx.newPage());
    },
    close: () => Promise.all(contexts.map((c) => c.close().catch(() => {}))),
  };
}

const test = base.extend<ChannelFixtures>({
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    writeRoomProject(testProject);
    await serve(serverBinary, testProject, CHANNEL_USERS.lead, testInfo, use);
  },
  as: async ({ browser, serverUrl }, use) => {
    const pages = pagesAs(browser, serverUrl);
    await use(pages.as);
    await pages.close();
  },
});

test.describe("channels", () => {
  test.describe.configure({ timeout: 120_000 });

  test("a post appears live for another reader; a user without the post role sees no composer", async ({
    as,
  }) => {
    const alice = await as(CHANNEL_USERS.lead);
    const bob = await as(CHANNEL_USERS.member);
    const carol = await as(CHANNEL_USERS.watcher);
    for (const p of [alice, bob, carol]) {
      await p.openEntity("feature", FEATURE);
      await p.expectChannel(ROOM.label);
    }

    await alice.expectComposer();
    await carol.expectNoComposer();

    await alice.post("Kickoff at **ten**");
    await alice.expectComposerCleared();
    await alice.expectMessage("Kickoff at ten");
    await bob.expectMessage("Kickoff at ten");
    await carol.expectMessage("Kickoff at ten");

    await bob.post("Works for me");
    await alice.expectMessage("Works for me");
  });

  test("a closed channel takes no posts", async ({ as }) => {
    const alice = await as(CHANNEL_USERS.lead);
    const bob = await as(CHANNEL_USERS.member);
    await alice.openEntity("feature", FEATURE);
    await bob.openEntity("feature", FEATURE);
    await alice.expectChannel(ROOM.label);

    // Lifecycle operations exist once the channel holds a message.
    await alice.post("Wrapping up");
    await bob.expectMessage("Wrapping up");
    await bob.expectNoLifecycle("Close");

    await alice.runLifecycle("Close");
    await alice.expectClosedNotice();
    await alice.expectNoComposer();
    // The other reader's open channel follows without a reload.
    await bob.expectClosedNotice();
    await bob.expectNoComposer();

    await alice.runLifecycle("Reopen");
    await bob.expectComposer();
  });

  test("a reader who loses read access loses the channel", async ({ as, testProject }) => {
    const alice = await as(CHANNEL_USERS.lead);
    const bob = await as(CHANNEL_USERS.member);
    await alice.openEntity("feature", FEATURE);
    await bob.openEntity("feature", FEATURE);
    await alice.post("Before the change");
    await bob.expectMessage("Before the change");

    // Only holders of channel-lead (acl.yaml) may read the channel now.
    writeRoomRealtime(testProject, { leadReadsOnly: true });
    await bob.expectNoPanel(30_000);

    await alice.post("After the change");
    await alice.expectMessage("After the change");
    await bob.expectNoPanel();
  });
});

// A faced type: the channel is per face, addressed by the face on screen.
const facedTest = base.extend<ChannelFixtures>({
  // eslint-disable-next-line no-empty-pattern -- Playwright reads the fixture list from the pattern
  testProject: async ({}, use) => {
    const dir = fs.mkdtempSync(path.join(fs.realpathSync(os.tmpdir()), "rela-e2e-channels-faced-"));
    try {
      writeFacedProject(dir);
      writePolicyRealtime(dir);
      await use(dir);
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  },
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    await serve(serverBinary, testProject, FACED_USERS.editor, testInfo, use);
  },
  as: async ({ browser, serverUrl }, use) => {
    const pages = pagesAs(browser, serverUrl);
    await use(pages.as);
    await pages.close();
  },
});

facedTest.describe("channels on a faced type", () => {
  facedTest.describe.configure({ timeout: 120_000 });

  facedTest("two users on the published face see each other's posts", async ({ as }) => {
    const editor = await as(FACED_USERS.editor);
    const reader = await as(FACED_USERS.reader);
    await editor.openEntity("policy", FACED_SEED.both.id);
    await reader.openEntity("policy", FACED_SEED.both.id);
    await editor.expectChannel(POLICY_ROOM.label);
    await reader.expectChannel(POLICY_ROOM.label);

    await editor.post("Is the published wording final?");
    await reader.expectMessage("Is the published wording final?");
    await reader.post("Yes");
    await editor.expectMessage("Yes");
  });
});
