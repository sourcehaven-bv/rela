import { type Page, expect } from '@playwright/test';
import { BasePage } from './base.page';

interface Recorded {
  title: string;
  body: string;
  tag: string;
}

/**
 * Browser notifications for the inbox (TKT-48S88RB), as one user sees them.
 *
 * Headless browsers cannot show real notifications, so `install` replaces the
 * Notification API with a recorder before the SPA loads, grants permission,
 * and lets the test decide whether the page is hidden and focused.
 */
export class NotificationsPage extends BasePage {
  constructor(page: Page) {
    super(page);
  }

  /** Call before the first navigation of the page. */
  static async install(page: Page): Promise<NotificationsPage> {
    await page.addInitScript(() => {
      const w = window as unknown as {
        __notes: Array<{ title: string; body: string; tag: string; click: () => void }>;
        __hidden: boolean;
        __focused: boolean;
      };
      w.__notes = [];
      w.__hidden = false;
      w.__focused = true;
      class RecordingNotification {
        static permission = 'granted';
        static requestPermission = async () => 'granted';
        onclick: (() => void) | null = null;
        constructor(title: string, opts: { body?: string; tag?: string } = {}) {
          w.__notes.push({
            title,
            body: opts.body ?? '',
            tag: opts.tag ?? '',
            click: () => this.onclick?.(),
          });
        }
        close() {}
      }
      Object.defineProperty(window, 'Notification', { value: RecordingNotification, configurable: true });
      Object.defineProperty(document, 'visibilityState', {
        get: () => (w.__hidden ? 'hidden' : 'visible'),
        configurable: true,
      });
      Object.defineProperty(document, 'hidden', { get: () => w.__hidden, configurable: true });
      document.hasFocus = () => w.__focused;
    });
    return new NotificationsPage(page);
  }

  /** Opens Settings and turns notifications on. */
  async enable() {
    await this.page.goto('/settings');
    const card = this.page.getByTestId('notification-setting');
    const toggle = card.getByRole('switch');
    await expect(toggle).toBeEnabled({ timeout: 15_000 });
    await toggle.click();
    await expect(toggle).toHaveAttribute('aria-checked', 'true');
  }

  /** Sends the page to the background: hidden and unfocused. */
  async background() {
    await this.page.evaluate(() => {
      const w = window as unknown as { __hidden: boolean; __focused: boolean };
      w.__hidden = true;
      w.__focused = false;
      window.dispatchEvent(new Event('blur'));
      document.dispatchEvent(new Event('visibilitychange'));
    });
  }

  /** The titles of the notifications shown so far. */
  async titles(): Promise<string[]> {
    return (await this.recorded()).map((n) => n.title);
  }

  private async recorded(): Promise<Recorded[]> {
    return this.page.evaluate(() =>
      (window as unknown as { __notes: Recorded[] }).__notes.map(({ title, body, tag }) => ({
        title,
        body,
        tag,
      })),
    );
  }

  /** Waits until `count` notifications were shown. */
  async expectCount(count: number, timeout = 30_000) {
    await expect.poll(async () => (await this.recorded()).length, { timeout }).toBe(count);
  }

  /** No notification was shown in this page. */
  async expectNone() {
    expect(await this.recorded()).toEqual([]);
  }

  /** Clicks notification `i` and waits for the URL it opens. */
  async click(i: number, url: RegExp) {
    await this.page.evaluate((n) => {
      (window as unknown as { __notes: Array<{ click: () => void }> }).__notes[n]!.click();
    }, i);
    await expect(this.page).toHaveURL(url, { timeout: 15_000 });
  }
}
