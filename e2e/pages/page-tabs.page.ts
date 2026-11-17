import { type Locator, expect } from "@playwright/test";
import { BasePage } from "./base.page";

/** A page of `pages:`: several views of one subject as tabs (TKT-ITQ0HL). */
export class PageTabsPage extends BasePage {
  /** The sidebar row of a navigation entry, by its exact label. */
  sidebarLink(label: string): Locator {
    return this.page.locator("#main-sidebar").getByRole("link", { name: label, exact: true });
  }

  /** The tabs of the header's tab bar. */
  get tabs(): Locator {
    return this.page.getByRole("tablist", { name: "View" }).getByRole("tab");
  }

  tab(label: string): Locator {
    return this.page.getByRole("tab", { name: label, exact: true });
  }

  async openFromSidebar(label: string) {
    await this.sidebarLink(label).click();
  }

  async selectTab(label: string) {
    await this.tab(label).click();
  }

  async expectTabs(labels: string[]) {
    await expect(this.tabs).toHaveText(labels);
  }

  async expectActiveTab(label: string) {
    await expect(this.tab(label)).toHaveAttribute("aria-selected", "true");
  }

  async expectNoTabBar() {
    await expect(this.page.getByRole("tablist", { name: "View" })).toHaveCount(0);
  }

  async expectTitle(title: string) {
    await expect(this.page.locator("h1").filter({ hasText: title }).first()).toBeVisible();
  }

  async expectSidebarActive(label: string) {
    await expect(this.sidebarLink(label)).toHaveAttribute("aria-current", "page");
  }

  async expectSidebarAbsent(label: string) {
    await expect(this.sidebarLink(label)).toHaveCount(0);
  }

  async expectNotFound(pageId: string) {
    await expect(this.page.getByText("Page not found")).toBeVisible();
    await expect(this.page.getByText(`There is no page “${pageId}” in the configuration.`)).toBeVisible();
  }

  async goBack() {
    await this.page.goBack();
  }

  async reload() {
    await this.page.reload();
  }

  /** Leave the entity form a tab opened. An autosaving form labels its
   *  Cancel button "Back". */
  async cancelForm() {
    await this.page.getByRole("button", { name: /^(Cancel|Back) Esc$/ }).click();
  }

  // Steps pages (TKT-35BY3PE).

  /** The step links of a steps page, in order. */
  get steps(): Locator {
    return this.page.getByRole("navigation", { name: "Steps" }).getByRole("link");
  }

  step(label: string): Locator {
    return this.page.getByRole("navigation", { name: "Steps" }).getByRole("link", { name: label });
  }

  async expectSteps(labels: RegExp[]) {
    await expect(this.steps).toHaveText(labels);
  }

  async expectCurrentStep(label: string) {
    await expect(this.step(label)).toHaveAttribute("aria-current", "step");
  }

  async expectPrompt(text: string) {
    await expect(this.page.getByRole("heading", { name: text, level: 2 })).toBeVisible();
  }

  stepButton(name: "Back" | "Next" | "Present" | "Exit"): Locator {
    return this.page.getByRole("button", { name, exact: true });
  }

  async clickStepButton(name: "Back" | "Next" | "Present" | "Exit") {
    await this.stepButton(name).click();
  }

  async pressKey(key: string) {
    await this.page.keyboard.press(key);
  }

  // A step that walks its rows (TKT-M07C97Y).

  /** The previous or next row link of a walking step. */
  walkLink(name: "Previous row" | "Next row"): Locator {
    return this.page.getByRole("navigation", { name: "Rows of this step" }).getByRole("link", { name, exact: true });
  }

  async expectWalkPosition(text: string) {
    await expect(this.page.getByTestId("walk-position")).toHaveText(text);
  }

  /** The walked row's detail view shows this exact text. */
  async expectWalkShows(text: string) {
    await expect(this.page.getByText(text, { exact: true })).toBeVisible();
  }

  /** The walk names a row that is not in the step's list; go to its first row. */
  async followWalkMissingRow() {
    await expect(this.page.getByText("Not in this list")).toBeVisible();
    await this.page.getByRole("link", { name: "Go to the first row" }).click();
  }

  /** Whether a step's number shows its done check (TKT-MWZF0UO). */
  async expectStepDone(label: string, done: boolean) {
    if (done) await expect(this.step(label)).toHaveAttribute("data-done", "true");
    else await expect(this.step(label)).not.toHaveAttribute("data-done");
  }

  async expectSidebarShown(shown: boolean) {
    await expect(this.page.locator("#main-sidebar")).toHaveCount(shown ? 1 : 0);
  }
}
