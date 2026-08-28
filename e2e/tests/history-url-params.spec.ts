import {
  postgresTest as test,
  expect,
  POSTGRES_E2E_ENABLED,
  type ApiHelpers,
} from "./fixtures";
import { HistoryPage } from "../pages/history.page";
import { RelationHistoryPage } from "../pages/relation-history.page";

// The compared-version pair lives in `?base=`/`?target=` so a specific diff can
// be shared, bookmarked, and survive a reload (TKT-YOHC3N).
//
// Version history is a pgstore-only capability, so this suite runs against
// `rela-server-postgres` and SKIPS when RELA_E2E_DATABASE_URL is unset. The
// postgres backend starts EMPTY, so each test creates the entities it needs.
test.describe("history view URL params", () => {
  test.skip(
    !POSTGRES_E2E_ENABLED,
    "requires a postgres backend (set RELA_E2E_DATABASE_URL)",
  );

  /** Create a feature and edit it until the sweep has captured `versions`
   *  distinct versions. Each edit must settle before the next, or the
   *  reconciliation sweep coalesces them into one version. */
  async function featureWithHistory(
    api: ApiHelpers,
    title: string,
    versions: number,
  ): Promise<string> {
    const e = await api.createEntity("features", {
      prefix: "FEAT",
      properties: { title, status: "draft" },
    });
    await api.waitForEntityVersions("feature", e.id, 1);
    for (let i = 2; i <= versions; i++) {
      await api.updateEntity("features", e.id, { title: `${title} v${i}` });
      await api.waitForEntityVersions("feature", e.id, i);
    }
    return e.id;
  }

  test("deep link selects the pair, and a bare URL publishes its defaults", async ({
    appPage,
    api,
  }) => {
    const id = await featureWithHistory(api, "Deep link", 3);
    const history = new HistoryPage(appPage);

    // A bare URL keeps the existing default (newest → current) and writes it
    // back, so the address bar becomes an explicit, shareable link.
    await history.open("feature", id);
    await history.waitForTimelineAtLeast(3);
    await history.expectUrlSelection(3, "current");
    expect(await history.selectedPair()).toEqual({ base: "3", target: "current" });

    // A deep link selects the pair it names, with no user interaction. Reading
    // the SELECT values (not just the URL) is the assertion that catches a
    // v-model type mismatch — a numeric side parsed as a string would leave the
    // dropdown blank while the URL still looked correct.
    await history.open("feature", id, { base: 1, target: 2 });
    await history.waitForTimelineAtLeast(3);
    expect(await history.selectedPair()).toEqual({ base: "1", target: "2" });
    await expect(history.caption()).toHaveText(/v1.*v2/);
    // The timeline highlights the base side.
    await history.expectSelectedVersion(1);
  });

  test("selecting a pair writes the URL, and a reload reproduces it", async ({
    appPage,
    api,
  }) => {
    const id = await featureWithHistory(api, "Reload stable", 3);
    const history = new HistoryPage(appPage);

    await history.open("feature", id);
    await history.waitForTimelineAtLeast(3);

    // A non-default pair, chosen through the dropdowns.
    await history.compare(1, 3);
    await history.expectUrlSelection(1, 3);

    // Reloading reproduces the same diff rather than resetting to defaults.
    await history.reload();
    await history.waitForTimelineAtLeast(3);
    expect(await history.selectedPair()).toEqual({ base: "1", target: "3" });

    // A timeline click sets the base side and is reflected in the URL.
    await history.selectVersion(2);
    await history.expectUrlSelection(2, 3);

    // Swap reverses the pair.
    await history.swapSides();
    await history.expectUrlSelection(3, 2);
  });

  test("malformed params fall back to defaults without erroring", async ({
    appPage,
    api,
  }) => {
    const id = await featureWithHistory(api, "Bad params", 2);
    const history = new HistoryPage(appPage);

    // An out-of-range ordinal and a non-numeric value both degrade to the
    // view's defaults — no crash, no error state, no failed fetch.
    await history.navigateTo(`/history/feature/${id}?base=999&target=nonsense`);
    await history.expectLoaded();
    await history.waitForTimelineAtLeast(2);
    expect(await history.selectedPair()).toEqual({ base: "2", target: "current" });
    await history.expectNoError();
  });

  test("selection params merge with an existing query param", async ({
    appPage,
    api,
  }) => {
    const id = await featureWithHistory(api, "Query merge", 2);
    const history = new HistoryPage(appPage);

    // An unrelated param must survive the selection write — the history view
    // merges into route.query rather than replacing it.
    await history.navigateTo(`/history/feature/${id}?return_to=%2Flist%2Ffeatures`);
    await history.expectLoaded();
    await history.waitForTimelineAtLeast(2);

    await history.compare(1, 2);
    await history.expectUrlSelection(1, 2);
    expect(history.queryParam("return_to")).toBe("/list/features");
  });

  test("relation history mirrors its pair into the URL too", async ({
    appPage,
    api,
  }) => {
    const from = await featureWithHistory(api, "Rel URL source", 1);
    const to = await featureWithHistory(api, "Rel URL target", 1);

    await api.setRelationMeta("features", from, "blocks", "feature", to, {
      reason: "initial reason",
    });
    await api.waitForRelationVersions("feature", from, "blocks", to, 1);
    await api.setRelationMeta("features", from, "blocks", "feature", to, {
      reason: "second reason",
    });
    await api.waitForRelationVersions("feature", from, "blocks", to, 2);

    const history = new RelationHistoryPage(appPage);
    await history.navigateTo(
      `/relation-history/feature/${from}/blocks/${to}?base=1&target=2`,
    );
    await history.expectLoaded();
    await history.waitForTimelineAtLeast(2);

    // The deep-linked pair is selected, and the diff shows the changed property.
    expect(await history.selectedPair()).toEqual({ base: "1", target: "2" });
    const labels = (await history.propDiffLabels()).map((l) => l.toLowerCase());
    expect(labels).toContain("reason");

    // Changing the selection republishes to the URL.
    await history.compare(2, 2);
    await history.expectUrlSelection(2, 2);

    // A BARE relation URL publishes a live-relative default (`target=current`),
    // the same shape the entity view publishes — not two frozen ordinals, which
    // would make an otherwise-identical shared link freeze for relations only.
    await history.navigateTo(
      `/relation-history/feature/${from}/blocks/${to}`,
    );
    await history.expectLoaded();
    await history.waitForTimelineAtLeast(2);
    await history.expectUrlSelection(1, "current");
  });

  test("an out-of-range ordinal is corrected in the URL, not left to rot", async ({
    appPage,
    api,
  }) => {
    // `base=999` names no version that exists. The view falls back to its
    // default AND rewrites the address bar, so the corrected pair is what gets
    // copied or bookmarked — leaving the stale ordinal there would let it be
    // re-applied verbatim after more versions are captured.
    const id = await featureWithHistory(api, "Stale ordinal", 2);
    const history = new HistoryPage(appPage);

    await history.navigateTo(`/history/feature/${id}?base=999&target=nonsense`);
    await history.expectLoaded();
    await history.waitForTimelineAtLeast(2);
    await history.expectUrlSelection(2, "current");
    await history.expectNoError();
  });
});
