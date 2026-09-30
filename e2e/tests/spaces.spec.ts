import fs from "node:fs";
import path from "node:path";
import { test as base, expect } from "./fixtures";
import { SpacesPage } from "../pages";

/**
 * Spaces (TKT-GNKR5H): named entry points onto one graph, each with its own
 * sidebar and Create menu, carried in the URL as /s/<id>.
 *
 * `spaces:` and a top-level `navigation:` are mutually exclusive, so this spec
 * rewrites the shared project's navigation into two spaces before the server
 * starts.
 */
const SPACES_YAML = `spaces:
  - id: product
    label: "Product"
    icon: box
    home:
      list: features
    create: [feature]
    navigation:
      - label: "Features"
        list: features
      - label: "Feature Board"
        kanban: feature-board
  - id: support
    label: "Support"
    home:
      list: bugs
    create: [bug, task]
    navigation:
      - label: "Bugs"
        list: bugs
      - label: "Tasks"
        list: tasks
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

test.describe("Spaces", () => {
  test("an unprefixed URL opens in the first space", async ({ appPage }) => {
    const spaces = new SpacesPage(appPage);
    await expect(appPage).toHaveURL(/\/s\/product(\/|$)/);
    await expect(spaces.sidebarLink("Features")).toBeVisible();
    await expect(spaces.sidebarLink("Bugs")).toHaveCount(0);
  });

  test("the switcher moves to another space and its navigation", async ({ appPage }) => {
    const spaces = new SpacesPage(appPage);
    await spaces.switchSpace(/Product/, "Support");

    await expect(appPage).toHaveURL(/\/s\/support\/list\/bugs$/);
    await expect(spaces.sidebarLink("Bugs")).toBeVisible();
    await expect(spaces.sidebarLink("Features")).toHaveCount(0);
    await expect(spaces.sidebarLink("Tasks")).toHaveAttribute(
      "href",
      /\/s\/support\/list\/tasks$/,
    );
  });

  test("a deep link keeps its space across a reload", async ({ appPage, serverUrl }) => {
    const spaces = new SpacesPage(appPage);
    await appPage.goto(`${serverUrl}/s/support/list/tasks`);
    await expect(spaces.sidebarLink("Tasks")).toBeVisible();
    await appPage.reload();
    await expect(appPage).toHaveURL(/\/s\/support\/list\/tasks$/);
  });

  test("an unknown space falls back to the first", async ({ appPage, serverUrl }) => {
    await appPage.goto(`${serverUrl}/s/nosuch/list/bugs`);
    await expect(appPage).toHaveURL(/\/s\/product\/list\/bugs$/);
  });

  test("the Create menu offers the space's types", async ({ appPage, serverUrl }) => {
    const spaces = new SpacesPage(appPage);
    await appPage.goto(`${serverUrl}/s/support/list/bugs`);
    await spaces.openCreateMenu();
    await expect(spaces.menuItems).toHaveText([/Bug/, /Task/]);
  });
});
