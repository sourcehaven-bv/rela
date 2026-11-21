import { type Page, type Locator } from '@playwright/test';
import { BasePage } from './base.page';

export class CalendarPage extends BasePage {
  readonly chips: Locator;

  constructor(page: Page) {
    super(page);
    this.chips = page.locator('.calendar-view .calendar-chip');
  }

  async navigateToCalendar(id: string) {
    await this.navigateTo(`/calendar/${id}`);
    await this.page.locator('.calendar-view .calendar-grid').waitFor();
  }

  /** The event chip with this summary. */
  chip(summary: string): Locator {
    return this.chips.filter({ hasText: summary });
  }
}
