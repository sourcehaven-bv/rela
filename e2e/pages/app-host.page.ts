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
  readonly cspProbe: Locator;
  /** The <rela-editor> element the demo app embeds. */
  readonly editor: Locator;
  /** The editor's writing surface (ProseMirror's contenteditable root). */
  readonly editorSurface: Locator;
  readonly editorToolbar: Locator;
  readonly editorPlaceholder: Locator;
  /** The app's mirror of `editor.value`, as a JSON string. */
  readonly editorValue: Locator;
  /** The app's running `input=N change=N` tally. */
  readonly editorEvents: Locator;

  constructor(page: Page) {
    super(page);
    this.frame = page.frameLocator('.app-host__frame');
    this.status = this.frame.locator('[data-testid="status"]');
    this.featureCount = this.frame.locator('[data-testid="feature-count"]');
    this.linkButton = this.frame.locator('[data-testid="link-btn"]');
    this.linkResult = this.frame.locator('[data-testid="link-result"]');
    this.cspProbe = this.frame.locator('[data-testid="csp-probe"]');
    this.editor = this.frame.locator('[data-testid="editor"]');
    this.editorSurface = this.editor.locator('.ProseMirror');
    this.editorToolbar = this.editor.locator('.rela-toolbar');
    this.editorPlaceholder = this.editor.locator('.rela-editor-placeholder');
    this.editorValue = this.frame.locator('[data-testid="editor-value"]');
    this.editorEvents = this.frame.locator('[data-testid="editor-events"]');
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

  // --- the embedded <rela-editor> (TKT-D2JML7) ---

  /** Wait until the app has loaded its body into the editor.
   *
   * Waits for the CONTENT to be on screen, not just for the app's value mirror
   * to be non-empty. The mirror is written as soon as `.value` is set, which can
   * be a frame before ProseMirror has rendered — and a click placed in that gap
   * lands on nothing, so the toolbar reports no active command and a test that
   * checks one fails intermittently.
   */
  async waitForEditor() {
    await expect(this.editorSurface).toBeVisible();
    await expect(this.editorValue).not.toHaveText('""');
    await expect(this.editorSurface.locator('h1')).toBeVisible();
  }

  /** The markdown the element's `value` property reports, unquoted. */
  async editorMarkdown(): Promise<string> {
    const raw = (await this.editorValue.textContent()) ?? '""';
    return JSON.parse(raw) as string;
  }

  /** Press a toolbar button by its command id (`h1`, `strong`, …). */
  async clickEditorCommand(command: string) {
    await this.editorToolbar.locator(`[data-command="${command}"]`).click();
  }

  /** Whether a toolbar command reads as active at the cursor. */
  async editorCommandActive(command: string): Promise<boolean> {
    const cls = await this.editorToolbar
      .locator(`[data-command="${command}"]`)
      .getAttribute('class');
    return (cls ?? '').includes('is-active');
  }

  /** Whether a toolbar command reads as unavailable where the cursor is. */
  async editorCommandUnavailable(command: string): Promise<boolean> {
    const v = await this.editorToolbar
      .locator(`[data-command="${command}"]`)
      .getAttribute('aria-disabled');
    return v === 'true';
  }

  /** Whether a toolbar command carries the NATIVE disabled attribute.
   *
   * It must not: a native disabled control cannot be focused, so if the cursor
   * moves into a context that disables the button the user is tabbed to, focus
   * drops to <body> mid-interaction. The editor uses `aria-disabled` and refuses
   * the command in its handler instead.
   */
  async editorCommandHasNativeDisabled(command: string): Promise<boolean> {
    return this.editorToolbar
      .locator(`[data-command="${command}"]`)
      .evaluate((el) => (el as HTMLButtonElement).disabled);
  }

  /** How many toolbar buttons the editor drew. */
  async editorToolbarButtonCount(): Promise<number> {
    return this.editorToolbar.locator('.rela-toolbar-button').count();
  }

  /** How many of those buttons drew an inline SVG glyph. */
  async editorToolbarIconCount(): Promise<number> {
    return this.editorToolbar.locator('.rela-toolbar-button svg').count();
  }

  /** The entity references the editor rendered, as their visible text. */
  async editorEntityRefs(): Promise<string[]> {
    return this.editorSurface.locator('a[data-entity-ref]').allTextContents();
  }

  /** A computed style on the editor's shell, for verifying the served CSS applied. */
  async editorShellStyle(prop: string): Promise<string> {
    return this.editor
      .locator('.rela-editor-shell')
      .evaluate((el, p) => getComputedStyle(el).getPropertyValue(p), prop);
  }

  /** A computed style on the writing surface. */
  async editorSurfaceStyle(prop: string): Promise<string> {
    return this.editorSurface.evaluate((el, p) => getComputedStyle(el).getPropertyValue(p), prop);
  }

  /** Put the cursor on the editor line containing `text`. */
  async clickEditorLine(text: string) {
    await this.editorSurface.getByText(text, { exact: false }).first().click();
  }
}
