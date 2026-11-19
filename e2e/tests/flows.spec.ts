import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import { test as base } from "./fixtures";
import { FLOW_SEED, FLOW_USERS, writeFlowsProject } from "./flows-project";
import { FlowsPage } from "../pages/flows.page";

// A flow run from the entity page (TKT-ZAXRXND): start it, read why it waits,
// complete the step through the form and see the run finish.

const test = base.extend({
  // eslint-disable-next-line no-empty-pattern
  testProject: async ({}, use) => {
    const dir = fs.mkdtempSync(path.join(fs.realpathSync(os.tmpdir()), "rela-e2e-flows-"));
    try {
      writeFlowsProject(dir);
      await use(dir);
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  },
});

test.describe("flows", () => {
  test.use({ serverEnv: { RELA_DATAENTRY_USER: FLOW_USERS.hr } });

  test("a run started on a person waits for HR, takes the step and finishes", async ({
    appPage,
  }) => {
    const flows = new FlowsPage(appPage);
    const { person, flow, states, role, writes } = FLOW_SEED;

    await flows.openEntity("person", person.id);
    await flows.expectStartOffered(flow.name, flow.label);
    await flows.start(flow.name);

    await flows.expectOneRunIn(states.collect);
    await flows.expectStatus("Waiting for input");
    await flows.expectWaitingFor(role, true);
    await flows.expectCancelOffered();

    await flows.completeStepWithCheckbox(writes);

    await flows.expectStatus("Completed");
    await flows.expectOneRunIn(states.finished);
    await flows.expectNoStepForm();
  });
});

// A person who may start the flow and do its step, but not manage it: a
// customer completing their own signup.
test.describe("flows for someone who only does the step", () => {
  test.use({ serverEnv: { RELA_DATAENTRY_USER: FLOW_USERS.applicant } });

  test("sees a plain step form and none of the flow's workings", async ({ appPage }) => {
    const flows = new FlowsPage(appPage);
    const { person, flow, states, writes, stepFields, guard } = FLOW_SEED;

    await flows.openEntity("person", person.id);
    await flows.expectStartOffered(flow.name, flow.label);
    await flows.start(flow.name);

    await flows.expectOneRunIn(states.collect);
    await flows.expectStartNotOffered(flow.name);
    await flows.expectStepFieldsStacked(stepFields);
    await flows.expectNoManagerDetails(guard);

    await flows.completeStep(writes, stepFields[1], "2026-12-01");

    await flows.expectStatus("Completed");
    await flows.expectOneRunIn(states.finished);
    // The run has ended, so per_subject: one allows another.
    await flows.expectStartOffered(flow.name, flow.label);
  });
});
