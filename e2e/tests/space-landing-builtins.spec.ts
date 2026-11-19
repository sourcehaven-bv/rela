import fs from "node:fs";
import path from "node:path";
import { test as base, expect } from "./fixtures";
import { SpacesPage } from "../pages";

/**
 * TKT-JEAR2UA: the app root opens the current space's home, and `builtins:`
 * hides the pinned sidebar rows per space.
 *
 * The shared project's top-level navigation is rewritten into two spaces
 * before the server starts, as in spaces.spec.ts.
 */
const SPACES_YAML = `spaces:
  - id: staff
    label: "Staff"
    home:
      list: features
    navigation:
      - label: "Features"
        list: features
  - id: portal
    label: "Portal"
    home:
      list: bugs
    builtins:
      search: {hidden: true}
      flow_tasks: {hidden: true}
      flow_runs: {hidden: true}
      analysis: {hidden: true}
    navigation:
      - label: "Bugs"
        list: bugs
`;

const test = base.extend({
  testProject: async ({ testProject }, use) => {
    const file = path.join(testProject, "data-entry.yaml");
    const yaml = fs.readFileSync(file, "utf8");
    const at = yaml.indexOf("\nnavigation:\n");
    if (at < 0) throw new Error("fixture data-entry.yaml has no navigation:");
    fs.writeFileSync(file, yaml.slice(0, at + 1) + SPACES_YAML);
    await use(testProject);
  },
});

test.describe("Space landing and built-in rows", () => {
  test("the root opens the first space's home", async ({ appPage }) => {
    await expect(appPage).toHaveURL(/\/s\/staff\/list\/features$/);
  });

  test("the root opens the home of the space last used", async ({ appPage, serverUrl }) => {
    const spaces = new SpacesPage(appPage);
    await appPage.goto(`${serverUrl}/s/portal/list/bugs`);
    await expect(spaces.sidebarLink("Bugs")).toBeVisible();

    await appPage.goto(`${serverUrl}/`);
    await expect(appPage).toHaveURL(/\/s\/portal\/list\/bugs$/);
  });

  test("/dashboard still opens the dashboard", async ({ appPage, serverUrl }) => {
    await appPage.goto(`${serverUrl}/dashboard`);
    await expect(appPage).toHaveURL(/\/s\/staff\/dashboard$/);
  });

  test("a space without builtins keeps every pinned row", async ({ appPage, serverUrl }) => {
    const spaces = new SpacesPage(appPage);
    await appPage.goto(`${serverUrl}/s/staff/list/features`);
    await expect(spaces.pinnedRows.first()).toHaveAttribute("href", /\/s\/staff\/search$/);
    await expect(spaces.pinnedRows.last()).toHaveAttribute("href", /\/s\/staff\/analyze$/);
  });

  test("a space whose builtins hide every row has no pinned rows", async ({ appPage, serverUrl }) => {
    const spaces = new SpacesPage(appPage);
    await appPage.goto(`${serverUrl}/s/portal/list/bugs`);
    await expect(spaces.sidebarLink("Bugs")).toBeVisible();
    await expect(spaces.pinnedRows).toHaveCount(0);
  });
});

base.describe("Root without spaces", () => {
  base("the root opens the dashboard", async ({ appPage }) => {
    await expect(appPage).toHaveURL(/\/dashboard$/);
  });
});
