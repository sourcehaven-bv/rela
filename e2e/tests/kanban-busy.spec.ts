import { test as base, SEED, contextsAs, serveWithPrincipalHeader } from "./fixtures";
import { BUSY_BOARD, CHANNEL_USERS, ROOM, RUN, addBusyBoard, writeRoomProject } from "./realtime-project";
import { ChannelsPage } from "../pages/channels.page";
import { KanbanPage } from "../pages";

// The busy indicator (TKT-21G6JS), on the agent-run session of RUN. The
// session starts in Queued, which waits for the owner, so a new channel is
// not busy. The lead's Start message moves it to Planning, whose status is
// working: the board shows the feature as busy and the channel shows an
// activity line. The lead's Finish message ends the session, and both go.
//
// A board does not refresh on a session move without outputs yet
// (TKT-W5ZSYJ), so the spec reloads it.

const FEATURE = SEED.features.authentication;
const FEATURE_TITLE = "User Authentication";
const OTHER_TITLE = "Dashboard Analytics";

const test = base.extend({
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    writeRoomProject(testProject, { run: true });
    addBusyBoard(testProject);
    await serveWithPrincipalHeader(serverBinary, testProject, testInfo, use, CHANNEL_USERS.lead);
  },
});

test.describe("busy indicator", () => {
  test.describe.configure({ timeout: 120_000 });

  test("a card is busy while its session works, and the channel says so", async ({ browser, serverUrl }) => {
    const pages = contextsAs(browser, serverUrl);
    const page = await pages.as(CHANNEL_USERS.lead);
    const channels = new ChannelsPage(page);
    const kanban = new KanbanPage(page);

    await channels.openEntity("feature", FEATURE);
    await channels.expectChannel(ROOM.label);
    await channels.selectMessageType("Text");
    await channels.post("Looking into it");
    await channels.expectMessage("Looking into it");
    await channels.expectSessionAt(RUN.queued);
    await channels.expectActivityLine("Working…", false);
    await kanban.navigateToKanban(BUSY_BOARD.id);
    await kanban.expectNoCardBusy(FEATURE_TITLE);

    await channels.openEntity("feature", FEATURE);
    await channels.expectChannel(ROOM.label);
    await channels.selectMessageType(RUN.startLabel);
    await channels.post("Planning the change", RUN.noteLabel);
    await channels.expectSessionAt(RUN.planning);
    await channels.expectActivityLine("Working…", true);

    await kanban.navigateToKanban(BUSY_BOARD.id);
    await kanban.expectCardBusy(
      FEATURE_TITLE,
      new RegExp(`^${RUN.planning} · <1m$`),
      new RegExp(`^${ROOM.label}: ${RUN.planning}, for less than a minute$`),
    );
    await kanban.expectNoCardBusy(OTHER_TITLE);

    await channels.openEntity("feature", FEATURE);
    await channels.expectChannel(ROOM.label);
    await channels.selectMessageType(RUN.finishLabel);
    await channels.post("Plan written", RUN.noteLabel);
    await channels.expectSessionAt(RUN.done);
    await channels.expectActivityLine("Working…", false);

    await kanban.navigateToKanban(BUSY_BOARD.id);
    await kanban.expectNoCardBusy(FEATURE_TITLE);
    await pages.close();
  });
});
