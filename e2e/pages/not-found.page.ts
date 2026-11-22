import { type Locator, expect } from "@playwright/test";
import { BasePage } from "./base.page";

/** The plain not-found state an address that names nothing shows. */
export class NotFoundPage extends BasePage {
  get state(): Locator {
    return this.page.getByTestId("not-found-state");
  }

  get homeLink(): Locator {
    return this.page.getByTestId("not-found-home");
  }

  /** The Retry button an error state offers. */
  get retryButton(): Locator {
    return this.page.getByRole("button", { name: "Retry" });
  }

  /** Open a path and wait for the not-found state. */
  async open(path: string) {
    await this.navigateTo(path);
    await expect(this.state).toBeVisible();
  }
}
