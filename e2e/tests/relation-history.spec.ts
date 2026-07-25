import {
  postgresTest as test,
  expect,
  POSTGRES_E2E_ENABLED,
  type ApiHelpers,
} from "./fixtures";
import { RelationHistoryPage } from "../pages/relation-history.page";
import { RelationCardsPage } from "../pages/relation-cards.page";

// Relation content history is a pgstore-only capability, so this suite runs
// against `rela-server-postgres` (the postgresTest fixture) and SKIPS when
// RELA_E2E_DATABASE_URL is unset — the default fsstore e2e run has no database.
//
// The postgres backend starts EMPTY (entities live in the DB, not the fixture's
// markdown seed, which only the fsstore server reads), so each test creates the
// entities it needs via the API. `feature` uses a sequential id, so we let the
// server assign the id and use the returned value.
test.describe("relation history UI", () => {
  test.skip(
    !POSTGRES_E2E_ENABLED,
    "requires a postgres backend (set RELA_E2E_DATABASE_URL)",
  );

  // The e2e postgres server runs the version sweep on a short interval (set via
  // RELA_VERSION_SWEEP_INTERVAL/_IDLE) so create/update versions of an edited
  // relation settle within seconds instead of the 5-minute production default.

  async function newFeature(api: ApiHelpers, title: string): Promise<string> {
    const e = await api.createEntity("features", {
      prefix: "FEAT",
      properties: { title, status: "draft" },
    });
    return e.id;
  }

  test("timeline, diff, and restore of an outgoing relation's history", async ({
    appPage,
    api,
  }) => {
    const from = await newFeature(api, "History source");
    const to = await newFeature(api, "History target");

    // Create a blocks edge from --blocks--> to, then edit its `reason` — each
    // edit must SETTLE (stay untouched past the sweep's idle window) before the
    // next so the reconciliation sweep captures it as a DISTINCT version rather
    // than coalescing them. `waitForRelationVersions` polls the API until the
    // count reaches the target, so it's robust to sweep cadence.
    await api.setRelationMeta("features", from, "blocks", "feature", to, {
      reason: "initial reason",
    });
    await api.waitForRelationVersions("features", from, "blocks", to, 1);
    await api.setRelationMeta("features", from, "blocks", "feature", to, {
      reason: "second reason",
    });
    await api.waitForRelationVersions("features", from, "blocks", to, 2);

    const cards = new RelationCardsPage(appPage);
    const history = new RelationHistoryPage(appPage);

    await cards.navigateToEdit("feature", from);
    await history.openHistoryForCard(to);

    // At least the two captured versions render.
    await expect
      .poll(async () => history.timelineCount(), { timeout: 20_000 })
      .toBeGreaterThanOrEqual(2);

    const n = await history.timelineCount();

    // Diff the oldest against the newest — the `reason` property changed, so the
    // property diff must list it.
    await history.compare(1, n);
    await expect(history.propDiffRows().first()).toBeVisible();
    // The diff pane renders the humanized property label ("Reason"); match
    // case-insensitively so a label-formatting tweak doesn't break the test.
    const labels = (await history.propDiffLabels()).map((l) => l.toLowerCase());
    expect(labels).toContain("reason");

    // Restore the oldest version through the entitymanager (authorized, audited).
    // The restore is a normal write, so its new version is captured by the sweep
    // asynchronously — wait for the API to reflect it, then re-open the view to
    // confirm the extra version renders in the timeline.
    await history.restoreVersion(1);
    await api.waitForRelationVersions("features", from, "blocks", to, n + 1);
    await cards.navigateToEdit("feature", from);
    await history.openHistoryForCard(to);
    await expect
      .poll(async () => history.timelineCount(), { timeout: 20_000 })
      .toBeGreaterThan(n);
  });

  test("History affordance is present on outgoing cards, absent on incoming", async ({
    appPage,
    api,
  }) => {
    const from = await newFeature(api, "Affordance source");
    const to = await newFeature(api, "Affordance target");

    // Outgoing blocks edge (from owns history) + an incoming implements edge from
    // a task (owned by the OTHER endpoint — must have no history affordance).
    await api.setRelationMeta("features", from, "blocks", "feature", to, {
      reason: "x",
    });
    const task = await api.createEntity("tasks", {
      prefix: "TASK",
      properties: { title: "Implementer" },
      relations: { implements: { data: [{ type: "feature", id: from }] } },
    });

    const cards = new RelationCardsPage(appPage);
    const history = new RelationHistoryPage(appPage);
    await cards.navigateToEdit("feature", from);

    // Outgoing blocks card → History button present.
    await expect(history.historyButtonForCard(to)).toBeVisible();

    // Incoming implements card (the task) → no History button on that card.
    await expect(history.historyButtonForCard(task.id)).toHaveCount(0);
  });
});
