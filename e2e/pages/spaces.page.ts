import { type Locator } from "@playwright/test";
import { BasePage } from "./base.page";

/** Spaces (TKT-GNKR5H): the sidebar's space switcher and per-space Create menu. */
export class SpacesPage extends BasePage {
  /** A sidebar navigation link, by its exact label. */
  sidebarLink(label: string): Locator {
    return this.page.locator("#main-sidebar").getByRole("link", { name: label, exact: true });
  }

  /** Open the switcher, whose button names the current space, and pick `target`. */
  async switchSpace(current: RegExp, target: string) {
    await this.page.locator("#main-sidebar").getByRole("button", { name: current }).click();
    await this.page.getByRole("menuitem", { name: target }).click();
  }

  async openCreateMenu() {
    await this.page.getByTestId("space-create").click();
  }

  /** Every item of the open menu. */
  get menuItems(): Locator {
    return this.page.getByRole("menuitem");
  }
}
