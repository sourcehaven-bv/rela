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
}
