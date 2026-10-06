import fs from "node:fs";
import path from "node:path";
import { test as base, expect, SEED } from "./fixtures";
import { SpacesPage } from "../pages";

/**
 * The space's Create menu on an entity page (BUG-PFLS22): a new item is linked
 * to the page's entity, as New in a tab links it.
 *
 * The feature page offers two relations for a task: `implements` on the Tasks
 * tab and `contains` on the Timeline tab. The open tab's relation wins. The
 * Blockers tab has neither, so the page has two candidates and no link is made.
 */
const CONTAINS_YAML = `relations:
  contains:
    from: [feature]
    to: [task]
    inverse: contained_in
`;

const SPACE_YAML = `spaces:
  - id: work
    label: "Work"
    home:
      list: features
    create: [task]
    navigation:
      - label: "Features"
        list: features

gantts:
  roadmap:
    title: "Roadmap"
    hierarchy: [contains]
    sources:
      feature: { label: title }
      task: { label: title }

pages:
  feature:
    entity_type: feature
    label: "Feature"
    tabs:
      - { id: tasks, label: "Tasks", list: tasks, scope: { relation: implements, direction: incoming } }
      - { id: timeline, label: "Timeline", gantt: roadmap, scope: root }
      - { id: blockers, label: "Blockers", list: bugs, scope: { relation: blocks, direction: outgoing } }
`;

const test = base.extend({
  testProject: async ({ testProject }, use) => {
    const schema = path.join(testProject, "schema.yaml");
    const meta = fs.readFileSync(schema, "utf8");
    if (!meta.includes("\nrelations:\n")) throw new Error("fixture schema.yaml has no relations:");
    fs.writeFileSync(schema, meta.replace("\nrelations:\n", `\n${CONTAINS_YAML}`));

    const file = path.join(testProject, "data-entry.yaml");
    const yaml = fs.readFileSync(file, "utf8");
    const at = yaml.indexOf("\nnavigation:\n");
    if (at < 0) throw new Error("fixture data-entry.yaml has no navigation:");
    fs.writeFileSync(file, yaml.slice(0, at + 1) + SPACE_YAML);
    await use(testProject);
  },
});

const FEATURE = SEED.features.authentication;

test.describe("Create menu on an entity page", () => {
  test("a list tab links the new task over the tab's relation", async ({ appPage, serverUrl, api }) => {
    const spaces = new SpacesPage(appPage);
    await appPage.goto(`${serverUrl}/s/work/p/feature/${FEATURE}/tasks`);
    const id = await spaces.createFromMenu("Task", "tasks", "Created on the tasks tab");

    expect((await api.listRelations("tasks", id, "implements")).map((r) => r.id)).toEqual([FEATURE]);
    expect((await api.listRelations("features", FEATURE, "contains")).map((r) => r.id)).not.toContain(id);
  });

  test("a timeline tab links the new task over the hierarchy relation", async ({ appPage, serverUrl, api }) => {
    const spaces = new SpacesPage(appPage);
    await appPage.goto(`${serverUrl}/s/work/p/feature/${FEATURE}/timeline`);
    const id = await spaces.createFromMenu("Task", "tasks", "Created on the timeline tab");

    expect((await api.listRelations("features", FEATURE, "contains")).map((r) => r.id)).toContain(id);
    expect(await api.listRelations("tasks", id, "implements")).toEqual([]);
  });

  test("two candidate relations on the page make no link", async ({ appPage, serverUrl, api }) => {
    const spaces = new SpacesPage(appPage);
    await appPage.goto(`${serverUrl}/s/work/p/feature/${FEATURE}/blockers`);
    const id = await spaces.createFromMenu("Task", "tasks", "Created on the blockers tab");

    expect(await api.listRelations("tasks", id, "implements")).toEqual([]);
    expect((await api.listRelations("features", FEATURE, "contains")).map((r) => r.id)).not.toContain(id);
  });
});
