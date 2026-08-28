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

  test("error flags persist across navigation until each is resolved", async ({
    appPage,
  }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_wizard");

    // Both `title` (step 0) and `status` (step 1, Assignment hidden) are
    // required and empty. Go to the last step (where Create lives) and submit.
    await formPage.clickStep(1);
    await formPage.clickCreate();
    expect(await formPage.erroredStepIndices()).toEqual([0, 1]);

    // Fix step 0's field, then navigate to step 1. Step 1's flag must persist
    // (navigating away from step 0 must NOT wipe step 1's error).
    await formPage.fillField("title", "Has a title now");
    expect(await formPage.erroredStepIndices()).toEqual([1]);
    await formPage.clickStep(1);
    expect(await formPage.erroredStepIndices()).toEqual([1]);

    // Fixing step 1 clears the last flag.
    await formPage.selectField("status", "approved");
    expect(await formPage.erroredStepIndices()).toEqual([]);
  });

  test("hiding an errored step clears its flag and summary (RR-U9ERK)", async ({
    appPage,
  }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_wizard");

    // Reveal Assignment (assignee required), fill Basics + Status, leave
    // assignee empty, then Create -> Assignment is flagged.
    await formPage.fillField("title", "Will hide the error");
    await formPage.setCheckbox("done", true); // reveal Assignment
    await formPage.clickStep(2); // Status
    await formPage.selectField("status", "approved");
    await formPage.clickCreate(); // assignee empty -> error, jumps to Assignment
    expect((await formPage.erroredStepIndices()).length).toBeGreaterThan(0);
    expect(await formPage.errorSummaryText()).not.toBeNull();

    // Go back to Basics and toggle `done` off -> the Assignment step (and its
    // error) disappears. No phantom summary should remain.
    await formPage.clickStep(0);
    await formPage.setCheckbox("done", false);
    expect(await formPage.erroredStepIndices()).toEqual([]);
    expect(await formPage.errorSummaryText()).toBeNull();
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

  test("wizard EDIT mode autosaves and has no Save button (TKT-ZKGY3)", async ({
    appPage,
    api,
  }) => {
    // Seed a task, then open it in the wizard form in EDIT mode.
    const created = await api.createEntity("tasks", {
      properties: { title: "Editable via wizard", status: "approved" },
    });
    createdTasks.push(created.id);

    const formPage = new FormPage(appPage);
    await formPage.navigateToEditForm("task_wizard", created.id);

    // No explicit Save/Create button — edit is autosave, same as a flat form.
    expect(await formPage.hasSubmitButton()).toBeFalsy();

    // Editing a field autosaves via PATCH (no submit).
    await formPage.fillField("title", "Retitled in wizard");
    await formPage.saveAndWaitForPatch("tasks", created.id);
    const entity = await api.getEntity("tasks", created.id);
    expect(entity.properties.title).toBe("Retitled in wizard");
  });

  // BUG-FB0LN8. This test previously asserted the OPPOSITE — that hiding a
  // branch unsets its stored value (AC5, TKT-ZKGY3). That behavior was the bug:
  // it destroyed data the user never touched, and the render gate then refused
  // to draw the field again, so the loss was silent and unrecoverable. Hiding
  // is presentation; `clear_when_hidden` (default `no`) now owns the value's
  // fate. The old behavior is still available and pinned below under `yes`.
  test("edit mode KEEPS a field whose branch is hidden (BUG-FB0LN8)", async ({
    appPage,
    api,
  }) => {
    // Seed a task with the conditional branch active and its field filled.
    const created = await api.createEntity("tasks", {
      properties: {
        title: "Has assignee",
        status: "approved",
        done: true,
        assignee: "alice",
      },
    });
    createdTasks.push(created.id);

    const formPage = new FormPage(appPage);
    await formPage.navigateToEditForm("task_wizard", created.id);

    // `done` lives on the first step (Basics); toggling it off hides the whole
    // Assignment step (which carries `assignee`). The user never touched
    // `assignee`, so its stored value must survive.
    await formPage.setCheckbox("done", false);
    await formPage.saveAndWaitForPatch("tasks", created.id);

    const entity = await api.getEntity("tasks", created.id);
    expect(entity.properties.done).toBe(false);
    expect(entity.properties.assignee).toBe("alice"); // NOT destroyed by hiding
  });

  // The reveal direction — the half of the round-trip nothing covered before,
  // and the half that made BUG-FB0LN8 unrecoverable rather than merely lossy.
  test("a hidden field reappears with its value when its branch is revealed (BUG-FB0LN8)", async ({
    appPage,
    api,
  }) => {
    const created = await api.createEntity("tasks", {
      properties: {
        title: "Round trip",
        status: "approved",
        done: true,
        assignee: "alice",
      },
    });
    createdTasks.push(created.id);

    const formPage = new FormPage(appPage);
    await formPage.navigateToEditForm("task_wizard", created.id);

    // Hide the branch...
    await formPage.setCheckbox("done", false);
    await formPage.saveAndWaitForPatch("tasks", created.id);

    // ...then reveal it again. The field must come back, holding its value.
    await formPage.setCheckbox("done", true);
    await formPage.clickStep(1);
    expect(await formPage.isFieldVisible("assignee")).toBe(true);
    await formPage.expectFieldValue("assignee", "alice");

    const entity = await api.getEntity("tasks", created.id);
    expect(entity.properties.assignee).toBe("alice");
  });

  // Survives a full reload, not just an in-session toggle: the value was never
  // deleted server-side, so a fresh form renders it from storage.
  test("a hidden-then-revealed field survives a page reload (BUG-FB0LN8)", async ({
    appPage,
    api,
  }) => {
    const created = await api.createEntity("tasks", {
      properties: {
        title: "Reload safe",
        status: "approved",
        done: true,
        assignee: "alice",
      },
    });
    createdTasks.push(created.id);

    const formPage = new FormPage(appPage);
    await formPage.navigateToEditForm("task_wizard", created.id);
    await formPage.setCheckbox("done", false);
    await formPage.saveAndWaitForPatch("tasks", created.id);

    // Reload with the branch hidden, then reveal it in the fresh session.
    await formPage.navigateToEditForm("task_wizard", created.id);
    await formPage.setCheckbox("done", true);
    await formPage.clickStep(1);
    await formPage.expectFieldValue("assignee", "alice");
  });

  // The opt-in escape hatch: `clear_when_hidden: yes` restores the old
  // destructive behavior for anyone who genuinely wanted it.
  test("clear_when_hidden:yes clears the value when the branch hides (BUG-FB0LN8)", async ({
    appPage,
    api,
  }) => {
    const created = await api.createEntity("tasks", {
      properties: {
        title: "Clears on hide",
        status: "approved",
        done: true,
        assignee: "alice",
      },
    });
    createdTasks.push(created.id);

    const formPage = new FormPage(appPage);
    await formPage.navigateToEditForm("task_clear_when_hidden", created.id);
    await formPage.setCheckbox("done", false);
    await formPage.saveAndWaitForPatch("tasks", created.id);

    const entity = await api.getEntity("tasks", created.id);
    expect(entity.properties.assignee ?? "").toBe(""); // opted in to clearing
  });

  // Mixed policies hidden by ONE trigger: each field must honor its own
  // setting rather than the batch taking a single decision for all of them.
  test("mixed clear policies each apply on a single hide (BUG-FB0LN8)", async ({
    appPage,
    api,
  }) => {
    const created = await api.createEntity("tasks", {
      properties: {
        title: "Mixed policies",
        status: "approved",
        done: true,
        assignee: "alice",
        note: "keep me",
      },
    });
    createdTasks.push(created.id);

    const formPage = new FormPage(appPage);
    await formPage.navigateToEditForm("task_mixed_policies", created.id);

    // Hides `note` + `assignee` (default keep) and `done` (clear) in one pass.
    await formPage.selectField("status", "draft");
    await formPage.saveAndWaitForPatch("tasks", created.id);

    const entity = await api.getEntity("tasks", created.id);
    expect(entity.properties.note).toBe("keep me");
    expect(entity.properties.assignee).toBe("alice");
    expect(entity.properties.done ?? "").toBe(""); // only the yes-policy field cleared

    // And the keep-policy fields come back when the branch is revealed.
    await formPage.selectField("status", "approved");
    await formPage.expectFieldValue("assignee", "alice");
    await formPage.expectFieldValue("note", "keep me");
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

  test("a relation under a revealed-then-hidden step is NOT persisted", async ({
    appPage,
    api,
  }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_wizard");

    await formPage.fillField("title", "Relation branch task");
    await formPage.setCheckbox("done", true); // reveal Assignment (has `implements`)
    await formPage.clickNext(); // -> Assignment
    await formPage.fillField("assignee", "carol");
    await formPage.addRelation("Implements", "FEAT-001"); // link under the branch
    await formPage.clickBack(); // -> Basics
    await formPage.setCheckbox("done", false); // hide the whole Assignment step
    await formPage.clickNext(); // -> Status
    await formPage.selectField("status", "approved");

    const created = await formPage.submitAndExpectCreate("tasks");
    createdTasks.push(created.id);

    // The relation belonged to a hidden step -> not persisted (mirrors props).
    const rels = await api.listRelations("tasks", created.id, "implements");
    expect(rels).toEqual([]);
  });
});

// After unifying flat + wizard (TKT-ZKGY3), single-page forms run through the
// same step model, so visible_when/required_when work there too — with NO
// stepper bar and NO ?step param.
test.describe("Flat forms with conditions", () => {
  const createdTasks: string[] = [];
  test.afterEach(async ({ api }) => {
    while (createdTasks.length) {
      const id = createdTasks.pop()!;
      await api.deleteEntity("tasks", id).catch(() => {});
    }
  });

  test("no stepper bar and no ?step on a single-page form", async ({
    appPage,
  }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_flat_conditional");
    expect(await formPage.wizardStepTitles()).toEqual([]); // no pills
    expect(formPage.readStepParam()).toBeNull(); // no ?step
    expect(await formPage.isFieldVisible("title")).toBeTruthy();
  });

  test("visible_when reveals a field inline on a flat form", async ({
    appPage,
  }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_flat_conditional");

    expect(await formPage.isFieldVisible("assignee")).toBeFalsy();
    await formPage.setCheckbox("done", true);
    expect(await formPage.isFieldVisible("assignee")).toBeTruthy();
    await formPage.setCheckbox("done", false);
    expect(await formPage.isFieldVisible("assignee")).toBeFalsy();
  });

  test("required_when blocks Create until the revealed field is filled", async ({
    appPage,
    api,
  }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm("task_flat_conditional");

    await formPage.fillField("title", "Flat conditional");
    await formPage.setCheckbox("done", true); // assignee now required

    // Empty assignee blocks Create.
    await formPage.clickCreate();
    expect(await formPage.hasValidationError()).toBeTruthy();

    // Fill it, then Create succeeds and persists the value.
    await formPage.fillField("assignee", "bob");
    const created = await formPage.submitAndExpectCreate("tasks");
    createdTasks.push(created.id);
    const entity = await api.getEntity("tasks", created.id);
    expect(entity.properties.assignee).toBe("bob");
  });
});
