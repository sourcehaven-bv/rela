import { test, expect } from './fixtures';
import { GanttPage } from '../pages';

/** Local calendar date `days` from today, as YYYY-MM-DD. */
function isoFromToday(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

test.describe('Gantt drag', () => {
  test.beforeEach(async ({ appPage }) => {
    await appPage.emulateMedia({ reducedMotion: 'reduce' });
  });

  test('moves a bar, resizes its end, and steps it from the keyboard', async ({ appPage, api }) => {
    const plan = await api.createEntity('plans', {
      properties: { title: 'Drag me', start: isoFromToday(-3), end: isoFromToday(3) },
    });
    const gantt = new GanttPage(appPage);
    await gantt.navigateToGantt('roadmap');
    await gantt.zoomButton('week').click();

    // Resting on the bar reads the entity; the handles appear once it allows the write.
    await gantt.barLabel(plan.id).hover();
    await appPage.locator(`.row[data-node-id="${plan.id}"] .bar`).hover();
    const move = gantt.dragHandle(plan.id, 'move');
    await expect(move).toBeVisible();
    // A short plan stretches to fill the screen, so measure a day off the
    // seven-day bar rather than assuming the zoom's minimum width.
    const day = (await move.boundingBox())!.width / 7;

    await gantt.dragBy(move, 2 * day);
    await expect
      .poll(async () => (await api.getEntity('plans', plan.id)).properties)
      .toMatchObject({ start: isoFromToday(-1), end: isoFromToday(5) });
    // A drag does not drill: the chart still shows the plan at the top level.
    await expect(gantt.rowTreeCell(plan.id)).toBeVisible();
    expect(appPage.url()).not.toContain('path=');

    // The handles stay through the reload; drag the end edge one day left.
    const end = gantt.dragHandle(plan.id, 'end');
    await expect(end).toBeVisible();
    await gantt.dragBy(end, -day);
    await expect
      .poll(async () => (await api.getEntity('plans', plan.id)).properties)
      .toMatchObject({ start: isoFromToday(-1), end: isoFromToday(4) });

    // Keyboard: focus the start slider, step two days right, write with Enter.
    await gantt.barLabel(plan.id).focus();
    const start = gantt.dragHandle(plan.id, 'start');
    await expect(start).toBeVisible();
    await start.focus();
    await appPage.keyboard.press('ArrowRight');
    await appPage.keyboard.press('ArrowRight');
    await appPage.keyboard.press('Enter');
    await expect
      .poll(async () => (await api.getEntity('plans', plan.id)).properties)
      .toMatchObject({ start: isoFromToday(1), end: isoFromToday(4) });
  });
});
