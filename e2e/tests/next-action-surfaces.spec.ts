import fs from "node:fs";
import path from "node:path";
import { test as base, expect, SEED } from "./fixtures";
import { NextActionPage } from "../pages";

/**
 * Where suggestions appear (TKT-6EEXXE). The card is mounted once above every
 * page; the server applies the operator's placement for the page the SPA
 * names, and resolves context sources for the entity it shows.
 */
const NEXT_ACTIONS_YAML = `next_action_bands:
  - id: focus
    prominence: banner
next_action_surfaces:
  search: false
next_actions:
  features-to-review:
    band: focus
    query: "type:feature"
    suggest: "Review {title}"
    visible_on: ["list", "kanban"]
  about-this-feature:
    band: focus
    context: feature
    suggest: "This one: {title}"
`;

const test = base.extend({
  testProject: async ({ testProject }, use) => {
    const file = path.join(testProject, "data-entry.yaml");
    const yaml = fs.readFileSync(file, "utf8");
    if (!yaml.includes("\nnavigation:\n")) throw new Error("fixture YAML has no navigation:");
    fs.writeFileSync(file, yaml.replace("\nnavigation:\n", `\n${NEXT_ACTIONS_YAML}\nnavigation:\n`));
    await use(testProject);
  },
});

test.describe("Next-action surfaces", () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test("a placed source shows on a list and above a full-height board", async ({ appPage, serverUrl }) => {
    const na = new NextActionPage(appPage);
    await appPage.goto(`${serverUrl}/list/features`);
    await na.expectShowing(/^Review /);

    await appPage.goto(`${serverUrl}/kanban/feature-board`);
    await na.expectShowing(/^Review /);
    await na.expectAboveBoard();
  });

  // The list page reports the impression, which starts that suggestion's
  // cooldown. Without the SPA naming it as current, the board would pick
  // another feature.
  test("the suggestion stays put across a navigation", async ({ appPage, serverUrl }) => {
    const na = new NextActionPage(appPage);
    const shown = na.waitForImpression();
    await appPage.goto(`${serverUrl}/list/features`);
    await na.expectShowing(/^Review /);
    await shown;
    const first = await na.entityId();

    // An in-app navigation: a page load would start the SPA afresh, with no
    // suggestion on screen to keep.
    const asked = na.waitForRequest("kanban");
    await na.clickNavLink("Feature Board");
    await asked;
    await na.expectShowing(/^Review /);
    expect(await na.entityId()).toBe(first);
  });

  test("a context source follows the viewed entity", async ({ appPage, serverUrl }) => {
    const na = new NextActionPage(appPage);
    await appPage.goto(`${serverUrl}/entity/feature/${SEED.features.authentication}`);
    await na.expectShowing(/^This one: /);
    expect(await na.entityId()).toBe(SEED.features.authentication);

    await appPage.goto(`${serverUrl}/entity/feature/${SEED.features.exportData}`);
    await expect.poll(() => na.entityId()).toBe(SEED.features.exportData);
  });

  test("nothing shows where the operator placed nothing", async ({ appPage, serverUrl }) => {
    const na = new NextActionPage(appPage);
    // The dashboard is not in visible_on, and has no context source.
    let asked = na.waitForRequest("dashboard");
    await appPage.goto(`${serverUrl}/dashboard`);
    await na.expectNoneAfter(asked);

    // Search is switched off for every source.
    asked = na.waitForRequest("search");
    await appPage.goto(`${serverUrl}/search`);
    await na.expectNoneAfter(asked);
  });
});
