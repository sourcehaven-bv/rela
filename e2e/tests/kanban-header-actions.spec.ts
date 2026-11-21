import fs from "node:fs";
import path from "node:path";
import { test as base } from "./fixtures";
import { KanbanPage } from "../pages";

/**
 * A board whose columns each offer Add does not repeat it as a header New
 * button. The plain-property board keeps New (see kanban.spec.ts).
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
  in_group:
    from: [task]
    to: [cluster]
    max_outgoing: 1
`;

const KANBAN_YAML = `kanbans:
  task-groups:
    entity_type: task
    title: "Task groups"
    create_form: task
    columns_from:
      relation: in_group
    card:
      title: title
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
    fs.writeFileSync(file, splice(fs.readFileSync(file, "utf8"), "kanbans", KANBAN_YAML));
    await use(testProject);
  },
});

test.describe("Kanban header actions", () => {
  test("a board with per-column Add has no header New button", async ({ appPage, api }) => {
    await api.createEntity("clusters", { properties: { title: "Group A" } });
    const kanban = new KanbanPage(appPage);

    await kanban.navigateToKanban("task-groups");

    await kanban.expectColumnAddWithoutHeaderNew();
  });
});
