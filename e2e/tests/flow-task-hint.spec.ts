import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import { test as base, expect } from "./fixtures";
import { FLOW_SEED, FLOW_USERS, writeFlowsProject } from "./flows-project";
import { FlowsPage } from "../pages/flows.page";
import { NextActionPage } from "../pages/next-action.page";

// A waiting flow task feeds the next-action banner (`source: flow_tasks`).
// The banner names the step and the subject, and its button opens the
// subject page with the run's step in view. Snoozing hides only the hint;
// completing the step removes it.

const NEXT_ACTIONS_YAML = `next_action_bands:
  - id: todo
    label: To do
    prominence: banner
next_actions:
  flow-steps:
    band: todo
    source: flow_tasks
    flows: [${FLOW_SEED.flow.name}]
    suggest: "Complete {step} for {subject}"
    actions:
      - snooze: ["1d"]
`;

const test = base.extend({
  // eslint-disable-next-line no-empty-pattern
  testProject: async ({}, use) => {
    const dir = fs.mkdtempSync(
      path.join(fs.realpathSync(os.tmpdir()), "rela-e2e-flowhint-"),
    );
    try {
      writeFlowsProject(dir);
      // Name persons by their name, so the hint can say whose step it is.
      const schema = path.join(dir, "schema.yaml");
      const types = fs.readFileSync(schema, "utf8");
      if (!types.includes("    id_type: sequential\n"))
        throw new Error("fixture schema changed");
      fs.writeFileSync(
        schema,
        types.replace(
          "    id_type: sequential\n",
          "    id_type: sequential\n    display_property: name\n",
        ),
      );
      const file = path.join(dir, "data-entry.yaml");
      const yaml = fs.readFileSync(file, "utf8");
      if (!yaml.includes("\nnavigation:\n"))
        throw new Error("fixture YAML has no navigation:");
      fs.writeFileSync(
        file,
        yaml.replace(
          "\nnavigation:\n",
          `\n${NEXT_ACTIONS_YAML}\nnavigation:\n`,
        ),
      );
      await use(dir);
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  },
});

test.use({ serverEnv: { RELA_DATAENTRY_USER: FLOW_USERS.hr } });

const MESSAGE = `Complete ${FLOW_SEED.states.collect} for ${FLOW_SEED.person.name}`;

test.describe("flow task hint", () => {
  test("the banner opens the step, and completing it clears the banner", async ({
    appPage,
    serverUrl,
  }) => {
    const flows = new FlowsPage(appPage);
    const na = new NextActionPage(appPage);
    const { person, flow, states, writes } = FLOW_SEED;

    await flows.openEntity("person", person.id);
    await flows.start(flow.name);
    await flows.expectOneRunIn(states.collect);

    await appPage.goto(`${serverUrl}/list/persons`);
    await na.expectShowing(MESSAGE);
    await expect(na.card).toHaveAttribute("data-entity-id", person.id);

    await na.openOffer("Open");
    await expect(appPage).toHaveURL(
      new RegExp(`/entity/person/${person.id}\\?run=`),
    );
    const card = flows.runs().first();
    await expect(card).toBeFocused({ timeout: 15_000 });
    await expect(card).toBeInViewport();
    // The step form itself is in view, not left at the bottom edge while
    // the page renders above it (BUG-UZGMHIK).
    await flows.expectStepFormTopInView();

    await flows.completeStepWithCheckbox(writes);
    await flows.expectOneRunIn(states.finished);
    await expect(na.card).toHaveCount(0, { timeout: 15_000 });

    const asked = na.waitForRequest("list");
    await appPage.goto(`${serverUrl}/list/persons`);
    await na.expectNoneAfter(asked);
  });

  test("snoozing hides the hint but not the task", async ({
    appPage,
    serverUrl,
  }) => {
    const flows = new FlowsPage(appPage);
    const na = new NextActionPage(appPage);
    const { person, flow, states } = FLOW_SEED;

    await flows.openEntity("person", person.id);
    await flows.start(flow.name);
    await flows.expectOneRunIn(states.collect);

    await appPage.goto(`${serverUrl}/list/persons`);
    await na.expectShowing(MESSAGE);
    await na.snoozeFor("1d");
    await expect(na.card).toHaveCount(0);

    await flows.openEntity("person", person.id);
    await flows.expectOneRunIn(states.collect);
    await expect(flows.stepForm()).toBeVisible();
    await expect(na.card).toHaveCount(0);
  });
});
