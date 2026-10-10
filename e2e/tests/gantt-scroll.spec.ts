import { test, expect } from './fixtures';
import { GanttPage } from '../pages';

/** Local calendar date `days` from today, as YYYY-MM-DD. */
function isoFromToday(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

const TREE_W = 280;
const MONTH_PX_PER_DAY = 12;

test.describe('Gantt scroll and navigation', () => {
  test.beforeEach(async ({ appPage }) => {
    // Instant scrolling: the buttons animate otherwise.
    await appPage.emulateMedia({ reducedMotion: 'reduce' });
  });

  test('scrolls sideways with Now and the arrows; the tree column stays put', async ({ appPage, api }) => {
    // 300 days at the month zoom's 12px per day is far wider than the screen.
    const plan = await api.createEntity('plans', {
      properties: { title: 'Long roadmap', start: isoFromToday(-150), end: isoFromToday(150) },
    });
    const gantt = new GanttPage(appPage);
    await gantt.navigateToGantt('roadmap');

    const chartBox = await gantt.chart.boundingBox();
    const viewportW = chartBox!.width - TREE_W;
    const centre = TREE_W + viewportW / 2;

    // Opens at today, centred in the visible timeline.
    await expect.poll(async () => Math.abs((await gantt.xInChart(gantt.todayLine())) - centre)).toBeLessThan(15);

    const start = await gantt.scrollLeft();
    // Room to scroll one month back from today, or the Earlier check clamps at 0.
    expect(start).toBeGreaterThanOrEqual(30 * MONTH_PX_PER_DAY);
    await gantt.laterButton.click();
    await expect.poll(() => gantt.scrollLeft()).toBe(start + 30 * MONTH_PX_PER_DAY);
    await gantt.earlierButton.click();
    await gantt.earlierButton.click();
    await expect.poll(() => gantt.scrollLeft()).toBe(start - 30 * MONTH_PX_PER_DAY);

    // The tree column is sticky: still at the chart's left edge while scrolled.
    expect(await gantt.xInChart(gantt.rowTreeCell(plan.id))).toBeLessThan(2);
    await expect(gantt.rowTreeCell(plan.id)).toContainText('Long roadmap');
    // The bar starts far off-screen; its name sticks beside the tree column.
    const label = gantt.barLabel(plan.id);
    await expect(label).toBeVisible();
    expect(await gantt.xInChart(label)).toBeGreaterThanOrEqual(TREE_W);

    await gantt.nowButton.click();
    await expect.poll(() => gantt.scrollLeft()).toBe(start);
  });

  test('a zoom change keeps the day at the left edge', async ({ appPage, api }) => {
    await api.createEntity('plans', {
      properties: { title: 'Zoom roadmap', start: isoFromToday(-150), end: isoFromToday(150) },
    });
    const gantt = new GanttPage(appPage);
    await gantt.navigateToGantt('roadmap');

    await gantt.setScrollLeft(1200);
    await expect.poll(() => gantt.scrollLeft()).toBe(1200);
    await gantt.zoomButton('week').click();
    // 1200px at 12px/day is 100 days in; at 40px/day that is 4000px.
    await expect.poll(() => gantt.scrollLeft()).toBe(4000);
  });
});
