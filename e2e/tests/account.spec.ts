import fs from "node:fs";
import path from "node:path";
import { test as base, expect } from "./fixtures";
import { AppShellPage } from "../pages";

/**
 * The account menu (TKT-MJTD12): who the principal is, and links to pages the
 * login proxy owns. The fixture server runs as RELA_DATAENTRY_USER
 * e2e@example.com with no person entity, so the menu names the user id.
 */
const ACCOUNT_YAML = `
account:
  sign_out: /oauth2/sign_out
  account: https://id.example.com/account
`;

const test = base.extend({
  testProject: async ({ testProject }, use) => {
    fs.appendFileSync(path.join(testProject, "data-entry.yaml"), ACCOUNT_YAML);
    await use(testProject);
  },
});

test.describe("Account menu", () => {
  test("names the user and links to the proxy's pages", async ({ appPage }) => {
    const shell = new AppShellPage(appPage);
    const trigger = shell.accountMenu;
    await expect(trigger).toContainText("e2e@example.com");

    await trigger.click();
    await expect(shell.accountMenuItem("Account")).toHaveAttribute(
      "href",
      "https://id.example.com/account",
    );
    await expect(shell.accountMenuItem("Sign out")).toHaveAttribute(
      "href",
      "/oauth2/sign_out",
    );
    await expect(shell.accountMenuItem("Admin")).toHaveCount(0);
  });
});
