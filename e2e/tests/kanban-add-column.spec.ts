import fs from "node:fs";
import path from "node:path";
import { test as base, expect } from "./fixtures";
import { KanbanPage } from "../pages";

/**
 * "Add column" on a kanban board (TKT-1X5G3BP). A new column is a new value
 * of the board's inline enum, saved through the Configure API. The server
 * runs with config editing on, and the e2e user holds config:edit.
 *
 * The inline-enum type and its board are added here, not in fixtures.ts, so
 * no other spec sees an extra type.
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

const CARD_TYPE = `entities:
  card:
    label: Card
    id_type: sequential
    id_prefix: CRD
    properties:
      title:
        type: string
      stage:
        type: enum
        values: [open, decided]
        labels:
          open: Open
          decided: Decided
`;

const BOARD = `kanbans:
  card-board:
    entity_type: card
    title: "Card Board"
    column_property: stage
    columns:
      - value: open
        label: Open
      - value: decided
        label: Decided
    card:
      title: title
    add_column: true
`;

function insertAfterKey(file: string, key: string, block: string) {
  const text = fs.readFileSync(file, "utf8");
  const marker = `\n${key}:\n`;
  expect(text.includes(marker), `${key}: not found in ${file}`).toBe(true);
  fs.writeFileSync(file, text.replace(marker, `\n${block}`));
}

const test = base.extend({
  testProject: async ({ testProject }, use) => {
    fs.writeFileSync(path.join(testProject, "acl.yaml"), ACL_YAML);
    insertAfterKey(path.join(testProject, "schema.yaml"), "entities", CARD_TYPE);
    insertAfterKey(path.join(testProject, "data-entry.yaml"), "kanbans", BOARD);
    await use(testProject);
  },
});

test.use({ serverEnv: { RELA_CONFIG_EDITING: "1" } });

test.describe("Kanban add column", () => {
  test("adds a column and keeps it in the schema and the board", async ({ appPage, testProject }) => {
    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban("card-board");
    await kanban.expectColumnCount(2);

    await kanban.addColumn("In review");

    await kanban.expectColumnLabel("In review");
    await kanban.expectColumnCount(3);
    const schema = fs.readFileSync(path.join(testProject, "schema.yaml"), "utf8");
    expect(schema).toMatch(/in_review: In review/);
    const dataEntry = fs.readFileSync(path.join(testProject, "data-entry.yaml"), "utf8");
    expect(dataEntry).toMatch(/value: in_review\s+label: In review/);
  });

  test("refuses a name the board already has", async ({ appPage }) => {
    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban("card-board");

    await kanban.addColumn("Decided");

    await kanban.expectAddColumnError("The board already has a column named Decided");
    await kanban.expectColumnCount(2);
  });

  test("is absent on a board without add_column", async ({ appPage }) => {
    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban("bug-board");
    await kanban.expectColumnCount(3);
    await expect(kanban.addColumnButton).toHaveCount(0);
  });
});

// Without config:edit the board offers no "Add column". The server refuses
// the save itself as well (TestConfigure_Gate in cmd/rela-server).
const NO_CONFIG_ACL = ACL_YAML.replace("    permissions: [config:edit]\n", "");

const testWithoutRight = base.extend({
  testProject: async ({ testProject }, use) => {
    fs.writeFileSync(path.join(testProject, "acl.yaml"), NO_CONFIG_ACL);
    insertAfterKey(path.join(testProject, "schema.yaml"), "entities", CARD_TYPE);
    insertAfterKey(path.join(testProject, "data-entry.yaml"), "kanbans", BOARD);
    await use(testProject);
  },
});

testWithoutRight.describe("Kanban add column without config:edit", () => {
  testWithoutRight.use({ serverEnv: { RELA_CONFIG_EDITING: "1" } });

  testWithoutRight("hides the control", async ({ appPage }) => {
    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban("card-board");
    await kanban.expectColumnCount(2);
    await expect(kanban.addColumnButton).toHaveCount(0);
  });
});
