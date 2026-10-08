import fs from "node:fs";
import path from "node:path";
import { test as base, expect, SEED, STATUS } from "./fixtures";
import { ListPage, RelationOrderPage } from "../pages";

/**
 * Setting the order of an orderable relation by hand (TKT-RCRUWZ): a
 * feature's tasks, on a list tab, a board tab and a table section of the
 * feature's own page. Each surface shows the tasks in the feature's order,
 * and a move there survives a reload, because it was stored on the edge.
 */
const CONTAINS_YAML = `relations:
  contains:
    from: [feature]
    to: [task]
    inverse: contained_in
    orderable: outgoing
`;

const PAGES_YAML = `pages:
  feature:
    entity_type: feature
    label: "Feature"
    tabs:
      - { id: tasks, label: "Tasks", list: tasks, scope: { relation: contains, direction: outgoing } }
      - { id: board, label: "Board", kanban: task-board, scope: { relation: contains, direction: outgoing } }
`;

const KANBAN_YAML = `kanbans:
  task-board:
    entity_type: task
    title: "Task Board"
    column_property: status
    columns:
      - { value: draft, label: Draft }
      - { value: done, label: Done }
    card:
      title: title
`;

const VIEW_YAML = `views:
  feature:
    title: "Feature"
    entry:
      type: feature
    traverse:
      - from: entry
        follow: contains
        collect_as: parts
    sections:
      - heading: "Parts"
        source: parts
        display: table
        columns:
          - property: title
`;

function splice(yaml: string, key: string, block: string): string {
  if (!yaml.includes(`\n${key}:\n`)) throw new Error(`fixture YAML has no ${key}:`);
  return yaml.replace(`\n${key}:\n`, `\n${block}`);
}

const test = base.extend({
  testProject: async ({ testProject }, use) => {
    const schema = path.join(testProject, "schema.yaml");
    fs.writeFileSync(schema, splice(fs.readFileSync(schema, "utf8"), "relations", CONTAINS_YAML));

    const file = path.join(testProject, "data-entry.yaml");
    let yaml = fs.readFileSync(file, "utf8");
    yaml = splice(yaml, "kanbans", KANBAN_YAML);
    yaml = splice(yaml, "views", VIEW_YAML);
    yaml = splice(yaml, "navigation", `${PAGES_YAML}\nnavigation:\n`);
    fs.writeFileSync(file, yaml);
    await use(testProject);
  },
});

const FEATURE = SEED.features.authentication;
const TITLES = ["Alpha step", "Beta step", "Gamma step"];

test.describe("Relation order", () => {
  test.beforeEach(async ({ api }) => {
    for (const title of TITLES) {
      const created = await api.createEntity("tasks", { properties: { title, status: STATUS.feature.draft } });
      await api.createRelation("features", FEATURE, "contains", created.id);
    }
  });

  test("a list tab moves a row by keyboard and by drag", async ({ appPage, serverUrl }) => {
    const order = new RelationOrderPage(appPage);
    await appPage.goto(`${serverUrl}/p/feature/${FEATURE}/tasks`);
    await expect.poll(() => order.listTitles()).toEqual(TITLES);

    await order.stepListRow("Alpha step", "ArrowDown");
    await expect.poll(() => order.listTitles()).toEqual(["Beta step", "Alpha step", "Gamma step"]);

    await order.dragListRow("Gamma step", "Beta step", "before");
    await expect.poll(() => order.listTitles()).toEqual(["Gamma step", "Beta step", "Alpha step"]);

    await order.dragListRow("Gamma step", "Alpha step", "after");
    await expect.poll(() => order.listTitles()).toEqual(["Beta step", "Alpha step", "Gamma step"]);

    await appPage.reload();
    await expect.poll(() => order.listTitles()).toEqual(["Beta step", "Alpha step", "Gamma step"]);
  });

  test("a board tab moves a card within its column", async ({ appPage, serverUrl }) => {
    const order = new RelationOrderPage(appPage);
    await appPage.goto(`${serverUrl}/p/feature/${FEATURE}/board`);
    await expect.poll(() => order.columnTitles("Draft")).toEqual(TITLES);

    await order.dragCard("Gamma step", "Alpha step", "before");
    await expect.poll(() => order.columnTitles("Draft")).toEqual(["Gamma step", "Alpha step", "Beta step"]);

    await order.dragCard("Gamma step", "Beta step", "after");
    await expect.poll(() => order.columnTitles("Draft")).toEqual(["Alpha step", "Beta step", "Gamma step"]);

    await order.dragCard("Alpha step", "Beta step", "after");
    await expect.poll(() => order.columnTitles("Draft")).toEqual(["Beta step", "Alpha step", "Gamma step"]);

    await appPage.reload();
    await expect.poll(() => order.columnTitles("Draft")).toEqual(["Beta step", "Alpha step", "Gamma step"]);
  });

  test("a table section on the entity page moves a row", async ({ appPage, serverUrl }) => {
    const order = new RelationOrderPage(appPage);
    await appPage.goto(`${serverUrl}/entity/feature/${FEATURE}`);
    await expect.poll(() => order.sectionTitles("Parts")).toEqual(["Alpha step", "Beta step", "Gamma step"]);

    await order.stepSectionRow("Parts", "Gamma step", "ArrowUp");
    await expect.poll(() => order.sectionTitles("Parts")).toEqual(["Alpha step", "Gamma step", "Beta step"]);

    await appPage.reload();
    await expect.poll(() => order.sectionTitles("Parts")).toEqual(["Alpha step", "Gamma step", "Beta step"]);
  });

  test("a list sorted by the reader offers no handles", async ({ appPage, serverUrl }) => {
    const order = new RelationOrderPage(appPage);
    await appPage.goto(`${serverUrl}/p/feature/${FEATURE}/tasks`);
    await order.expectHandles(TITLES.length);

    // A sort of the reader's own shows another order, so a move would land
    // somewhere the reader cannot see.
    await new ListPage(appPage).sortByColumn("Status");
    await order.expectHandles(0);
  });
});
