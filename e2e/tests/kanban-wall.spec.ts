import fs from "node:fs";
import path from "node:path";
import { test as base, expect, SEED, STATUS } from "./fixtures";
import { KanbanWallPage } from "../pages";

/**
 * Walls of cards (TKT-0VBQW0H). A feature's tasks are notes on its tabs. On
 * the Notes tab they are one grid, reordered by a drop on a card's edge. On
 * the Wall tab, dropping one on a loose note makes a group, named after that
 * note and offered by the feature; dropping a third on a grouped note adds it
 * to that group, and a group moves by its handle. Groups, their order and the
 * card order are stored, so they survive a reload.
 */
const CLUSTER_YAML = `entities:
  cluster:
    label: Cluster
    id_type: sequential
    id_prefix: CL
    properties:
      title:
        type: string
        required: true
`;

const RELATIONS_YAML = `relations:
  contains:
    from: [feature]
    to: [task]
    orderable: outgoing
  in_group:
    from: [task]
    to: [cluster]
    max_outgoing: 1
  offers_group:
    from: [feature]
    to: [cluster]
    orderable: outgoing
`;

const KANBAN_YAML = `kanbans:
  task-wall:
    entity_type: task
    title: "Wall"
    layout: wall
    columns_from:
      relation: in_group
      offered_by: offers_group
    card:
      title: title
      style: note
  task-notes:
    entity_type: task
    title: "Notes"
    layout: wall
    card:
      title: title
      style: note
`;

const PAGES_YAML = `pages:
  feature:
    entity_type: feature
    label: "Feature"
    tabs:
      - { id: wall, label: "Wall", kanban: task-wall, scope: { relation: contains, direction: outgoing } }
      - { id: notes, label: "Notes", kanban: task-notes, scope: { relation: contains, direction: outgoing } }
`;

function splice(yaml: string, key: string, block: string): string {
  if (!yaml.includes(`\n${key}:\n`)) throw new Error(`fixture YAML has no ${key}:`);
  return yaml.replace(`\n${key}:\n`, `\n${block}`);
}

const test = base.extend({
  testProject: async ({ testProject }, use) => {
    const schema = path.join(testProject, "schema.yaml");
    let meta = fs.readFileSync(schema, "utf8");
    meta = splice(meta, "entities", CLUSTER_YAML);
    meta = splice(meta, "relations", RELATIONS_YAML);
    fs.writeFileSync(schema, meta);

    const file = path.join(testProject, "data-entry.yaml");
    let yaml = fs.readFileSync(file, "utf8");
    yaml = splice(yaml, "kanbans", KANBAN_YAML);
    yaml = splice(yaml, "navigation", `${PAGES_YAML}\nnavigation:\n`);
    fs.writeFileSync(file, yaml);
    await use(testProject);
  },
});

const FEATURE = SEED.features.authentication;
const TITLES = ["Alpha note", "Beta note", "Gamma note"];

test.describe("Kanban wall", () => {
  test.beforeEach(async ({ api }) => {
    for (const title of TITLES) {
      const created = await api.createEntity("tasks", { properties: { title, status: STATUS.feature.draft } });
      await api.createRelation("features", FEATURE, "contains", created.id);
    }
  });

  test("dropping a note on another groups them", async ({ appPage, serverUrl }) => {
    const wall = new KanbanWallPage(appPage);
    await appPage.goto(`${serverUrl}/p/feature/${FEATURE}/wall`);
    await expect.poll(() => wall.pileTitles()).toEqual(TITLES);

    await wall.dropOnto("Alpha note", "Beta note");
    await expect.poll(() => wall.groups()).toEqual([{ name: "Beta note", cards: ["Alpha note", "Beta note"] }]);
    await expect.poll(() => wall.pileTitles()).toEqual(["Gamma note"]);

    await wall.dropOnto("Gamma note", "Alpha note");
    await expect.poll(() => wall.groups()).toEqual([
      { name: "Beta note", cards: TITLES },
    ]);

    await appPage.reload();
    await expect.poll(() => wall.groups()).toEqual([{ name: "Beta note", cards: TITLES }]);
    await expect.poll(() => wall.pileTitles()).toEqual([]);
  });

  test("a plain wall shows every card in one grid and reorders them", async ({ appPage, serverUrl }) => {
    const wall = new KanbanWallPage(appPage);
    await appPage.goto(`${serverUrl}/p/feature/${FEATURE}/notes`);
    await expect.poll(() => wall.wallTitles()).toEqual(TITLES);

    await wall.dropAfter("Alpha note", "Gamma note");
    await expect.poll(() => wall.wallTitles()).toEqual(["Beta note", "Gamma note", "Alpha note"]);
    await appPage.reload();
    await expect.poll(() => wall.wallTitles()).toEqual(["Beta note", "Gamma note", "Alpha note"]);
  });

  test("a group moves by its handle", async ({ api, appPage, serverUrl }) => {
    const extra = await api.createEntity("tasks", { properties: { title: "Delta note", status: STATUS.feature.draft } });
    await api.createRelation("features", FEATURE, "contains", extra.id);
    const wall = new KanbanWallPage(appPage);
    await appPage.goto(`${serverUrl}/p/feature/${FEATURE}/wall`);
    await expect.poll(() => wall.pileTitles()).toEqual([...TITLES, "Delta note"]);
    await wall.dropOnto("Alpha note", "Beta note");
    await expect.poll(() => wall.pileTitles()).toEqual(["Gamma note", "Delta note"]);
    await wall.dropOnto("Delta note", "Gamma note");
    await expect.poll(async () => (await wall.groups()).map((g) => g.name)).toEqual(["Beta note", "Gamma note"]);

    await wall.moveGroupBefore("Gamma note", "Beta note");
    await expect.poll(async () => (await wall.groups()).map((g) => g.name)).toEqual(["Gamma note", "Beta note"]);
    await appPage.reload();
    await expect.poll(async () => (await wall.groups()).map((g) => g.name)).toEqual(["Gamma note", "Beta note"]);
  });
});
