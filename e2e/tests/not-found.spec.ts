import { test, expect } from "./fixtures";
import { NotFoundPage } from "../pages";

// An entity, list or form address that names nothing shows the plain
// not-found state with a way home, not a raw server error with Retry.
test.describe("Not-found addresses", () => {
  for (const path of [
    "/entity/feature/FEAT-99999",
    "/list/no-such-list",
    "/form/no-such-form",
  ]) {
    test(`${path} shows the not-found state`, async ({ appPage }) => {
      const notFound = new NotFoundPage(appPage);
      await notFound.open(path);
      await expect(notFound.homeLink).toBeVisible();
      await expect(notFound.retryButton).toHaveCount(0);
      await expect(notFound.state).not.toContainText("not found:");
      await expect(notFound.state).not.toContainText("configuration");
    });
  }
});
