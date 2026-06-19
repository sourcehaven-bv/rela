import { type Page, type FrameLocator, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/** Page object for the custom-app host (/app/:id). The app renders inside a
 *  sandboxed iframe; all assertions go through the frame locator so specs never
 *  reach into iframe internals directly. */
export class AppHostPage extends BasePage {
  readonly frame: FrameLocator;
  readonly status: Locator;
  readonly featureCount: Locator;
  readonly linkButton: Locator;
  readonly linkResult: Locator;

  constructor(page: Page) {
    super(page);
    this.frame = page.frameLocator('.app-host__frame');
    this.status = this.frame.locator('[data-testid="status"]');
    this.featureCount = this.frame.locator('[data-testid="feature-count"]');
    this.linkButton = this.frame.locator('[data-testid="link-btn"]');
    this.linkResult = this.frame.locator('[data-testid="link-result"]');
  }

  /** Navigate to an app and wait for its bridge read to complete. */
  async open(appId: string) {
    await this.navigateTo(`/app/${appId}`);
    await expect(this.status).toHaveText('loaded');
  }

  /** The feature count the app fetched through the bridge (as a string). */
  async expectFeatureCount(n: number) {
    await expect(this.featureCount).toHaveText(String(n));
  }

  /** Click the app's "Link" button and wait for the bridge write to report. */
  async clickLink() {
    await this.linkButton.click();
    await expect(this.linkResult).toHaveText('linked');
  }

  /** Read the sandbox attribute of the host iframe (security assertion). */
  async iframeSandbox(): Promise<string | null> {
    return this.page.locator('.app-host__frame').getAttribute('sandbox');
  }
}
