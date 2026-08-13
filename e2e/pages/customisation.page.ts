import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/**
 * Page object for the operator-customisation hooks (TKT-3DBK6I).
 *
 * The property under test is a CASCADE outcome, so everything here works in
 * computed styles rather than markup. It cannot be asserted in Go: only a real
 * browser resolves `@layer` against the ~19 stylesheets Vite emits, 18 of which
 * are route chunks appended to <head> at RUNTIME after the operator's <link>.
 */
export class CustomisationPage extends BasePage {
  readonly sidebarEl: Locator;

  constructor(page: Page) {
    super(page);
    this.sidebarEl = page.locator('.sidebar').first();
  }

  /** Whether the SPA shell references the operator's stylesheet. */
  async hasCustomStylesheet(): Promise<boolean> {
    return (await this.page.locator('link[href="/_custom/custom.css"]').count()) > 0;
  }

  /**
   * Fetch a /_custom/ asset directly and report status + content-type. Used to
   * prove an operator asset is actually reachable, not merely referenced.
   */
  async fetchCustomAsset(serverUrl: string, relPath: string) {
    const res = await this.page.request.get(`${serverUrl}/_custom/${relPath}`);
    return { status: res.status(), contentType: res.headers()['content-type'] ?? '', body: await res.text() };
  }

  /** Whether the SPA shell references the operator's module script. */
  async hasCustomScript(): Promise<boolean> {
    return (await this.page.locator('script[src="/_custom/custom.js"]').count()) > 0;
  }

  /** Resolved value of a CSS property on the sidebar. */
  async sidebarStyle(property: string): Promise<string> {
    return this.sidebarEl.evaluate(
      (el, prop) => getComputedStyle(el).getPropertyValue(prop).trim(),
      property,
    );
  }

  /**
   * Number of stylesheets currently in the document, and whether the operator's
   * is among them. Used to confirm route chunks really did load (the condition
   * that broke operator CSS before layering).
   */
  async stylesheetHrefs(): Promise<string[]> {
    return this.page.evaluate(() =>
      [...document.querySelectorAll('link[rel=stylesheet]')].map((l) => l.getAttribute('href') ?? ''),
    );
  }

  /**
   * Assert a marker attribute that the operator's custom.js set on <html>.
   * Keeps the DOM query out of the spec, per the page-object rule.
   */
  async expectOperatorScriptMarker(attribute: string, value: string) {
    await expect.poll(async () => this.page.locator(`html[${attribute}]`).count()).toBeGreaterThan(0);
    await expect(this.page.locator('html')).toHaveAttribute(attribute, value);
  }

  /** Assert the sidebar resolves to the operator's colour. */
  async expectSidebarBackground(rgb: string) {
    await expect
      .poll(async () => this.sidebarStyle('background-color'), {
        message: `sidebar background should resolve to the operator's ${rgb}`,
      })
      .toBe(rgb);
  }
}
