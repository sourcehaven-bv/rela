import { test, expect } from "./fixtures";
import { FormPage } from "../pages";

test.describe("Wizard (multi-step) forms", () => {
  const createdTasks: string[] = [];

  test.afterEach(async ({ api }) => {
    while (createdTasks.length) {
      const id = createdTasks.pop()!;
      await api.deleteEntity("tasks", id).catch(() => {});
    }
  });

  test("renders steps and starts on the first step", async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_wizard");

    // The conditional "Assignment" step is hidden until `done` is checked.
    expect(await formPage.wizardStepTitles()).toEqual(["Basics", "Status"]);
    expect(await formPage.activeStepIndex()).toBe(0);
    // Only step 1's fields are shown.
    expect(await formPage.isFieldVisible("title")).toBeTruthy();
    expect(await formPage.isFieldVisible("status")).toBeFalsy();
  });

  test("visible_when reveals a conditional step", async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_wizard");

    await formPage.setCheckbox("done", true);
    expect(await formPage.wizardStepTitles()).toEqual([
      "Basics",
      "Assignment",
      "Status",
    ]);

    await formPage.setCheckbox("done", false);
    expect(await formPage.wizardStepTitles()).toEqual(["Basics", "Status"]);
  });

  test("next/back navigation moves between steps and syncs ?step=", async ({
    appPage,
  }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_wizard");

    await formPage.fillField("title", "Wizard task");
    await formPage.clickNext();
    expect(await formPage.activeStepIndex()).toBe(1);
    expect(formPage.readStepParam()).toBe("1");
    // Step 2 (Status, since done is unchecked) is shown.
    expect(await formPage.isFieldVisible("status")).toBeTruthy();

    await formPage.clickBack();
    expect(await formPage.activeStepIndex()).toBe(0);
    expect(formPage.readStepParam()).toBe("0");
    // Value entered on step 1 is preserved across back/next.
    expect(await formPage.getFieldValue("title")).toBe("Wizard task");
  });

  test("clicking a step pill jumps to it (forward and back)", async ({
    appPage,
  }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_wizard");

    // Jump forward to the last step (Status, index 1 while Assignment hidden)
    // without going through Next — a deliberate pill click is not gated.
    await formPage.clickStep(1);
    expect(await formPage.activeStepIndex()).toBe(1);
    expect(formPage.readStepParam()).toBe("1");
    expect(await formPage.isFieldVisible("status")).toBeTruthy();

    // Jump back to the first step by clicking its pill.
    await formPage.clickStep(0);
    expect(await formPage.activeStepIndex()).toBe(0);
    expect(await formPage.isFieldVisible("title")).toBeTruthy();
  });

  test("refresh returns to the encoded step", async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_wizard");
    await formPage.fillField("title", "Deep link task");
    await formPage.clickNext();
    expect(formPage.readStepParam()).toBe("1");

    await appPage.reload();
    await formPage.waitForSpinnerToDisappear();
    expect(await formPage.activeStepIndex()).toBe(1);
  });

  test("required_when blocks Next until the conditional field is filled", async ({
    appPage,
  }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_wizard");

    await formPage.fillField("title", "Needs assignee");
    await formPage.setCheckbox("done", true); // reveals Assignment + makes assignee required
    await formPage.clickNext(); // Basics -> Assignment
    expect(await formPage.activeStepIndex()).toBe(1);

    // assignee is empty and required_when is true -> Next is blocked.
    await formPage.clickNext();
    expect(await formPage.activeStepIndex()).toBe(1);
    expect(await formPage.hasValidationError()).toBeTruthy();

    // Fill it and Next advances.
    await formPage.fillField("assignee", "alice");
    await formPage.clickNext();
    expect(await formPage.activeStepIndex()).toBe(2);
  });

  test("Create with a missing required field on an earlier step flags it and jumps back", async ({
    appPage,
  }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_wizard");

    // Leave the required `title` (step 0) empty; jump to the last step (Status,
    // index 1 while Assignment is hidden) and try to Create.
    await formPage.clickStep(1);
    expect(await formPage.activeStepIndex()).toBe(1);
    await formPage.clickCreate();

    // Bounced back to step 0, which is flagged, with a summary shown.
    expect(await formPage.activeStepIndex()).toBe(0);
    expect(await formPage.erroredStepIndices()).toContain(0);
    const summary = await formPage.errorSummaryText();
    expect(summary).toContain("attention");
    expect(await formPage.hasValidationError()).toBeTruthy();

    // Fixing the field clears the flag live.
    await formPage.fillField("title", "Now valid");
    expect(await formPage.erroredStepIndices()).not.toContain(0);
  });

  test("submits a wizard and drops hidden-branch values", async ({
    appPage,
    api,
  }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_wizard");

    // Leave `done` unchecked, so the Assignment step (assignee) stays hidden.
    await formPage.fillField("title", "Hidden branch task");
    await formPage.clickNext(); // -> Status (index 1, since Assignment hidden)
    await formPage.selectField("status", "approved");

    const created = await formPage.submitAndExpectCreate("tasks");
    createdTasks.push(created.id);

    const entity = await api.getEntity("tasks", created.id);
    expect(entity.properties.title).toBe("Hidden branch task");
    expect(entity.properties.status).toBe("approved");
    // assignee belonged to a hidden step -> not persisted.
    expect(entity.properties.assignee ?? "").toBe("");
  });

  test("a revealed-then-hidden field is NOT persisted on create", async ({
    appPage,
    api,
  }) => {
    // The real hidden-branch case: the user reveals the branch, fills it, then
    // hides it again before submitting. The filled value must not persist.
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_wizard");

    await formPage.fillField("title", "Toggled branch task");
    await formPage.setCheckbox("done", true); // reveal Assignment
    await formPage.clickNext(); // -> Assignment
    await formPage.fillField("assignee", "alice"); // fill the conditional field
    await formPage.clickBack(); // -> Basics
    await formPage.setCheckbox("done", false); // hide Assignment again
    // Now the only visible steps are Basics + Status.
    await formPage.clickNext(); // -> Status
    await formPage.selectField("status", "approved");

    const created = await formPage.submitAndExpectCreate("tasks");
    createdTasks.push(created.id);

    const entity = await api.getEntity("tasks", created.id);
    expect(entity.properties.title).toBe("Toggled branch task");
    // assignee was entered but its step ended up hidden -> dropped.
    expect(entity.properties.assignee ?? "").toBe("");
  });
});
