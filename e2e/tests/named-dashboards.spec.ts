import fs from "node:fs";
import path from "node:path";
import { test as base, expect } from "./fixtures";
import { DashboardPage } from "../pages";

/**
 * Named dashboards (TKT-5LQNLY): `dashboards:` beside the default
 * `dashboard:`, opened by `dashboard: <name>` from a navigation entry or a
 * space home, at /dashboard/<name>.
 *
 * The spec rewrites the shared project's navigation into two spaces, one of
 * which opens on a named dashboard, before the server starts.
 */
const NAMED_YAML = `dashboards:
  triage:
    title: "Bug triage"
    cards:
      - title: "Critical Bugs"
        query: "type:bug prop:severity=critical"
        display: count
      - title: "Open Tasks"
        query: "type:task"
        display: count
spaces:
  - id: product
    label: "Product"
    navigation:
      - label: "Overview"
        dashboard: true
      - label: "Features"
        list: features
  - id: support
    label: "Support"
    home:
      dashboard: triage
    navigation:
      - label: "Triage"
        dashboard: triage
      - label: "Bugs"
        list: bugs
`;

const test = base.extend({
  testProject: async ({ testProject }, use) => {
    const file = path.join(testProject, "data-entry.yaml");
    const yaml = fs.readFileSync(file, "utf8");
    const at = yaml.indexOf("\nnavigation:\n");
    if (at < 0) throw new Error("fixture data-entry.yaml has no navigation:");
    fs.writeFileSync(file, yaml.slice(0, at + 1) + NAMED_YAML);
    await use(testProject);
  },
});

test.describe("Named dashboards", () => {
  test("a space opens on its named dashboard home", async ({ appPage, serverUrl }) => {
    const dashboard = new DashboardPage(appPage);
    await appPage.goto(`${serverUrl}/s/support`);
    await expect(appPage).toHaveURL(/\/s\/support\/dashboard\/triage$/);
    await dashboard.expectTitle("Bug triage");
    await dashboard.expectCardTitles(["Critical Bugs", "Open Tasks"]);
    await dashboard.expectSidebarActive("Triage");
    await dashboard.expectSidebarInactive("Bugs");
  });

  test("a navigation entry links the named dashboard in its space", async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigateNamed("triage", "support");
    await expect(dashboard.sidebarLink("Triage")).toHaveAttribute(
      "href",
      /\/s\/support\/dashboard\/triage$/,
    );
    await dashboard.sidebarLink("Bugs").click();
    await expect(appPage).toHaveURL(/\/s\/support\/list\/bugs$/);
    await dashboard.sidebarLink("Triage").click();
    await expect(appPage).toHaveURL(/\/s\/support\/dashboard\/triage$/);
    await dashboard.expectTitle("Bug triage");
  });

  test("the default dashboard is unchanged beside a named one", async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigateTo("/s/product/dashboard");
    await dashboard.expectTitle("Dashboard");
    expect(await dashboard.getStatValue("Open Features")).toMatch(/\d+/);
  });

  test("an unknown name shows a not-found state", async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigateNamed("nosuch", "product");
    await dashboard.expectNotFound("nosuch");
  });
});
