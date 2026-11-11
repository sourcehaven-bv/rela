import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import type { Browser } from "@playwright/test";
import { test as base, expect, SEED, contextsAs, serveWithPrincipalHeader } from "./fixtures";
import { FACED_SEED, FACED_USERS, writeFacedProject } from "./faced-project";
import {
  CHANNEL_USERS,
  CHOICES,
  DECISIONS,
  INBOX,
  POLICY_ROOM,
  QUIET,
  ROOM,
  SESSION,
  addCompactSession,
  addOfferFalse,
  addFeatureOwner,
  addQuietRun,
  addRoomAboveRelations,
  setFeatureOwner,
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

  test("a mention reaches the target's inbox live and clears when the channel is read", async ({
    as,
  }) => {
    const alice = await as(CHANNEL_USERS.lead);
    const bob = await as(CHANNEL_USERS.member);
    await alice.openHome();
    await alice.expectNoInboxBadge();

    await bob.openEntity("feature", FEATURE);
    await bob.expectChannel(ROOM.label);
    await bob.post(`\`@${CHANNEL_USERS.lead}\` can you approve?`);
    await bob.expectMessage("can you approve?");

    // The badge moves without a reload.
    await alice.expectInboxBadge(1);
    await alice.openInbox();
    await alice.openInboxRow("mentions", `${CHANNEL_USERS.member} mentioned you`);
    await alice.expectChannelDeepLink("feature", FEATURE, `${ROOM.type}/`);
    await alice.expectChannel(ROOM.label);
    await alice.expectMessage("can you approve?");

    // Opening the channel marks it read, and the badge clears.
    await alice.expectNoInboxBadge();
  });
});

// A room with a session: the lead's message moves it for every reader.
const sessionTest = base.extend<ChannelFixtures>({
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    writeRoomProject(testProject, { session: true });
    await serveWithPrincipalHeader(serverBinary, testProject, testInfo, use, CHANNEL_USERS.lead);
  },
  as: asFixture,
});

sessionTest.describe("channels with a session", () => {
  sessionTest.describe.configure({ timeout: 120_000 });

  sessionTest("the lead's message moves the session live for every reader", async ({ as }) => {
    const alice = await as(CHANNEL_USERS.lead);
    const bob = await as(CHANNEL_USERS.member);
    for (const p of [alice, bob]) {
      await p.openEntity("feature", FEATURE);
      await p.expectChannel(ROOM.label);
      await p.expectSessionAt(SESSION.from);
    }
    // Both may post Contained, but only the lead's moves the session. The
    // lead gets it as a decision button in the header, and so not in the
    // composer's picker (TKT-A0RTC52).
    await alice.expectDecision(SESSION.messageLabel, true);
    await alice.expectMessageType(SESSION.messageLabel, false);
    await bob.expectComposer();
    await bob.expectMessageType(SESSION.messageLabel, true);
    // Wrap-up has in: [contained], so neither is offered it yet.
    await alice.expectMessageType(SESSION.wrapUpLabel, false);
    await bob.expectMessageType(SESSION.wrapUpLabel, false);
    await bob.expectTransitionMark(SESSION.messageLabel, false);
    await bob.selectMessageType(SESSION.messageLabel);
    await bob.expectSessionHint(null);

    // An ordinary message can be edited by its author.
    await alice.post("Looking into it");
    await alice.expectMessage("Looking into it");
    await alice.expectMessageControl("Looking into it", "Edit", true);

    // The note is required, so the button opens the form in decision mode.
    await alice.decide(SESSION.messageLabel);
    await alice.expectComposeStrip(SESSION.messageLabel);
    await alice.expectSessionHint(`Sending this moves the session to ${SESSION.to}.`);
    await alice.post("Access revoked", SESSION.noteLabel);

    const notice = `Session moved from ${SESSION.from} to ${SESSION.to}`;
    for (const p of [alice, bob]) {
      await p.expectSessionAt(SESSION.to);
      await p.expectMessage(notice);
      // The session is in Contained now, so Wrap-up is offered.
      await p.expectMessageType(SESSION.wrapUpLabel, true);
    }
    // A message that moved the session cannot be edited.
    await alice.expectMessage("Access revoked");
    await alice.expectMessageControl("Access revoked", "Edit", false);
    // The session ended, so nothing moves it any more.
    await alice.expectTransitionMark(SESSION.messageLabel, false);
  });
});

// A room with a review session: the lead decides with buttons (TKT-T9G1AE).
const decisionTest = base.extend<ChannelFixtures>({
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    writeRoomProject(testProject, { decisions: true });
    await serveWithPrincipalHeader(serverBinary, testProject, testInfo, use, CHANNEL_USERS.lead);
  },
  as: asFixture,
});

decisionTest.describe("session decisions", () => {
  decisionTest.describe.configure({ timeout: 120_000 });

  decisionTest("the lead decides with one click", async ({ as }) => {
    const alice = await as(CHANNEL_USERS.lead);
    const bob = await as(CHANNEL_USERS.member);
    for (const p of [alice, bob]) {
      await p.openEntity("feature", FEATURE);
      await p.expectChannel(ROOM.label);
      await p.expectSessionAt(DECISIONS.drafting);
    }

    // Nothing moved the session yet, so its decisions are in the header.
    await alice.decide(DECISIONS.containedLabel);
    await alice.post("Ready for review", DECISIONS.noteLabel);
    for (const p of [alice, bob]) await p.expectSessionAt(DECISIONS.review);

    // The lead may decide, once, on the message that asked; the header
    // points there (TKT-A0RTC52). The member may not, so sees no button.
    for (const label of [DECISIONS.approveLabel, DECISIONS.reviseLabel]) {
      await alice.expectDecisionOnMessage("Ready for review", label, true);
      await alice.expectDecision(label, false);
      await bob.expectDecision(label, false);
      await bob.expectDecisionOnMessage("Ready for review", label, false);
    }
    await alice.expectGoToDecision(true);
    await bob.expectGoToDecision(false);
    await alice.goToDecision();
    await alice.expectDecisionFocused("Ready for review", DECISIONS.approveLabel);
    // Every type the lead may post is a button now, or Text.
    await alice.expectMessageType(DECISIONS.reviseLabel, false);

    // Revise has a required field, so it opens the form.
    await alice.decideOnMessage("Ready for review", DECISIONS.reviseLabel);
    await alice.expectComposeStrip(DECISIONS.reviseLabel);
    await alice.post("Add the rollback plan", DECISIONS.feedbackLabel);
    for (const p of [alice, bob]) await p.expectSessionAt(DECISIONS.drafting);
    await alice.expectDecision(DECISIONS.approveLabel, false);

    await alice.decideOnMessage("Add the rollback plan", DECISIONS.containedLabel);
    await alice.post("Ready again", DECISIONS.noteLabel);
    for (const p of [alice, bob]) await p.expectSessionAt(DECISIONS.review);

    // Approve ends the session, so it asks first.
    await alice.decideOnMessage("Ready again", DECISIONS.approveLabel);
    await alice.confirmDialog(DECISIONS.approveLabel);
    const notice = `Session moved from ${DECISIONS.review} to ${DECISIONS.approved}`;
    for (const p of [alice, bob]) {
      await p.expectMessage(notice);
      await p.expectSessionAt(DECISIONS.approved);
    }
    await alice.expectDecision(DECISIONS.approveLabel, false);
  });
});

// The decisions room where Revise is a preset that opens the composer with
// the topic fixed (TKT-53FN0L).
const composeDecisionTest = base.extend<ChannelFixtures>({
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    writeRoomProject(testProject, { decisions: true, composePreset: true });
    await serveWithPrincipalHeader(serverBinary, testProject, testInfo, use, CHANNEL_USERS.lead);
  },
  as: asFixture,
});

composeDecisionTest.describe("compose decisions", () => {
  composeDecisionTest.describe.configure({ timeout: 120_000 });

  composeDecisionTest("a preset that needs text opens the composer", async ({ as }) => {
    const alice = await as(CHANNEL_USERS.lead);
    const bob = await as(CHANNEL_USERS.member);
    for (const p of [alice, bob]) {
      await p.openEntity("feature", FEATURE);
      await p.expectChannel(ROOM.label);
      await p.expectSessionAt(DECISIONS.drafting);
    }

    await alice.decide(DECISIONS.containedLabel);
    await alice.post("Ready for review", DECISIONS.noteLabel);
    for (const p of [alice, bob]) await p.expectSessionAt(DECISIONS.review);
    await alice.expectDecisionOnMessage("Ready for review", DECISIONS.requestChangesLabel, true);
    await bob.expectDecisionOnMessage("Ready for review", DECISIONS.requestChangesLabel, false);

    // The topic is fixed, so it shows in the strip and has no field; the
    // focus goes to the empty required Feedback.
    await alice.decideOnMessage("Ready for review", DECISIONS.requestChangesLabel);
    await alice.expectComposeStrip(DECISIONS.requestChangesLabel);
    await alice.expectComposeStrip(`${DECISIONS.topicLabel}: ${DECISIONS.topicValue}`);
    await alice.expectNoField(DECISIONS.topicLabel);
    await alice.expectFieldFocused(DECISIONS.feedbackLabel);

    // Topic is required, so the session moves only when the Send carries
    // the preset.
    await alice.post("Add the rollback plan", DECISIONS.feedbackLabel);
    for (const p of [alice, bob]) {
      await p.expectSessionAt(DECISIONS.drafting);
      await p.expectMessage("Add the rollback plan");
    }
  });
});

// The session header by width and display (TKT-56TPOTE). The header is its
// own size container; jsdom does not evaluate container queries, so the
// four combinations of display and width are pinned here.
sessionTest.describe("session header layout", () => {
  sessionTest.describe.configure({ timeout: 120_000 });

  sessionTest("without a display the list shows when wide and folds when narrow", async ({ as }) => {
    const alice = await as(CHANNEL_USERS.lead);
    await alice.openEntity("feature", FEATURE);
    await alice.expectChannel(ROOM.label);
    await alice.expectSessionAt(SESSION.from);

    await alice.setSessionWidth(60);
    await expect(alice.sessionSteps).toBeVisible();
    await expect(alice.sessionSummary).toBeHidden();
    await expect(alice.sessionToggle).toBeHidden();
    await expect(alice.sessionContext).toHaveCount(0);

    await alice.setSessionWidth(24);
    await expect(alice.sessionSummary).toHaveText(`Session · ${SESSION.from}`);
    await expect(alice.sessionToggle).toBeVisible();
    await expect(alice.sessionSteps).toBeHidden();
    await alice.sessionToggle.click();
    await expect(alice.sessionSteps).toBeVisible();
    await expect(alice.sessionSteps).toHaveCSS("flex-direction", "column");
  });
});

const compactTest = base.extend<ChannelFixtures>({
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    writeRoomProject(testProject, { decisions: true });
    addCompactSession(testProject);
    await serveWithPrincipalHeader(serverBinary, testProject, testInfo, use, CHANNEL_USERS.lead);
  },
  as: asFixture,
});

compactTest.describe("compact session header", () => {
  compactTest.describe.configure({ timeout: 120_000 });

  compactTest("shows the previous, current and next states, and the full list on request", async ({ as }) => {
    const alice = await as(CHANNEL_USERS.lead);
    await alice.openEntity("feature", FEATURE);
    await alice.expectChannel(ROOM.label);
    await alice.setSessionWidth(60);

    // A session that never moved has no previous state.
    await alice.expectSessionContext([`Current state: ${DECISIONS.drafting}`, `Next state: ${DECISIONS.review}`]);
    await alice.expectSessionAt(DECISIONS.drafting);
    await expect(alice.sessionSteps).toBeHidden();
    await expect(alice.sessionSummary).toBeHidden();
    await expect(alice.sessionToggle).toHaveText("Show all states");

    // The move stores Drafting as the previous state. Review leads back to
    // Drafting and on to Approved, in declaration order.
    const inReview = [
      `Previous state: ${DECISIONS.drafting}`,
      `Current state: ${DECISIONS.review}`,
      `Next state: ${DECISIONS.drafting}`,
      `Next state: ${DECISIONS.approved}`,
    ];
    await alice.decide(DECISIONS.containedLabel);
    await alice.post("Ready for review", DECISIONS.noteLabel);
    await alice.expectSessionContext(inReview);
    // The decisions sit on the message; the compact header points there.
    await alice.expectGoToDecision(true);

    // Unfolding shows the full list on its own row and removes the short
    // one, so one item carries aria-current.
    await alice.sessionToggle.click();
    await expect(alice.sessionToggle).toHaveAttribute("aria-expanded", "true");
    await expect(alice.sessionSteps).toBeVisible();
    await expect(alice.sessionContext).toHaveCount(0);
    await alice.expectSessionAt(DECISIONS.review);
    await expect(alice.sessionSteps).toHaveCSS("flex-direction", "row");
    await alice.sessionToggle.click();
    await expect(alice.sessionContext).toBeVisible();

    // A reload reads the previous state from the store.
    await alice.openEntity("feature", FEATURE);
    await alice.expectChannel(ROOM.label);
    await alice.setSessionWidth(60);
    await alice.expectSessionContext(inReview);

    // Below 32rem the compact header folds like any other.
    await alice.setSessionWidth(24);
    await expect(alice.sessionContext).toBeHidden();
    await expect(alice.sessionSummary).toHaveText(`Session · ${DECISIONS.review}`);
    await expect(alice.sessionSteps).toBeHidden();
    await alice.sessionToggle.click();
    await expect(alice.sessionSteps).toBeVisible();
    await expect(alice.sessionSteps).toHaveCSS("flex-direction", "column");
  });
});

// A room where a member asks with options and the lead answers with a
// click (TKT-03FQPD).
const choiceTest = base.extend<ChannelFixtures>({
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    writeRoomProject(testProject, { choices: true });
    await serveWithPrincipalHeader(serverBinary, testProject, testInfo, use, CHANNEL_USERS.lead);
  },
  as: asFixture,
});

choiceTest.describe("choices", () => {
  choiceTest.describe.configure({ timeout: 120_000 });

  choiceTest("a click on an option posts the answer", async ({ as }) => {
    const alice = await as(CHANNEL_USERS.lead);
    const bob = await as(CHANNEL_USERS.member);
    for (const p of [alice, bob]) {
      await p.openEntity("feature", FEATURE);
      await p.expectChannel(ROOM.label);
    }

    const question = "When do we ship?";
    const options = ["Friday", "After the audit"];
    await bob.postOverApi("features", FEATURE, ROOM.type, CHOICES.askType, {
      question,
      [CHOICES.optionsProperty]: options,
    });

    // The lead may answer, so sees a button per option and Other. The
    // member may not, so sees the options as a list.
    for (const label of [...options, CHOICES.other]) await alice.expectChoice(question, label, true);
    await bob.expectMessage(question);
    await bob.expectChoice(question, options[0], false);
    await bob.expectListedOptions(question, options);

    await alice.choose(question, options[1]);
    for (const p of [alice, bob]) await p.expectReply(question, options[1]);
    // Answered: the buttons are gone, and the options show as a list.
    await alice.expectChoice(question, options[0], false);
    await alice.expectListedOptions(question, options);

    // Other opens the composer with the answer type and the reply set.
    const second = "Who reviews the release notes?";
    await bob.postOverApi("features", FEATURE, ROOM.type, CHOICES.askType, {
      question: second,
      [CHOICES.optionsProperty]: ["Ann", "Bo"],
    });
    await alice.choose(second, CHOICES.other);
    await alice.post("Both of them", CHOICES.answerLabel);
    for (const p of [alice, bob]) await p.expectReply(second, "Both of them");
    await alice.expectChoice(second, "Ann", false);
  });
});

// A room where a member, standing in for an agent, asks and plans, and the
// feature's owner answers from the inbox (TKT-V05706X).
const inboxTest = base.extend<ChannelFixtures>({
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    writeRoomProject(testProject, { inbox: true });
    addFeatureOwner(testProject);
    setFeatureOwner(testProject, FEATURE, CHANNEL_USERS.lead);
    await serveWithPrincipalHeader(serverBinary, testProject, testInfo, use, CHANNEL_USERS.lead);
  },
  as: asFixture,
});

inboxTest.describe("answering from the inbox", () => {
  inboxTest.describe.configure({ timeout: 120_000 });

  inboxTest("the owner answers questions and approves a plan from the inbox", async ({ as }) => {
    const alice = await as(CHANNEL_USERS.lead);
    const bob = await as(CHANNEL_USERS.member);
    await bob.openEntity("feature", FEATURE);
    await bob.expectChannel(ROOM.label);
    await alice.openHome();
    await alice.openInbox();
    await alice.expectNothingWaiting();

    // A question with options waits on the lead, who answers with a click.
    const question = "When do we ship?";
    await bob.postOverApi("features", FEATURE, ROOM.type, INBOX.askType, {
      question,
      [INBOX.optionsProperty]: ["Friday", "After the audit"],
    });
    await alice.expectWaitingRow(question, true);

    // A user who cannot read the feature sees nothing waiting.
    const dave = await as(CHANNEL_USERS.outsider);
    await dave.openHome();
    await dave.openInbox();
    await dave.expectNothingWaiting();

    await alice.chooseInInbox(question, "After the audit");
    await alice.expectNothingWaiting();

    // Other answer opens the channel in a drawer; the send closes it.
    const second = "Who reviews the release notes?";
    await bob.postOverApi("features", FEATURE, ROOM.type, INBOX.askType, {
      question: second,
      [INBOX.optionsProperty]: ["Ann", "Bo"],
    });
    await alice.expectWaitingRow(second, true);
    await alice.chooseInInbox(second, INBOX.other);
    await alice.postInDrawer("Both of them", INBOX.answerLabel);
    await alice.expectDrawerClosed();
    await alice.expectNothingWaiting();

    // A plan is approved from the row. Approve ends the session, so it asks.
    const plan = "Ship behind a flag first";
    await bob.postOverApi("features", FEATURE, ROOM.type, INBOX.planType, { plan });
    await alice.expectWaitingRow(plan, true);
    await alice.decideInInbox(plan, INBOX.approveLabel);
    await alice.confirmDialog(INBOX.approveLabel);
    await alice.expectNothingWaiting();

    // The answers are in the channel, as replies to their questions.
    await bob.expectReply(question, "After the audit");
    await bob.expectReply(second, "Both of them");
    await bob.expectSessionAt(INBOX.approved);
  });
});

// A room whose agent types people do not post (`offer: false`, TKT-WC63SNN).
const offerFalseTest = base.extend<ChannelFixtures>({
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    writeRoomProject(testProject, { quiet: true });
    addOfferFalse(testProject);
    addFeatureOwner(testProject);
    setFeatureOwner(testProject, FEATURE, CHANNEL_USERS.lead);
    await serveWithPrincipalHeader(serverBinary, testProject, testInfo, use, CHANNEL_USERS.lead);
  },
  as: asFixture,
});

offerFalseTest.describe("message types people do not post", () => {
  offerFalseTest.describe.configure({ timeout: 120_000 });

  offerFalseTest("the web app offers no way to post them; the API still does and they show", async ({ as }) => {
    const alice = await as(CHANNEL_USERS.lead);
    const bob = await as(CHANNEL_USERS.member);
    await alice.openEntity("feature", FEATURE);
    await alice.expectChannel(ROOM.label);
    await bob.openEntity("feature", FEATURE);
    await bob.expectChannel(ROOM.label);

    // Step moves the session but has offer: false, so it has no decision
    // button; Plan keeps one. Progress is not in the composer.
    await bob.expectComposer();
    await bob.expectDecision(QUIET.planLabel, true);
    await bob.expectDecision(QUIET.stepLabel, false);
    await bob.expectMessageType(QUIET.progressLabel, false);
    await bob.expectMessageType(QUIET.stepLabel, false);

    await bob.postOverApi("features", FEATURE, ROOM.type, QUIET.progressType, { body: "Reading the code." });
    await alice.expectMessage("Reading the code.");
    await bob.expectMessage("Reading the code.");
    await bob.postOverApi("features", FEATURE, ROOM.type, QUIET.stepType, { body: "Start building." });
    await bob.expectMessage("Start building.");
    await bob.expectSessionAt(QUIET.building);
  });
});

// A room where a member, standing in for an agent, posts quiet progress and
// moves the session between working states (TKT-LKIVLR2).
const quietTest = base.extend<ChannelFixtures>({
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    writeRoomProject(testProject, { quiet: true });
    addQuietRun(testProject);
    addFeatureOwner(testProject);
    setFeatureOwner(testProject, FEATURE, CHANNEL_USERS.lead);
    await serveWithPrincipalHeader(serverBinary, testProject, testInfo, use, CHANNEL_USERS.lead);
  },
  as: asFixture,
});

quietTest.describe("quiet messages across session notices", () => {
  quietTest.describe.configure({ timeout: 120_000 });

  quietTest("progress and working moves fold into one line; a wait stays in view", async ({ as }) => {
    const alice = await as(CHANNEL_USERS.lead);
    const bob = await as(CHANNEL_USERS.member);
    await alice.openEntity("feature", FEATURE);
    await alice.expectChannel(ROOM.label);
    await bob.openEntity("feature", FEATURE);
    await bob.expectChannel(ROOM.label);
    const post = (type: string, body: string) =>
      bob.postOverApi("features", FEATURE, ROOM.type, type, { body });
    await post("text", "Please pick this up.");
    await alice.expectMessage("Please pick this up.");
    await post(QUIET.progressType, "Reading the code.");
    await post(QUIET.stepType, "Start building.");
    await post(QUIET.progressType, "Writing the fold.");
    await post(QUIET.stepType, "Back to planning.");
    await post(QUIET.progressType, "Planning the tests.");

    const moved = (from: string, to: string) => `Session moved from ${from} to ${to}`;
    const fold = `5 messages, 2 session steps: ${QUIET.progressLabel}, ${QUIET.stepLabel}`;
    await alice.expectFold(fold);
    await alice.expectFoldCount(1);
    // In Planning, a working state, the owner's decision is in the header,
    // so the step that caused it stays folded.
    await alice.expectDecision(QUIET.approveLabel, true);
    await alice.expectGoToDecision(false);
    await alice.expectNotice(moved(QUIET.planning, QUIET.building), false);
    await alice.toggleFold(fold, true);
    await alice.expectNotice(moved(QUIET.planning, QUIET.building), true);
    await alice.expectNotice(moved(QUIET.building, QUIET.planning), true);
    await alice.toggleFold(fold, false);

    // The plan moves the session into a state that waits for the owner: that
    // move and the plan, with its decision, stay in view after the fold.
    const plan = "Ship behind a flag first";
    await bob.postOverApi("features", FEATURE, ROOM.type, QUIET.planType, { plan });
    await alice.expectNotice(moved(QUIET.planning, QUIET.review), true);
    await alice.expectDecisionOnMessage(plan, QUIET.approveLabel, true);
    await alice.expectGoToDecision(true);
    await alice.expectFold(fold);

    // After the owner's decision, the move back to Planning stays in view
    // next to it, and new progress folds after it.
    await alice.decideOnMessage(plan, QUIET.approveLabel);
    await alice.expectSessionAt(QUIET.planning);
    await alice.expectNotice(moved(QUIET.review, QUIET.planning), true);
    await post(QUIET.progressType, "Picking the next step.");
    await alice.expectFold(`1 message: ${QUIET.progressLabel}`);
    await alice.expectNotice(moved(QUIET.review, QUIET.planning), true);
    await alice.expectFoldCount(2);
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

const placedTest = base.extend<ChannelFixtures>({
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    writeRoomProject(testProject);
    addRoomAboveRelations(testProject);
    await serveWithPrincipalHeader(serverBinary, testProject, testInfo, use, CHANNEL_USERS.lead);
  },
  as: asFixture,
});

// TKT-VXBE16H: `channels.room.placement: above_relations` moves the channel
// from the end of the page to before the first relation section.
placedTest.describe("channel placement", () => {
  placedTest.describe.configure({ timeout: 120_000 });

  placedTest("a channel placed above the relations sits before them", async ({ as }) => {
    const alice = await as(CHANNEL_USERS.lead);
    await alice.openEntity("feature", FEATURE);
    await alice.expectChannel(ROOM.label);

    // The generated view shows properties and the body, then one section per
    // relation. The one channels block sits between the body and the first
    // relation section.
    await alice.expectPanelBetweenSections("Implement user authentication system.");

    // It still works where it sits.
    await alice.post("Placed above the relations");
    await alice.expectMessage("Placed above the relations");
  });
});
