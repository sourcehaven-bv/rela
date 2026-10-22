import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import type { Browser } from "@playwright/test";
import { test as base, SEED, contextsAs, serveWithPrincipalHeader } from "./fixtures";
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
// header identifies.

const FEATURE = SEED.features.authentication;

interface ChannelFixtures {
  /** Opens a page as `user` on the running server. */
  as: (user: string) => Promise<ChannelsPage>;
}

// The `as` fixture of both describes below.
async function asFixture(
  { browser, serverUrl }: { browser: Browser; serverUrl: string },
  use: (as: ChannelFixtures["as"]) => Promise<void>,
) {
  const pages = contextsAs(browser, serverUrl);
  await use(async (user) => new ChannelsPage(await pages.as(user)));
  await pages.close();
}

const test = base.extend<ChannelFixtures>({
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    writeRoomProject(testProject);
    await serveWithPrincipalHeader(serverBinary, testProject, testInfo, use, CHANNEL_USERS.lead);
  },
  as: asFixture,
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

  test("a reader sees who is typing until they send", async ({ as }) => {
    const alice = await as(CHANNEL_USERS.lead);
    const bob = await as(CHANNEL_USERS.member);
    for (const p of [alice, bob]) {
      await p.openEntity("feature", FEATURE);
      await p.expectChannel(ROOM.label);
    }
    await alice.expectNoTyping();

    await bob.draft("Almost done");
    await alice.expectTyping(`${CHANNEL_USERS.member} is typing…`);
    // The typist does not see themselves.
    await bob.expectNoTyping();

    await bob.post("Almost done");
    await alice.expectMessage("Almost done");
    await alice.expectNoTyping();
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

    // The channel's `read` rule in realtime.yaml narrows to holders of
    // channel-lead. rela-server reads acl.yaml once at start, so a running
    // server's read access is revoked through realtime.yaml, which it
    // applies live.
    writeRoomRealtime(testProject, { leadReadsOnly: true });
    await bob.expectNoPanel(30_000);

    await alice.post("After the change");
    await alice.expectMessage("After the change");
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
    await serveWithPrincipalHeader(serverBinary, testProject, testInfo, use, FACED_USERS.editor);
  },
  as: asFixture,
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

  facedTest("each face has its own channel", async ({ as }) => {
    const draft = await as(FACED_USERS.editor);
    const published = await as(FACED_USERS.editor);
    const reader = await as(FACED_USERS.reader);
    await published.openEntity("policy", FACED_SEED.both.id);
    await reader.openEntity("policy", FACED_SEED.both.id);
    await published.expectChannel(POLICY_ROOM.label);
    await published.post("On the published face");
    await reader.expectMessage("On the published face");

    await draft.openEntity("policy", `${FACED_SEED.both.id}@draft`);
    await draft.expectChannel(POLICY_ROOM.label);
    await draft.expectComposer();
    await draft.post("On the draft face");
    await draft.expectMessage("On the draft face");
    await draft.expectNoMessage("On the published face");

    // A later published-face post arrives live, so the draft post had its
    // chance to arrive too; it reaches neither published-face reader.
    await reader.post("Later on the published face");
    await published.expectMessage("Later on the published face");
    await published.expectNoMessage("On the draft face");
    await reader.expectNoMessage("On the draft face");
    await draft.expectNoMessage("Later on the published face");
  });
});
