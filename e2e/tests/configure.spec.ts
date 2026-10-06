import fs from "node:fs";
import path from "node:path";
import { test as base, expect } from "./fixtures";
import { ConfigurePage } from "../pages";

/**
 * The Configure space (TKT-F5NGMG). The server runs with config editing on,
 * and the e2e user holds config:edit, so the space switcher offers
 * Configure and every screen may change the setup.
 */
const ACL_YAML = `roles:
  admin:
    create: ["*"]
    read: ["*"]
    update: ["*"]
    delete: ["*"]
    permissions: [config:edit]
assignments:
  e2e@example.com: admin
`;

const test = base.extend({
  testProject: async ({ testProject }, use) => {
    fs.writeFileSync(path.join(testProject, "acl.yaml"), ACL_YAML);
    await use(testProject);
  },
});

test.use({ serverEnv: { RELA_CONFIG_EDITING: "1" } });

test.describe("Configure", () => {
  test("the space switcher opens Configure", async ({ appPage }) => {
    const configure = new ConfigurePage(appPage);
    await configure.openSwitcher();
    await configure.configureMenuItem.click();
    await expect(appPage).toHaveURL(/\/configure\/entity-types$/);
    await expect(configure.entityTypes).toContainText("Feature");
    await expect(configure.entityTypes).toContainText("Bug");
  });

  test("a renamed entity type is saved to the data model", async ({
    appPage,
    serverUrl,
    testProject,
  }) => {
    const configure = new ConfigurePage(appPage);
    await configure.goto(serverUrl, "/entity-types/feature");
    await configure.entitySetting("Name").fill("Epic");
    await expect(configure.draftCount).toHaveText("1 unsaved change");

    await configure.openReview();
    await expect(configure.reviewDrawer).toContainText("Name");
    await expect(configure.reviewDrawer).toContainText("Epic");
    await configure.save();

    await expect(configure.toastContainer).toContainText(/saved/i);
    await expect(configure.draftCount).toHaveCount(0);
    const schema = fs.readFileSync(
      path.join(testProject, "schema.yaml"),
      "utf8",
    );
    expect(schema).toMatch(/label: Epic/);
  });

  test("a board column header is saved to the screens", async ({
    appPage,
    serverUrl,
    testProject,
  }) => {
    const configure = new ConfigurePage(appPage);
    await configure.goto(serverUrl, "/boards/feature-board");
    await configure.boardColumnHeader("approved").fill("Signed off");
    await expect(configure.draftCount).toHaveText("1 unsaved change");

    await configure.openReview();
    await configure.save();

    await expect(configure.toastContainer).toContainText(/saved/i);
    const screens = fs.readFileSync(
      path.join(testProject, "data-entry.yaml"),
      "utf8",
    );
    expect(screens).toMatch(/label: Signed off/);
    expect(screens).not.toMatch(/label: Approved/);
  });

  test("a renamed property moves the values records hold", async ({
    appPage,
    serverUrl,
    api,
  }) => {
    const created = await api.createEntity("bugs", {
      properties: { title: "Slow page", priority: "high" },
    });
    const configure = new ConfigurePage(appPage);
    await configure.renameProperty(serverUrl, "bug", "priority", "urgency");
    // The rename, plus the screens that show the property following it.
    await expect(configure.draftCount).toHaveText(/\d+ unsaved changes?/);

    await configure.openReview();
    await expect(configure.reviewDrawer).toContainText("priority");
    await expect(configure.saveButton).toHaveText(
      /Save and migrate \d+ records?/,
    );
    await configure.save();
    await expect(configure.toastContainer).toContainText(/saved/i);

    const bug = await api.getEntity("bugs", created.id);
    expect(bug.properties).toMatchObject({ urgency: "high" });
    expect(bug.properties).not.toHaveProperty("priority");
  });

  test("a list keeps several sort keys and its filters", async ({
    appPage,
    serverUrl,
    testProject,
  }) => {
    const configure = new ConfigurePage(appPage);
    await configure.goto(serverUrl, "/lists/features");
    await configure.addSortKey("priority");
    await configure.addSortKey("status");
    await configure.setSortDirection("status", "desc");
    await configure.chooseQueryScope("in_flight");
    await configure.addFixedFilter("priority", "high");

    await configure.openReview();
    await configure.save();
    await expect(configure.toastContainer).toContainText(/saved/i);

    const screens = fs.readFileSync(
      path.join(testProject, "data-entry.yaml"),
      "utf8",
    );
    const features = screens.slice(
      screens.indexOf("  features:"),
      screens.indexOf("  bugs:"),
    );
    expect(features).toMatch(
      /sort:\n\s+- property: priority\n\s+- property: status\n\s+direction: desc/,
    );
    expect(features).toMatch(/query_scope: in_flight/);
    expect(features).toMatch(
      /filters:\n\s+- property: priority\n\s+operator: "?="?\n\s+value: high/,
    );
    // The filter controls the screen did not touch are kept as they were.
    expect(features).toMatch(
      /filter_controls:\n\s+- property: status\n\s+- property: priority/,
    );
  });

  test("a sidebar group lists the records of a query scope", async ({
    appPage,
    serverUrl,
    testProject,
  }) => {
    const configure = new ConfigurePage(appPage);
    await configure.goto(serverUrl, "/navigation");
    await configure.addNavGroup("In flight");
    await configure.addNavEntry("entities:feature", "In flight");
    await configure.openNavEntrySettings("feature");
    await configure.chooseNavRecords("in_flight");

    await configure.openReview();
    await configure.save();
    await expect(configure.toastContainer).toContainText(/saved/i);

    const screens = fs.readFileSync(
      path.join(testProject, "data-entry.yaml"),
      "utf8",
    );
    expect(screens).toMatch(
      /- group: In flight\n\s+items:\n\s+- entities: feature\n\s+query_scope: in_flight/,
    );
  });

  test("the draft survives a reload and can be discarded", async ({
    appPage,
    serverUrl,
  }) => {
    const configure = new ConfigurePage(appPage);
    await configure.goto(serverUrl, "/choice-lists/priority");
    await configure.addOption("urgent");
    await expect(configure.options).toContainText("urgent");
    await expect(configure.preview).toContainText("Preview");
    await expect(configure.draftCount).toHaveText("1 unsaved change");

    await configure.reload();
    await expect(configure.options).toContainText("urgent");
    await expect(configure.draftCount).toHaveText("1 unsaved change");

    await configure.openReview();
    await configure.discardAll();
    await expect(configure.options).not.toContainText("urgent");
    await expect(configure.draftCount).toHaveCount(0);
  });
});
