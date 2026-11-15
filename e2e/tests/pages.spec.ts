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
  workshop:
    label: "Workshop"
    mode: steps
    tabs:
      - id: features
        label: "Features"
        list: features
        prompt: "Which features are in scope?"
      - id: board
        label: "Board"
        kanban: feature-board
  review:
    label: "Review"
    mode: steps
    tabs:
      - id: each
        label: "Each feature"
        list: features
        walk: true
        done_when: "entity.status == 'approved'"
      - id: bugs
        label: "Bugs"
        list: bugs
        done_when: "entity.status ~= 'done'"
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
  - page: workshop
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

  test("a steps page walks its steps, and presents without the sidebar", async ({ appPage, serverUrl }) => {
    const pages = new PageTabsPage(appPage);
    await appPage.goto(`${serverUrl}/p/workshop/features`);
    await pages.expectNoTabBar();
    await pages.expectSteps([/1\s*Features/, /2\s*Board/]);
    await pages.expectCurrentStep("Features");
    await pages.expectPrompt("Which features are in scope?");
    await expect(pages.stepButton("Back")).toBeDisabled();

    await pages.clickStepButton("Next");
    await expect(appPage).toHaveURL(/\/p\/workshop\/board$/);
    await pages.expectCurrentStep("Board");
    await expect(pages.stepButton("Next")).toBeDisabled();
    await pages.goBack();
    await pages.expectCurrentStep("Features");

    await pages.clickStepButton("Present");
    await expect(appPage).toHaveURL(/\/p\/workshop\/features\?present=1$/);
    await pages.expectSidebarShown(false);
    await pages.pressKey("PageDown");
    await expect(appPage).toHaveURL(/\/p\/workshop\/board\?present=1$/);
    await pages.expectCurrentStep("Board");

    await pages.clickStepButton("Exit");
    await expect(appPage).toHaveURL(/\/p\/workshop\/board$/);
    await pages.expectSidebarShown(true);
  });

  // TKT-M07C97Y and TKT-MWZF0UO. The features list has no sort of its own,
  // so its rows come in id order: FEAT-001 to FEAT-004.
  test("a walking step shows one row at a time, and a step shows its done check", async ({
    appPage,
    serverUrl,
  }) => {
    const pages = new PageTabsPage(appPage);
    await appPage.goto(`${serverUrl}/p/review/each`);
    await expect(appPage).toHaveURL(/\/p\/review\/each\?item=FEAT-001$/);
    await pages.expectWalkPosition("1 of 4");
    await expect(appPage.getByText("User Authentication", { exact: true })).toBeVisible();
    await expect(pages.walkLink("Previous row")).toHaveCount(0);

    await pages.walkLink("Next row").click();
    await expect(appPage).toHaveURL(/\?item=FEAT-002$/);
    await pages.expectWalkPosition("2 of 4");
    await expect(appPage.getByText("Dashboard Analytics", { exact: true })).toBeVisible();

    // Every bug is open, but only one feature is approved.
    await pages.expectStepDone("Bugs", true);
    await pages.expectStepDone("Each feature", false);
    await expect(pages.stepButton("Next")).toBeEnabled();

    // Presenting, PageDown walks the rows, then the steps.
    await appPage.goto(`${serverUrl}/p/review/each?present=1&item=FEAT-003`);
    await pages.expectWalkPosition("3 of 4");
    await pages.pressKey("PageDown");
    await expect(appPage).toHaveURL(/\?present=1&item=FEAT-004$/);
    await pages.expectWalkPosition("4 of 4");
    await pages.pressKey("PageDown");
    await expect(appPage).toHaveURL(/\/p\/review\/bugs\?present=1$/);
    await pages.expectCurrentStep("Bugs");
  });

  test("a walking step says when its row is not in the list", async ({ appPage, serverUrl }) => {
    const pages = new PageTabsPage(appPage);
    await appPage.goto(`${serverUrl}/p/review/each?item=BUG-001`);
    await expect(appPage.getByText("Not in this list")).toBeVisible();
    await appPage.getByRole("link", { name: "Go to the first row" }).click();
    await expect(appPage).toHaveURL(/\?item=FEAT-001$/);
    await pages.expectWalkPosition("1 of 4");
  });
});
