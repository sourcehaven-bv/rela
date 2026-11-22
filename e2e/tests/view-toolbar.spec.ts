import { test, expect } from './fixtures';
import { CalendarPage, GanttPage } from '../pages';

/** Local calendar date `days` from today, as YYYY-MM-DD. */
function isoFromToday(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

// The search box, Filters menu and Sort menu on calendars and gantts. Document
// views have no toolbar: a document is one rendered entity, not a list.

test.describe('Gantt toolbar', () => {
  test.beforeEach(async ({ appPage, api }) => {
    await appPage.emulateMedia({ reducedMotion: 'reduce' });
    await api.createEntity('plans', {
      properties: { title: 'Alpha launch', phase: 'draft', start: isoFromToday(-10), end: isoFromToday(10) },
    });
    await api.createEntity('plans', {
      properties: { title: 'Bravo launch', phase: 'active', start: isoFromToday(-20), end: isoFromToday(20) },
    });
    await api.createEntity('plans', {
      properties: { title: 'Charlie rollout', phase: 'active', start: isoFromToday(-5), end: isoFromToday(5) },
    });
  });

  test('search narrows the rows to the matches', async ({ appPage }) => {
    const gantt = new GanttPage(appPage);
    await gantt.navigateToGantt('roadmap');
    await expect(gantt.rows).toHaveCount(3);

    await gantt.search('launch');
    await expect(gantt.rows).toHaveCount(2);
    await expect(gantt.rowTreeCellWithText('Charlie rollout')).toHaveCount(0);
  });

  test('a filter narrows the rows and shows in the URL', async ({ appPage }) => {
    const gantt = new GanttPage(appPage);
    await gantt.navigateToGantt('roadmap');

    await gantt.setHeaderFilter('Phase', 'draft');

    await expect(gantt.rows).toHaveCount(1);
    await expect(gantt.rowTreeCellWithText('Alpha launch')).toHaveCount(1);
    await expect(appPage).toHaveURL(/filter(%5B|\[)phase/);
  });

  test('the sort menu orders the rows and keeps the sort in the URL', async ({ appPage }) => {
    const gantt = new GanttPage(appPage);
    await gantt.navigateToGantt('roadmap');

    await gantt.sortBy('Title', 'Descending');

    await expect(appPage).toHaveURL(/sort=-title/);
    await expect
      .poll(async () => (await gantt.rowTitles()).map((t) => t.match(/(Alpha|Bravo|Charlie)/)?.[1]))
      .toEqual(['Charlie', 'Bravo', 'Alpha']);
  });
});

test.describe('Calendar toolbar', () => {
  test.beforeEach(async ({ api }) => {
    await api.createEntity('plans', {
      properties: { title: 'Alpha launch', phase: 'draft', start: isoFromToday(0), end: isoFromToday(1) },
    });
    await api.createEntity('plans', {
      properties: { title: 'Bravo review', phase: 'active', start: isoFromToday(0), end: isoFromToday(1) },
    });
  });

  test('search narrows the events to the matches', async ({ appPage }) => {
    const calendar = new CalendarPage(appPage);
    await calendar.navigateToCalendar('plan-calendar');
    await expect(calendar.chips).toHaveCount(2);

    await calendar.search('Alpha');

    await expect(calendar.chip('Alpha launch')).toHaveCount(1);
    await expect(calendar.chip('Bravo review')).toHaveCount(0);
  });

  test('a filter narrows the events', async ({ appPage }) => {
    const calendar = new CalendarPage(appPage);
    await calendar.navigateToCalendar('plan-calendar');

    await calendar.setHeaderFilter('Phase', 'active');

    await expect(calendar.chip('Bravo review')).toHaveCount(1);
    await expect(calendar.chip('Alpha launch')).toHaveCount(0);
  });
});
