import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import { test as base } from "./fixtures";
import { REMOTE_SEED, REMOTE_USERS, writeRemotesProject } from "./remotes-project";
import { RemotesPage } from "../pages/remotes.page";

// The remotes page and a remote's conflict list (TKT-0FMYPEZ): the user finds
// the page from the footer, sees only the entries on rows they can read, and
// works the list with the actions each entry offers.

const test = base.extend({
  // eslint-disable-next-line no-empty-pattern
  testProject: async ({}, use) => {
    const dir = fs.mkdtempSync(path.join(fs.realpathSync(os.tmpdir()), "rela-e2e-remotes-"));
    try {
      writeRemotesProject(dir);
      await use(dir);
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  },
});

test.use({ serverEnv: { RELA_DATAENTRY_USER: REMOTE_USERS.syncer } });

test.describe("remotes", () => {
  test("lists the readable entries and resolves, dismisses and links them", async ({ appPage }) => {
    const remotes = new RemotesPage(appPage);
    const { remote, both, superseded, uncertain, hidden } = REMOTE_SEED;

    await remotes.openFromFooter();
    // The entry on the secret row is neither counted nor shown.
    await remotes.expectRemote(remote, 3);
    await remotes.openConflicts(remote);
    await remotes.expectEntries([both.id, superseded.id, uncertain.id]);
    await remotes.expectNoText(hidden.id);

    await remotes.expectEntryText(both.id, both.remote);
    await remotes.expectActions(both.id, { keep: true, take: true, dismiss: false, link: false });
    await remotes.expectNotKept(superseded.id);
    await remotes.expectActions(superseded.id, { keep: false, take: false, dismiss: true, link: false });
    await remotes.expectEntryText(uncertain.id, "Uncertain create");
    await remotes.expectActions(uncertain.id, { keep: true, take: false, dismiss: false, link: true });

    // Keeping the local side of an uncertain create asks first.
    await remotes.keepLocal(uncertain.id);
    await remotes.answerDialog("Cancel");
    await remotes.expectEntries([both.id, superseded.id, uncertain.id]);

    await remotes.keepLocal(both.id);
    await remotes.expectEntries([superseded.id, uncertain.id]);

    await remotes.dismiss(superseded.id);
    await remotes.expectEntries([uncertain.id]);

    await remotes.link(uncertain.id, "requirement/R-3");
    await remotes.expectEntries([]);
  });
});
