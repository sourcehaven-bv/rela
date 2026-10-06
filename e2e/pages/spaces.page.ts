import { expect, type Locator } from "@playwright/test";
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

  /**
   * Create an entity through the Create menu: pick `item`, fill the title in
   * the create dialog and save. Returns the new id once the entity's own page
   * opens, which happens after the page link (if any) is made.
   */
  async createFromMenu(item: string, plural: string, title: string): Promise<string> {
    await this.openCreateMenu();
    await this.page.getByRole("menuitem", { name: item }).click();
    const dialog = this.page.locator(".inline-create-panel");
    await dialog.locator("#field-title").fill(title);
    // The form also POSTs to the same path with ?dry_run=true; skip those.
    const created = this.page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === `/api/v1/${plural}` &&
        !r.url().includes("dry_run") &&
        r.request().method() === "POST",
    );
    await dialog.locator('button[type="submit"]').click();
    const { id } = (await (await created).json()) as { id: string };
    await expect(this.page).toHaveURL(new RegExp(`/entity/[^/]+/${id}$`));
    return id;
  }
}
