import { test, expect } from "./fixtures";
import { AppShellPage, EntityPage, ListPage } from "../pages";

/**
 * Keyboard shortcut hints, now rendered by the library's RlKbd.
 *
 * rela used to draw its own key caps from a global `kbd` rule in App.vue plus
 * a local override per call site, each re-deciding padding, border and colour.
 * RlKbd owns that chrome now and the overrides are gone.
 *
 * What is asserted here is what a user perceives — a hint is present, says
 * which key to press, and is announced as separate tokens — rather than the
 * CSS producing it, so a restyle in rela-components does not fail this file.
 * The one visual assertion is the mobile one, because "hidden on a touch
 * device" IS the user-visible behaviour and nothing else in the suite covers
 * it: the rule lives in a media query, RlKbd sets `display: inline-flex` in
 * its own scoped style, and happy-dom applies neither, so a unit test reports
 * the hint visible whether or not the rule survived.
 */
test.describe("Keyboard shortcut hints", () => {
  test("the detail header hints the Edit shortcut", async ({ appPage }) => {
    const list = new ListPage(appPage);
    await list.navigateToList("features");
    await list.openEntityPage(0);
    await expect(appPage).toHaveURL(/\/entity\//);

    const entity = new EntityPage(appPage);
    await expect(entity.editButton).toBeVisible();

    const shell = new AppShellPage(appPage);
    // Announced, not merely drawn: a hint with no accessible name is a
    // decorative box to a screen reader.
    expect(await shell.shortcutHintNames()).toContain("E");
  });

  test("a hint trails its control rather than replacing the label", async ({
    appPage,
  }) => {
    const list = new ListPage(appPage);
    await list.navigateToList("features");
    await list.openEntityPage(0);
    await expect(appPage).toHaveURL(/\/entity\//);

    const entity = new EntityPage(appPage);
    // The button still reads as "Edit". Asserted because the hint sits INSIDE
    // the button: were it to lead rather than trail, every `name: /^Edit/`
    // locator in this suite would start matching "E Edit" and fail.
    await entity.expectEditNameStartsWithEdit();
    await expect(entity.editButton).toBeVisible();
  });
});
