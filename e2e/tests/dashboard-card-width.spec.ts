import fs from "node:fs";
import path from "node:path";
import { test as base, expect } from "./fixtures";
import { DashboardPage } from "../pages";

/**
 * Dashboard card width (TKT-H21TYS): `width: wide` spans two grid tracks, or
 * the whole row when the grid has fewer; `width: full` spans the whole row.
 * jsdom has no layout, so the geometry is measured here.
 *
 * The spec adds a named dashboard to the shared project before the server
 * starts: a normal card, a wide one, a normal one, a full one.
 */
const WIDTH_YAML = `
dashboards:
  widths:
    title: "Widths"
    cards:
      - title: "Normal A"
        query: "type:feature"
        display: count
      - title: "Wide"
        query: "type:feature"
        display: count
        width: wide
      - title: "Normal B"
        query: "type:feature"
        display: count
      - title: "Full"
        query: "type:feature"
        display: count
        width: full
`;

const test = base.extend({
  testProject: async ({ testProject }, use) => {
    fs.appendFileSync(path.join(testProject, "data-entry.yaml"), WIDTH_YAML);
    await use(testProject);
  },
});

interface Geometry {
  gridWidth: number;
  tracks: number[];
  gap: number;
  overflows: boolean;
  tiles: Record<string, { x: number; width: number }>;
}

async function measure(page: import("@playwright/test").Page): Promise<Geometry> {
  return page.evaluate(() => {
    const grid = document.querySelector<HTMLElement>(".dashboard-grid")!;
    const style = getComputedStyle(grid);
    const tiles: Record<string, { x: number; width: number }> = {};
    for (const el of document.querySelectorAll<HTMLElement>(".dashboard-card")) {
      const r = el.getBoundingClientRect();
      tiles[el.querySelector("h3")!.textContent!.trim()] = { x: r.x, width: r.width };
    }
    return {
      gridWidth: grid.getBoundingClientRect().width,
      tracks: style.gridTemplateColumns.split(" ").map(parseFloat),
      gap: parseFloat(style.columnGap),
      overflows: grid.scrollWidth > grid.clientWidth + 1,
      tiles,
    };
  });
}

test.describe("Dashboard card width", () => {
  for (const width of [1440, 900, 700, 375]) {
    test(`tiles take their width without overflow at ${width}px`, async ({ appPage }) => {
      await appPage.setViewportSize({ width, height: 1000 });
      const dashboard = new DashboardPage(appPage);
      await dashboard.navigateNamed("widths");
      await dashboard.expectCardTitles(["Normal A", "Wide", "Normal B", "Full"]);

      const g = await measure(appPage);
      expect(g.overflows).toBe(false);
      for (const tile of Object.values(g.tiles)) {
        expect(tile.width).toBeLessThanOrEqual(g.gridWidth + 1);
      }

      // Two tracks, or the whole row when the grid has only one.
      const track = g.tracks[0];
      const row = g.gridWidth;
      const two = Math.min(2 * track + g.gap, row);
      expect(g.tiles["Normal A"].width).toBeCloseTo(track, 0);
      expect(g.tiles["Wide"].width).toBeCloseTo(two, 0);
      expect(g.tiles["Full"].width).toBeCloseTo(row, 0);
    });
  }
});
