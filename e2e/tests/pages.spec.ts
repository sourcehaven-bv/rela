import fs from "node:fs";
import path from "node:path";
import { test as base, expect, SEED } from "./fixtures";
import { KanbanPage, ListPage, PageTabsPage } from "../pages";

/**
 * Pages (TKT-ITQ0HL): several views of one subject as tabs, addressed as
 * /p/<page>/<tab>.
 *
 * The e2e user holds every entity grant but not `work:triage`, so a tab gated
 * on it is left out: "Work" shows two tabs, "Solo" is left with one and hides
 * its tab bar, and "Gated" has none and is left out of the sidebar.
 */
const PAGES_YAML = `
pages:
  work:
    label: "Work"
    tabs:
      - id: board
        label: "Board"
        kanban: feature-board
      - id: table
        label: "Table"
        list: features
      - id: triage
        label: "Triage"
        list: bugs
        permission: work:triage
  solo:
    label: "Solo"
    tabs:
      - id: table
        label: "Bugs"
        list: bugs
      - id: triage
        label: "Triage"
        list: bugs_triage
        permission: work:triage
  gated:
    label: "Gated"
    tabs:
      - id: triage
        label: "Triage"
        list: bugs_triage
        permission: work:triage
`;

const PAGE_NAV = `navigation:
  - page: work
  - page: solo
  - page: gated
`;

const ACL_YAML = `roles:
  editor:
    create: ["*"]
    read: ["*"]
    update: ["*"]
    delete: ["*"]
assignments:
  e2e@example.com: editor
`;

const test = base.extend({
  testProject: async ({ testProject }, use) => {
    const file = path.join(testProject, "data-entry.yaml");
    const yaml = fs.readFileSync(file, "utf8");
    if (!yaml.includes("\nnavigation:\n")) throw new Error("fixture data-entry.yaml has no navigation:");
    fs.writeFileSync(file, yaml.replace("\nnavigation:\n", `\n${PAGE_NAV}`) + PAGES_YAML);
    fs.writeFileSync(path.join(testProject, "acl.yaml"), ACL_YAML);
    await use(testProject);
  },
});

test.describe("Pages", () => {
  test("the sidebar opens a page on its first visible tab", async ({ appPage }) => {
    const pages = new PageTabsPage(appPage);
    await pages.openFromSidebar("Work");

    await expect(appPage).toHaveURL(/\/p\/work\/board$/);
    await pages.expectTitle("Work");
    await pages.expectTabs(["Board", "Table"]);
    await pages.expectActiveTab("Board");
    await pages.expectSidebarActive("Work");
  });

  test("a tab URL survives a reload", async ({ appPage, serverUrl }) => {
    const pages = new PageTabsPage(appPage);
    await appPage.goto(`${serverUrl}/p/work/table`);
    await pages.expectActiveTab("Table");
    await pages.reload();
    await expect(appPage).toHaveURL(/\/p\/work\/table$/);
    await pages.expectActiveTab("Table");
    await pages.expectSidebarActive("Work");
  });

  test("switching tabs changes the URL, drops the query and Back returns", async ({ appPage, serverUrl }) => {
    const pages = new PageTabsPage(appPage);
    await appPage.goto(`${serverUrl}/p/work/table?sort=-title`);
    await pages.expectActiveTab("Table");
    // A real link, so middle-click and "open in new tab" work.
    await expect(pages.tab("Board")).toHaveAttribute("href", /\/p\/work\/board$/);

    await pages.selectTab("Board");
    await expect(appPage).toHaveURL(/\/p\/work\/board$/);
    await pages.expectActiveTab("Board");

    await pages.goBack();
    await expect(appPage).toHaveURL(/\/p\/work\/table\?sort=-title$/);
    await pages.expectActiveTab("Table");
  });

  test("an unknown tab opens the first tab, an unknown page says so", async ({ appPage, serverUrl }) => {
    const pages = new PageTabsPage(appPage);
    await appPage.goto(`${serverUrl}/p/work/nosuch`);
    await expect(appPage).toHaveURL(/\/p\/work\/board$/);

    await appPage.goto(`${serverUrl}/p/nosuch`);
    await pages.expectNotFound("nosuch");
  });

  test("a hidden tab is left out, and so is a page with none", async ({ appPage, serverUrl }) => {
    const pages = new PageTabsPage(appPage);
    await appPage.goto(`${serverUrl}/p/work/triage`);
    await expect(appPage).toHaveURL(/\/p\/work\/board$/);
    await pages.expectTabs(["Board", "Table"]);

    await pages.openFromSidebar("Solo");
    await expect(appPage).toHaveURL(/\/p\/solo\/table$/);
    await pages.expectTitle("Solo");
    await pages.expectNoTabBar();

    await pages.expectSidebarAbsent("Gated");
  });

  test("Back on an entity opened from a tab returns to the tab", async ({ appPage, serverUrl }) => {
    const pages = new PageTabsPage(appPage);
    const list = new ListPage(appPage);
    await appPage.goto(`${serverUrl}/p/work/table`);
    await list.openEntityPageById(SEED.features.authentication);
    await expect(appPage).toHaveURL(/from_page=work(%2F|\/)table/);

    await pages.reload();
    await pages.clickBack();
    await expect(appPage).toHaveURL(/\/p\/work\/table$/);
    await pages.expectActiveTab("Table");
  });

  test("Cancel on a form opened from a board tab returns to the tab", async ({ appPage, serverUrl }) => {
    const pages = new PageTabsPage(appPage);
    const board = new KanbanPage(appPage);
    await appPage.goto(`${serverUrl}/p/work/board`);
    await board.openCardPage("User Authentication");
    await expect(appPage).toHaveURL(/from_page=work(%2F|\/)board/);

    // Opened within the app, Cancel steps back in history to the board.
    const formUrl = appPage.url();
    await pages.cancelForm();
    await expect(appPage).toHaveURL(/\/p\/work\/board(\?|$)/);

    // Opened cold, from a bookmark, it goes to the tab the link names.
    await appPage.goto(formUrl);
    await pages.cancelForm();
    await expect(appPage).toHaveURL(/\/p\/work\/board$/);
  });

  test("the standalone view has no tab bar", async ({ appPage, serverUrl }) => {
    const pages = new PageTabsPage(appPage);
    await appPage.goto(`${serverUrl}/list/features`);
    await pages.expectTitle("Features");
    await pages.expectNoTabBar();
  });
});
