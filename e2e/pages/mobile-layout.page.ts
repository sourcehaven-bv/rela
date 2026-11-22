import { type Page, type Locator, expect } from "@playwright/test";
import { BasePage } from "./base.page";

/** An element's box in viewport pixels. */
export interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

/**
 * Geometry of the mobile app shell.
 *
 * Exists because the guarantees below are positional and none of them is
 * reachable from the rest of the suite: Desktop Chrome never crosses the
 * 768px breakpoint, and jsdom has no layout engine, so a unit test cannot
 * tell a sticky bar that works from one that silently stopped sticking.
 *
 * The shell (RlAppShell) makes the main pane the scroll container rather than
 * the document. That is what lets a right-hand panel exist at all, and it is
 * also what puts every sticky bar in PageLayout at risk: `top: 0` now means
 * the top of the pane. These assertions pin the outcome, not the mechanism,
 * so the same expectations hold if the shell's internals change again.
 */
export class MobileLayoutPage extends BasePage {
  readonly hamburger: Locator;
  readonly drawer: Locator;
  readonly scrim: Locator;
  /** The shell's scrolling pane — the scroll container, not the document. */
  readonly mainPane: Locator;
  /** PageLayout's sticky header bar, present only on the compact layout. */
  readonly topbar: Locator;

  constructor(page: Page) {
    super(page);
    // Two triggers, one outcome. A screen whose header is hoisted into the
    // shell (a list) gets RlPageHeader's own nav toggle; one that still
    // renders its header inline keeps App.vue's button. Exactly one is
    // present per screen — App.vue suppresses its own when a hoisted header
    // is there — so this stays a single-element locator.
    this.hamburger = page.locator(
      ".mobile-menu-btn, .rl-page-header__nav-toggle",
    );
    this.drawer = page.locator(".rl-app-shell__sidebar");
    this.scrim = page.locator(".rl-app-shell__scrim");
    this.mainPane = page.locator(".rl-app-shell__main");
    this.topbar = page.locator(".mobile-topbar").first();
  }

  /**
   * Open a list and wait for its rows.
   *
   * Not `ListPage.navigateToList`: below the breakpoint EntityList renders
   * cards rather than table rows, so that helper's row selector matches
   * nothing here and every assertion would time out before it ran.
   */
  async openList(listId: string) {
    await this.navigateTo(`/list/${listId}`);
    // RlTable stacks each row into a card at narrow widths; there is no
    // separate mobile markup any more, so wait on the row itself.
    await expect(this.page.locator(".rl-table-row").first()).toBeVisible();
  }

  /**
   * Keyboard shortcut hints (RlKbd).
   *
   * A hint is meaningless on a touch device, so the shell hides every one
   * below the breakpoint. The rule lives in a media query and RlKbd sets its
   * own `display: inline-flex`, so only a real browser at a real width can
   * tell whether the hint is actually hidden — happy-dom applies neither the
   * library stylesheet nor media queries and reports it visible either way.
   */
  get shortcutHints(): Locator {
    return this.page.locator(".rl-kbd");
  }

  /** The list's search box, in the header's tools row. */
  get listSearch(): Locator {
    return this.page.locator(".rl-page-header__tools .search-box input");
  }

  /** The drawer's computed box-shadow. */
  async drawerShadow(): Promise<string> {
    return this.drawer.evaluate((el) => getComputedStyle(el).boxShadow);
  }

  /** Whether this screen renders a sticky topbar at all. */
  async hasTopbar(): Promise<boolean> {
    return (await this.topbar.count()) > 0;
  }

  async openDrawer() {
    await this.hamburger.click();
  }

  /**
   * The drawer is on screen.
   *
   * Asserted by its box rather than by a class: the sidebar is always in the
   * DOM and is moved with a transform, so `toBeVisible` is true either way and
   * only its position says whether the user can see it.
   *
   * Polled, because the slide is a 200ms transition — reading the box once,
   * straight after the click, samples it mid-flight and reports a position the
   * drawer is only passing through.
   */
  async expectDrawerOnScreen() {
    await expect
      .poll(async () => (await this.drawer.boundingBox())?.x ?? null)
      .toBeGreaterThanOrEqual(-1);
  }

  /** The drawer is off-canvas to the left. */
  async expectDrawerOffScreen() {
    await expect
      .poll(async () => {
        const box = await this.drawer.boundingBox();
        return box ? box.x + box.width : null;
      })
      .toBeLessThanOrEqual(1);
  }

  /**
   * Click the scrim clear of the open drawer.
   *
   * The scrim covers the viewport but the 240px drawer sits on top of its left
   * edge, so a click there hits a nav item instead and the drawer stays open.
   */
  async tapScrimBesideDrawer() {
    const drawer = await this.drawer.boundingBox();
    expect(drawer, "drawer has no box").not.toBeNull();
    await this.scrim.click({ position: { x: drawer!.width + 40, y: 300 } });
  }

  /**
   * Make the pane overflow.
   *
   * The seeded lists fit a phone screen, and a pane with nothing to scroll
   * reports scrollTop 0 no matter which element owns the overflow — so a
   * scroll-root assertion against the stock fixture passes without testing
   * anything.
   *
   * `flex-basis`, not `height`: the pane is a column flex container, so a
   * plain height on an EMPTY child is shrunk straight back to nothing and the
   * pane never overflows. Real content holds itself open, which is why the app
   * scrolls correctly while a naive filler suggests it does not.
   */
  async growPaneContent() {
    await this.mainPane.evaluate((el) => {
      const filler = document.createElement("div");
      filler.style.flex = "0 0 2000px";
      filler.dataset.testFiller = "true";
      el.appendChild(filler);
    });
  }

  /**
   * Scroll the pane, not the window.
   *
   * `window.scrollBy` is a no-op once the shell owns the scrolling, so a test
   * written that way passes while asserting nothing.
   */
  async scrollPane(y: number) {
    await this.mainPane.evaluate((el, dy) => el.scrollBy(0, dy), y);
    // Let the sticky recalculation land before anything is measured.
    await this.page.waitForTimeout(100);
  }

  async paneScrollTop(): Promise<number> {
    return this.mainPane.evaluate((el) => el.scrollTop);
  }

  /**
   * The alpha of an element's computed background colour: 0 when content
   * under it shows through, 1 when it is opaque.
   */
  async backgroundAlpha(locator: Locator): Promise<number> {
    return locator.evaluate((el) => {
      const probe = document.createElement("canvas").getContext("2d")!;
      probe.fillStyle = getComputedStyle(el).backgroundColor;
      probe.fillRect(0, 0, 1, 1);
      return probe.getImageData(0, 0, 1, 1).data[3] / 255;
    });
  }

  /** Top edge of an element, in viewport coordinates. */
  async topOf(locator: Locator): Promise<number> {
    const box = await locator.boundingBox();
    expect(box, "element has no box").not.toBeNull();
    return box!.y;
  }

  /** The value of `--page-padding-x` as the pane resolves it. */
  /**
   * The USED length of --page-padding-x, in pixels.
   *
   * `getPropertyValue` on a custom property returns the token unresolved —
   * now `calc(16px + 0px)`, since the value comes from the library's page
   * gutter plus a safe-area inset. The bars bleed by the resolved length, so
   * that is what has to be compared. Resolved by letting the engine lay the
   * value out as a real width.
   */
  async pagePaddingX(): Promise<string> {
    return this.mainPane.evaluate((el) => {
      const probe = document.createElement("div");
      probe.style.width = "var(--page-padding-x)";
      probe.style.position = "absolute";
      probe.style.visibility = "hidden";
      el.appendChild(probe);
      const width = getComputedStyle(probe).width;
      probe.remove();
      return width;
    });
  }

  /** Computed padding of the pane, which carries the safe-area insets. */
  async panePadding(): Promise<{
    top: string;
    bottom: string;
    left: string;
    right: string;
  }> {
    return this.mainPane.evaluate((el) => {
      const s = getComputedStyle(el);
      return {
        top: s.paddingTop,
        bottom: s.paddingBottom,
        left: s.paddingLeft,
        right: s.paddingRight,
      };
    });
  }

  /**
   * The empty space between the bottom of the shell's header band and the
   * top of the list's table, in pixels. The table is the first thing under
   * the header, so anything above a few pixels is a band (BUG-6WQ1U8S).
   */
  async headerToTableGap(): Promise<number> {
    const header = await this.page.locator(".rl-app-shell__header").boundingBox();
    const table = await this.page.locator(".entity-list .rl-table").first().boundingBox();
    if (!header || !table) throw new Error("header or table not laid out");
    const firstRow = await this.page.locator(".rl-table-row").first().boundingBox();
    if (!firstRow) throw new Error("no table row laid out");
    // On a phone the column headers are hidden, so the first row is the
    // first thing in the table; the table's own top is where it starts.
    return Math.min(table.y, firstRow.y) - (header.y + header.height);
  }

  /** The title beside the menu toggle in the phone top bar. */
  menuBarTitle(): Locator {
    return this.page.getByTestId("mobile-menu-bar-title");
  }

  /**
   * The left edge of each kept (primary) cell, per list row, in pixels. The
   * compressed row hides the other cells with `display: none`, which leaves
   * them without a box.
   */
  async keptCellLefts(): Promise<number[][]> {
    return this.page.locator(".entity-list .rl-table-row").evaluateAll((rows) =>
      rows.map((row) =>
        Array.from(row.querySelectorAll<HTMLElement>(".rl-table-row__cell"))
          .filter((cell) => getComputedStyle(cell).display !== "none")
          .map((cell) => Math.round(cell.getBoundingClientRect().left)),
      ),
    );
  }

  /**
   * The boxes of the first list row's title cell and of its kept (primary)
   * cells, in pixels.
   */
  async firstRowBoxes(): Promise<{
    row: Box;
    title: Box;
    cells: Box[];
  }> {
    return this.page
      .locator(".entity-list .rl-table-row")
      .first()
      .evaluate((row) => {
        const box = (el: Element) => {
          const r = el.getBoundingClientRect();
          return { x: r.x, y: r.y, width: r.width, height: r.height };
        };
        // The title's cell, which holds the select box and the link.
        const title = row.querySelector(".rl-table-row__name");
        if (!title) throw new Error("row has no title cell");
        return {
          row: box(row),
          title: box(title),
          cells: Array.from(row.querySelectorAll(".rl-table-row__cell"))
            .filter((cell) => getComputedStyle(cell).display !== "none")
            .map(box),
        };
      });
  }

  /** Whether the first row's title is cut off with an ellipsis. */
  async firstRowTitleTruncated(): Promise<boolean> {
    return this.page
      .locator(".entity-list .rl-table-row .rl-table-row__primary > *")
      .first()
      .evaluate((el) => el.scrollWidth > el.clientWidth + 1);
  }

  /** The page title in the shell's header band. */
  pageHeaderTitle(): Locator {
    return this.page.locator(".rl-page-header__title");
  }

  /** Open the default dashboard and wait for its cards. */
  async openDashboard() {
    await this.navigateTo("/dashboard");
    await expect(this.page.locator(".dashboard-card").first()).toBeVisible();
  }

  /** Open an entity page and wait for its `display: table` section. */
  async openEntityTable(type: string, id: string, heading: string) {
    await this.navigateTo(`/entity/${type}/${id}`);
    await expect(this.sectionTable(heading)).toBeVisible();
  }

  /** The table of the view section with this heading. */
  sectionTable(heading: string): Locator {
    return this.page
      .locator(".view-section", {
        has: this.page.getByRole("heading", { name: heading, exact: true }),
      })
      .locator(".data-table")
      .first();
  }

  /** Whether the section's table is wider than its wrapper (scrolls sideways). */
  async sectionTableScrolls(heading: string): Promise<boolean> {
    return this.sectionTable(heading).evaluate((table) => {
      const wrapper = table.closest(".table-wrapper");
      return !!wrapper && wrapper.scrollWidth > wrapper.clientWidth + 1;
    });
  }

  /** Open an entity page and wait for the properties section with this heading. */
  async openEntitySection(type: string, id: string, heading: string) {
    await this.navigateTo(`/entity/${type}/${id}`);
    await expect(this.section(heading)).toBeVisible();
  }

  /** The view section with this heading. */
  section(heading: string): Locator {
    return this.page.locator(".view-section", {
      has: this.page.getByRole("heading", { name: heading, exact: true }),
    });
  }

  /** Whether the main pane is wider than the screen (scrolls sideways). */
  async paneScrollsSideways(): Promise<boolean> {
    return this.mainPane.evaluate((pane) => pane.scrollWidth > pane.clientWidth + 1);
  }

  /** The value in the section that shows exactly this text. */
  sectionValue(heading: string, text: string): Locator {
    return this.section(heading).locator(".display-value", { hasText: text });
  }

  /**
   * Whether any value in the section reaches past the section's right edge.
   * Throws when the section shows no value, so a selector that matches
   * nothing cannot pass as "no overflow".
   */
  async sectionValueOverflows(heading: string): Promise<boolean> {
    return this.section(heading).evaluate((section) => {
      const values = [...section.querySelectorAll(".display-value")];
      if (values.length === 0) throw new Error("section shows no .display-value");
      const right = section.getBoundingClientRect().right;
      return values.some((el) => el.getBoundingClientRect().right > right + 1);
    });
  }

  /** The number of lines the element's text takes. */
  async lineCount(locator: Locator): Promise<number> {
    return locator.evaluate((el) => {
      const lineHeight = parseFloat(getComputedStyle(el).lineHeight);
      return Math.round(el.getBoundingClientRect().height / lineHeight);
    });
  }

  /** The column-name labels a stacked section row shows beside its values. */
  sectionCellLabels(heading: string): Locator {
    return this.sectionTable(heading).locator("tbody td .section-cell__label");
  }
}
