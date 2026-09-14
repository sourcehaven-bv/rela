// "Create & add another" (TKT-7YHKD1) end to end.
//
// The unit tests drive the reset against a mocked store; what they cannot show
// is that TWO DISTINCT entities actually reach the server with the right
// values. That is the failure this file exists to catch: a reset that silently
// carried record 1's values into record 2 would satisfy every DOM assertion in
// the unit suite up to the moment the second POST goes out.
//
// Compare with create-redirect.spec.ts, whose "rapid create" test loops
// navigateToCreateForm three times — that navigation loop is the cost this
// feature removes.

import { test, expect } from "./fixtures";
import { FormPage, ListPage } from "../pages";

test.describe("Create & add another", () => {
  test("creates two entities without leaving the form", async ({ appPage }) => {
    const formPage = new FormPage(appPage);

    await formPage.navigateToCreateForm("task_add_another");
    await formPage.fillFields({ title: "First task" });
    const first = await formPage.addAnotherAndExpectCreate("tasks");

    // Still on the create form — no navigation to the entity detail page.
    await expect(appPage).toHaveURL(/\/form\/task_add_another/);

    // ...and blank, ready for the next record.
    await expect(formPage.titleInput).toHaveValue("");

    await formPage.fillFields({ title: "Second task" });
    const second = await formPage.addAnotherAndExpectCreate("tasks");

    expect(second.id).not.toBe(first.id);

    // Both exist, with their own titles. A reset that leaked record 1 into
    // record 2 would show the first title twice — asserting per ROW (rather
    // than anywhere on the page) is what makes that visible.
    const listPage = new ListPage(appPage);
    await listPage.navigateToList("tasks");
    await listPage.expectCellInRow(first.id, "First task");
    await listPage.expectCellInRow(second.id, "Second task");
  });

  test("keeps a keep_on_add_another field and clears the rest", async ({
    appPage,
  }) => {
    const formPage = new FormPage(appPage);

    await formPage.navigateToCreateForm("task_add_another");
    await formPage.fillFields({ title: "Task one", assignee: "alice" });
    await formPage.addAnotherAndExpectCreate("tasks");

    // `assignee` is marked keep_on_add_another; `title` is not.
    await expect(formPage.titleInput).toHaveValue("");
    await formPage.expectFieldValue("assignee", "alice");

    // The kept value must actually be SENT, not merely displayed — if it were
    // re-applied without being marked touched, the commit filter would drop it.
    await formPage.fillFields({ title: "Task two" });
    const created = (await formPage.addAnotherAndExpectCreate("tasks")) as {
      id: string;
      properties: Record<string, unknown>;
    };
    expect(created.properties.assignee).toBe("alice");
    expect(created.properties.title).toBe("Task two");
  });

  test("keeps a marked relation and can still save the next record", async ({
    appPage,
  }) => {
    // A kept RELATION exercises a path a kept property does not: the create
    // payload needs the picker's id -> type map to build a JSON:API resource
    // identifier, and the reset clears that map. Carrying the ids without the
    // types made the SECOND save abort with "unknown types, reload the form"
    // while the widget still showed the peer — i.e. it looked fine and then
    // silently lost the record.
    const formPage = new FormPage(appPage);

    await formPage.navigateToCreateForm("task_add_another");
    await formPage.fillFields({ title: "Linked one" });
    await formPage.addRelation("Implements Feature", "FEAT-001");
    await formPage.addAnotherAndExpectCreate("tasks");

    await formPage.fillFields({ title: "Linked two" });
    const second = (await formPage.addAnotherAndExpectCreate("tasks")) as {
      id: string;
    };

    // The second entity really carries the kept relation.
    const origin = new URL(appPage.url()).origin;
    const detail = await appPage.request.get(
      `${origin}/api/v1/tasks/${second.id}?include=implements`,
    );
    expect(detail.ok()).toBeTruthy();
    expect(JSON.stringify(await detail.json())).toContain("FEAT-001");
  });

  test("the primary Create still navigates to the new entity", async ({
    appPage,
  }) => {
    // The second outcome must not have changed the first one.
    const formPage = new FormPage(appPage);

    await formPage.navigateToCreateForm("task_add_another");
    await formPage.fillFields({ title: "Navigating task" });
    await formPage.submitAndExpectCreate("tasks");

    await expect(appPage).toHaveURL(/\/entity\/task\/TASK-\d+/);
  });
});
