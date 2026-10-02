import { test, expect } from "./fixtures";
import { AppShellPage } from "../pages";

test.describe("Sidebar width", () => {
  test("drags from the keyboard and survives a reload", async ({ appPage }) => {
    const shell = new AppShellPage(appPage);
    const handle = shell.sidebarResizeHandle;
    await expect(handle).toHaveAttribute("aria-valuenow", "260");

    await handle.focus();
    await appPage.keyboard.press("PageUp");
    await expect(handle).toHaveAttribute("aria-valuenow", "324");
    await expect.poll(() => shell.sidebarWidth()).toBe(324);

    await appPage.reload();
    await expect(shell.sidebarResizeHandle).toHaveAttribute(
      "aria-valuenow",
      "324",
    );
  });
});
