import fs from "node:fs";
import path from "node:path";
import { test as base, expect } from "./fixtures";
import { DashboardPage } from "../pages";

/**
 * Dashboard cards render values as list cells do (TKT-69PZ09D): a breakdown
 * on an enum shows the authored labels as styled badges in declared order,
 * with bars in the style colour; a table shows enum badges, currency amounts
 * and formatted dates. The locale is pinned so amounts and dates read one way.
 *
 * The spec gives `feature_status` labels and `bug` a date property in its own
 * copy of the project, and adds a named dashboard before the server starts.
 */
const DASHBOARD_YAML = `
dashboards:
  rendering:
    title: "Rendering"
    cards:
      - title: "Bugs by status"
        query: "type:bug Dashdeal"
        display: breakdown
        group_by: status
      - title: "Bug table"
        query: "type:bug Dashdeal"
        display: table
        width: wide
        columns:
          - property: title
            label: "Title"
            link: detail
          - property: status
            label: "Status"
          - property: estimate
            label: "Estimate"
          - property: deadline
            label: "Deadline"
        sort:
          - property: title
            direction: asc
`;

const STATUS_TYPE = `  feature_status:
    values: [draft, approved, in_progress, done]
    default: draft
`;

const STATUS_TYPE_LABELLED = `${STATUS_TYPE}    labels:
      draft: Nieuw
      approved: Goedgekeurd
      in_progress: Bezig
      done: Klaar
`;

const ESTIMATE = `      estimate:
        type: integer
        display:
          currency: EUR
`;

const test = base.extend({
  testProject: async ({ testProject }, use) => {
    const schemaPath = path.join(testProject, "schema.yaml");
    const schema = fs.readFileSync(schemaPath, "utf8");
    if (!schema.includes(STATUS_TYPE) || !schema.includes(ESTIMATE)) {
      throw new Error("fixture schema changed: update dashboard-card-rendering.spec.ts");
    }
    fs.writeFileSync(
      schemaPath,
      schema
        .replace(STATUS_TYPE, STATUS_TYPE_LABELLED)
        .replace(ESTIMATE, `${ESTIMATE}      deadline:\n        type: date\n`),
    );
    fs.appendFileSync(path.join(testProject, "data-entry.yaml"), DASHBOARD_YAML);
    await use(testProject);
  },
});

test.use({ locale: "nl-NL" });

test.describe("Dashboard card value rendering", () => {
  test("breakdown and table render labels, styles, amounts and dates", async ({ appPage, api }) => {
    const first = await api.createEntity("bugs", {
      properties: { title: "Dashdeal A", status: "in_progress", estimate: 3500, deadline: "2026-03-01" },
    });
    await api.createEntity("bugs", { properties: { title: "Dashdeal B", status: "in_progress" } });
    await api.createEntity("bugs", { properties: { title: "Dashdeal C", status: "in_progress" } });
    await api.createEntity("bugs", { properties: { title: "Dashdeal D", status: "done" } });
    await api.createEntity("bugs", { properties: { title: "Dashdeal E", status: "draft" } });

    const dashboard = new DashboardPage(appPage);
    await dashboard.navigateNamed("rendering");

    // Declared order, not count order: Bezig has the most rows.
    await dashboard.expectBreakdownLabels("Bugs by status", ["Nieuw", "Bezig", "Klaar"]);
    await expect(dashboard.card("Bugs by status").locator(".breakdown-label .badge")).toHaveCount(3);
    const bars = await dashboard.breakdownBarStyles("Bugs by status");
    expect(bars[0]).toContain("--rl-color-status-blue");
    expect(bars[1]).toContain("--rl-color-status-amber");
    expect(bars[2]).toContain("--rl-color-status-green");

    const cells = dashboard.tableRow("Bug table", "Dashdeal A").locator("td");
    await expect(cells.nth(0).locator("a")).toHaveAttribute("href", new RegExp(`/entity/bug/${first.id}$`));
    await expect(cells.nth(1).locator(".badge")).toHaveText("Bezig");
    await expect(cells.nth(1).locator(".badge")).toHaveClass(/badge--orange/);
    await expect(cells.nth(2)).toHaveText(/^€\s3\.500$/);
    await expect(cells.nth(3)).toHaveText(/^1 mrt 2026$/);
  });
});
